package registry

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
)

type chanStream struct {
	name     string
	c        chan []byte
	closer   *signaler
	watching map[string]bool
	nWatch   int64
}

type logStreamer struct {
	config    *restclient.Config
	namespace string
	name      string
}

func (s *logStreamer) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (s *logStreamer) DeepCopyObject() runtime.Object {
	panic("DeepCopyObject not implemented for logStreamer")
}

// getPods stream logs in non-follow mode for existing pods of the stack
func (s *logStreamer) getPods(cs *chanStream, core corev1.PodsGetter, tail *int64) error {
	pods, err := core.Pods(s.namespace).List(
		metav1.ListOptions{
			LabelSelector: "com.docker.stack.namespace=" + s.name,
		})
	if err != nil {
		return err
	}
	for _, pod := range pods.Items {
		s.streamLogs(cs, core, pod.Name, false, tail)
	}
	return nil
}

// watchPods stream logs for pods of the stack, watching for new pods
func (s *logStreamer) watchPods(cs *chanStream, podWatcher watch.Interface, core corev1.PodsGetter, tail *int64) {
	abort := cs.closer.Channel()
	for {
		select {
		case <-abort:
			log.Debugf("Stopping pod watcher for log stream of %s/%s", s.namespace, s.name)
			podWatcher.Stop()
			return
		case ev := <-podWatcher.ResultChan():
			if ev.Type != watch.Added && ev.Type != watch.Modified {
				continue
			}
			pod := ev.Object.(*apiv1.Pod)
			if pod.Status.Phase != apiv1.PodRunning {
				continue
			}
			if cs.watching[pod.Name] {
				continue
			}
			log.Debugf("Adding pod %s to log stream of %s/%s", pod.Name, s.namespace, s.name)
			s.streamLogs(cs, core, pod.Name, true, tail)
		}
	}
}

func (s *logStreamer) streamLogs(cs *chanStream, core corev1.PodsGetter, podName string, follow bool, tail *int64) {
	logreader, err := core.Pods(s.namespace).GetLogs(podName, &apiv1.PodLogOptions{
		Follow:    follow,
		TailLines: tail,
	}).Stream()
	if err != nil {
		log.Errorf("Failed to get pod %s/%s logs: %s", s.namespace, podName, err)
		return
	}
	cs.watching[podName] = true
	atomic.AddInt64(&cs.nWatch, 1)
	go func() {
		log.Debugf("Entering log reader goroutine for %s", podName)
		id := cs.closer.Register(func() { logreader.Close() })
		defer cs.closer.Unregister(id)
		in := bufio.NewReader(logreader)
		for {
			input, err := in.ReadSlice('\n')
			if err != nil {
				log.Debugf("Read error on pod logs %s: %s", podName, err)
				break
			}
			cs.c <- append([]byte(podName+" "), input...)
		}
		log.Debugf("Exiting log reader goroutine for %s", podName)
		atomic.AddInt64(&cs.nWatch, -1)
		cs.c <- nil // wake up forwarder goroutine so that it can close the stream if we were the last
	}()
}

func parseArgs(in *http.Request) (bool, *int64, *regexp.Regexp, error) {
	sFollow := in.FormValue("follow")
	sTail := in.FormValue("tail")
	sFilter := in.FormValue("filter")
	follow := sFollow != "" && sFollow != "0" && sFollow != "false"
	var tail *int64
	if sTail != "" {
		itail, err := strconv.Atoi(sTail)
		if err != nil {
			return false, nil, nil, err
		}
		i64tail := int64(itail)
		tail = &i64tail
	}
	if sFilter != "" {
		re, err := regexp.Compile(sFilter)
		return follow, tail, re, err
	}
	return follow, tail, nil, nil
}

func forwardLogs(cs *chanStream, follow bool, filter *regexp.Regexp, out io.Writer) { //nolint:gocyclo
	onClose := out.(http.CloseNotifier).CloseNotify()
	needFlush := false
	// time.After() is only GCed after timer expiration, so don't create one every iteration
	var flusher <-chan time.Time
loop:
	for {
		if flusher == nil && needFlush {
			flusher = time.After(2 * time.Second)
		}
		select {
		case <-flusher: // reading from a nil chan is legal
			if streamFlusher, ok := out.(http.Flusher); ok {
				streamFlusher.Flush()
			}
			needFlush = false
			flusher = nil
		case <-onClose:
			log.Debugf("EOF on log output stream for %s, terminating", cs.name)
			cs.closer.Signal()
			break loop
		case line := <-cs.c:
			if line != nil && (filter == nil || filter.Match(line)) {
				_, err := out.Write(line)
				if err != nil {
					log.Debugf("Write error on log output stream for %s, terminating: %s", cs.name, err)
					cs.closer.Signal()
					break loop
				}
				if follow {
					needFlush = true
				}
			}
			if !follow && atomic.LoadInt64(&cs.nWatch) == 0 {
				// drain before exiting
				for {
					select {
					case line := <-cs.c:
						if line != nil && (filter == nil || filter.Match(line)) {
							out.Write(line)
						}
					default:
						return
					}
				}
			}
		}
	}
}

func (s *logStreamer) ServeHTTP(out http.ResponseWriter, in *http.Request) {
	follow, tail, filter, err := parseArgs(in)
	if err != nil {
		fmt.Fprintf(out, "Argument parse error: %s\n", err)
		return
	}
	log.Infof("Processing log request for %s/%s follow=%v tail=%v", s.namespace, s.name, follow, tail)
	c := make(chan []byte, 100)
	cs := &chanStream{fmt.Sprintf("%s/%s", s.namespace, s.name), c, newSignaler(), make(map[string]bool), 0}
	core, err := corev1.NewForConfig(s.config)
	if err != nil {
		fmt.Fprintf(out, "Failed to get corev1 interface: %s\n", err)
		return
	}
	if follow {
		podWatcher, err := core.Pods(s.namespace).Watch(
			metav1.ListOptions{
				LabelSelector: "com.docker.stack.namespace=" + s.name,
			})
		if err != nil {
			fmt.Fprintf(out, "Failed to watch pods: %s\n", err)
			return
		}
		go s.watchPods(cs, podWatcher, core, tail)
	} else {
		err := s.getPods(cs, core, tail)
		if err != nil {
			fmt.Fprintf(out, "Failed to get pods: %s\n", err)
			return
		}
		if atomic.LoadInt64(&cs.nWatch) == 0 {
			fmt.Fprintf(out, "Stack %s/%s does not exist or has no running pods\n", s.namespace, s.name)
			return
		}
	}
	forwardLogs(cs, follow, filter, out)
}

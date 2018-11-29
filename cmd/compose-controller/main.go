package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	cliopts "github.com/docker/cli/opts"
	"github.com/docker/compose-on-kubernetes/api/client/clientset"
	"github.com/docker/compose-on-kubernetes/api/compose/v1beta2"
	"github.com/docker/compose-on-kubernetes/api/constants"
	"github.com/docker/compose-on-kubernetes/internal"
	"github.com/docker/compose-on-kubernetes/internal/check"
	"github.com/docker/compose-on-kubernetes/internal/controller"
	"github.com/docker/compose-on-kubernetes/internal/deduplication"
	homedir "github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	coretypes "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/server/healthz"
	k8sclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (

	// reconcileQueueLength is the size of the buffer for reconciliation deduplication (it means that up to 200 stacks can be updated
	// concurrently before getting unnecessary duplicated events)
	reconcileQueueLength = 200

	// deletionChannelSize is the size of the chan in which delete events are queued.
	// this means that the stackInformer won't block until we get more than 50 deletions messages in queue
	deletionChannelSize = 50
)

type controllerOptions struct {
	kubeconfig             string
	reconciliationInterval cliopts.PositiveDurationOpt
	logLevel               string
	defaultServiceType     string
}

func defaultOptions() controllerOptions {
	defaultReconciliation := constants.DefaultFullSyncInterval
	return controllerOptions{
		reconciliationInterval: cliopts.PositiveDurationOpt{
			DurationOpt: *cliopts.NewDurationOpt(&defaultReconciliation),
		},
	}
}

func main() {
	opts := defaultOptions()

	flag.StringVar(&opts.kubeconfig, "kubeconfig", "~/.kube/config", "Path to a kube config. Only required if out-of-cluster.")
	flag.Var(&opts.reconciliationInterval, "reconciliation-interval", "Reconciliation interval of the stack controller (default: 12h)")
	flag.StringVar(&opts.logLevel, "log-level", "info", `Set the logging level ("debug"|"info"|"warn"|"error"|"fatal")`)
	flag.StringVar(&opts.defaultServiceType, "default-service-type", "LoadBalancer", `Specify the default service type for published ports ("LoadBalancer"|"NodePort")`)

	flag.Parse()

	if err := start(&opts); err != nil {
		log.Fatalln(err)
	}
}

func start(opts *controllerOptions) error {
	initLogger(opts.logLevel, os.Stdout)
	fmt.Println(internal.FullVersion())

	configFile, err := homedir.Expand(opts.kubeconfig)
	if err != nil {
		return err
	}
	log.Debugf("Using config file: %s", configFile)

	config, err := clientcmd.BuildConfigFromFlags("", configFile)
	if err != nil {
		return err
	}

	// Chances are we were started at the same time as the API server, so give
	// it time to start
	for i := 0; i < 60; i++ {
		err = check.APIPresent(config)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		return err
	}

	clientSet, err := clientset.NewForConfig(config)
	if err != nil {
		return err
	}
	k8sClientSet, err := k8sclientset.NewForConfig(config)
	if err != nil {
		return err
	}
	cache, err := controller.NewStackOwnerCache(config)
	if err != nil {
		return err
	}
	timeoutctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()
	for {
		if wi, err := clientSet.ComposeV1beta2().Stacks(metav1.NamespaceAll).Watch(metav1.ListOptions{}); err == nil {
			wi.Stop()
			break
		}
		select {
		case <-timeoutctx.Done():
			return errors.New("cannot watch stacks")
		default:
			time.Sleep(time.Second)
		}
	}
	stop := make(chan struct{})
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		log.Infof("Received signal: %v", sig)
		close(stop)
	}()
	healthz.DefaultHealthz()
	go http.ListenAndServe(":8080", nil)
	reconcileQueue := deduplication.NewStringChan(reconcileQueueLength)
	deletionQueue := make(chan *v1beta2.Stack, deletionChannelSize)
	childrenStore, err := controller.NewChildrenListener(k8sClientSet, *opts.reconciliationInterval.Value(), reconcileQueue.In())
	if err != nil {
		return err
	}
	if !childrenStore.StartAndWaitForFullSync(stop) {
		return errors.New("children store failed to sync")
	}

	stackStore := controller.NewStackListener(clientSet, *opts.reconciliationInterval.Value(), reconcileQueue.In(), deletionQueue, cache)
	stackStore.Start(stop)
	stackReconciler, err := controller.NewStackReconciler(
		stackStore,
		childrenStore,
		coretypes.ServiceType(opts.defaultServiceType),
		controller.NewImpersonatingResourceUpdaterProvider(*config, cache),
		cache)
	if err != nil {
		return err
	}
	stackReconciler.Start(reconcileQueue.Out(), deletionQueue, stop)
	log.Infof("Controller ready")
	<-stop
	return nil
}

func initLogger(level string, out io.Writer) {
	log.SetOutput(out)
	parseLogLevel(level)
}

func parseLogLevel(level string) {
	lvl, err := log.ParseLevel(level)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to parse log level: %s\n", level)
		os.Exit(1)
	}
	log.SetLevel(lvl)
}

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/containerd/console"
	clientset "github.com/docker/compose-on-kubernetes/api/client/clientset/typed/compose/v1beta2"
	"github.com/docker/compose-on-kubernetes/api/constants"
	"github.com/docker/compose-on-kubernetes/install"
	e2ewait "github.com/docker/compose-on-kubernetes/internal/e2e/wait"
	"github.com/morikuni/aec"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	appstypes "k8s.io/api/apps/v1"
	coretypes "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func (s *workerState) getPhaseTimings(start time.Time) map[string]time.Duration {
	last := start
	res := make(map[string]time.Duration)
	for _, p := range s.PreviousPhases {
		res[p.Name] = p.DoneTime.Sub(last)
		last = p.DoneTime
	}
	return res
}

func computePhaseTimingAverages(start time.Time, states []*workerState) []timedPhase {
	if len(states) == 0 {
		return nil
	}
	timings := make([]map[string]time.Duration, len(states))
	for ix, s := range states {
		timings[ix] = s.getPhaseTimings(start)
	}
	var result []timedPhase
	for _, phase := range states[0].PreviousPhases {
		count := 0
		var total time.Duration
		for _, t := range timings {
			if v, ok := t[phase.Name]; ok {
				count++
				total += v
			}
		}
		result = append(result, timedPhase{
			duration: time.Duration(int64(total) / int64(count)),
			name:     phase.Name,
		})
	}
	return result
}

type statusPrinter struct {
	out               io.Writer
	previousLineCount int
}

func (r *statusPrinter) print(states []*workerState, start time.Time, withConsole bool) {
	if withConsole {
		b := aec.EmptyBuilder
		for ix := 0; ix < r.previousLineCount; ix++ {
			b = b.Up(1).EraseLine(aec.EraseModes.All)
		}
		fmt.Fprintf(r.out, b.ANSI.Apply(""))
	}
	tw := tabwriter.NewWriter(r.out, 5, 1, 4, ' ', 0)
	defer tw.Flush()
	count := 0
	// headers
	fmt.Fprint(tw, " ")
	maxPrevious := 0
	for _, s := range states {
		s.Lock()
		fmt.Fprintf(tw, "\t%s", strings.ToUpper(s.ID))
		if l := len(s.PreviousPhases); l > maxPrevious {
			maxPrevious = l
		}
		s.Unlock()
	}
	fmt.Fprint(tw, "\n")
	count++
	for ix := 0; ix < len(states)+1; ix++ {
		fmt.Fprint(tw, "---\t")
	}
	fmt.Fprint(tw, "\n")
	count++

	// previous steps
	for ix := 0; ix < maxPrevious; ix++ {
		if ix == 0 {
			fmt.Fprint(tw, "PREVIOUS STEPS")
		} else {
			fmt.Fprint(tw, " ")
		}
		for _, s := range states {
			s.Lock()
			fmt.Fprint(tw, "\t")
			if len(s.PreviousPhases) > ix {
				baseDate := start
				if ix > 0 {
					baseDate = s.PreviousPhases[ix-1].DoneTime
				}
				duration := s.PreviousPhases[ix].DoneTime.Sub(baseDate)
				fmt.Fprintf(tw, "%s: %v", s.PreviousPhases[ix].Name, duration)
			} else {
				fmt.Fprint(tw, " ")
			}
			s.Unlock()
		}
		fmt.Fprint(tw, "\n")
		count++
	}

	for ix := 0; ix < len(states)+1; ix++ {
		fmt.Fprint(tw, "---\t")
	}
	fmt.Fprint(tw, "\n")
	count++
	// current step
	fmt.Fprint(tw, "CURRENT STEP")
	for _, s := range states {
		s.Lock()
		fmt.Fprintf(tw, "\t%s", s.CurrentPhase)
		s.Unlock()
	}
	fmt.Fprint(tw, "\n")
	count++

	tw.Write([]byte(" "))
	for _, s := range states {
		s.Lock()
		fmt.Fprintf(tw, "\t%s", s.CurrentMessage)
		s.Unlock()
	}
	fmt.Fprint(tw, "\n")
	count++
	r.previousLineCount = count
}

func main() {
	opts := &options{}
	cmd := &cobra.Command{
		Use: "e2e_benchmark [options]",
		RunE: func(_ *cobra.Command, _ []string) error {
			return run(opts)
		},
	}
	cmd.Flags().StringVar(&opts.kubeconfig, "kubeconfig", "", "kubeconfig path")
	cmd.Flags().IntVar(&opts.workerCount, "worker-count", 5, "number of benchmark workers")
	cmd.Flags().IntVar(&opts.totalStacks, "total-stacks", 200, "number of stacks created/removed per worker")
	cmd.Flags().StringVarP(&opts.format, "format", "f", "auto", "output format: auto|json|interactive|report")
	cmd.Flags().StringVar(&opts.collectLogsNamespace, "logs-namespace", "", "namespace to collect Compose on Kubernetes logs from")
	cmd.Flags().DurationVar(&opts.maxDuration, "max-duration", 0, "maximum duration of the benchmark (fails if exceeded)")

	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}

func run(opts *options) error {
	ctx := context.Background()
	if opts.maxDuration > 0 {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, opts.maxDuration)
		defer cancel()
	}
	var (
		out io.Writer
		err error
	)
	if out, opts.format, err = configureOutput(opts.format); err != nil {
		return err
	}
	restCfg, err := configureRest(opts.kubeconfig)
	if err != nil {
		return err
	}
	if opts.collectLogsNamespace != "" {
		defer collectLogsToStderr(restCfg, opts.collectLogsNamespace)
	}
	if err := ensureInstalled(restCfg); err != nil {
		return err
	}

	start := time.Now()

	eg, _ := errgroup.WithContext(ctx)
	var states []*workerState
	stacksPerWorker := opts.totalStacks / opts.workerCount
	for workerIX := 0; workerIX < opts.workerCount; workerIX++ {
		workerID := fmt.Sprintf("bench-worker-%d", workerIX)
		state := &workerState{
			ID: workerID,
		}
		states = append(states, state)
		stacksForThisWorker := stacksPerWorker
		if workerIX < (opts.totalStacks % opts.workerCount) {
			stacksForThisWorker++
		}
		eg.Go(func() error {
			return benchmarkRun(restCfg, workerID, stacksForThisWorker, func(u stateUpdater) {
				state.Lock()
				defer state.Unlock()
				u(state)
			})
		})
	}
	finishedC := make(chan error)

	go func() {
		defer close(finishedC)
		finishedC <- eg.Wait()
	}()
	return reportBenchStatus(ctx, out, finishedC, start, opts.format, states)
}

func configureOutput(format string) (io.Writer, string, error) {
	switch format {
	case "interactive", "auto":
		c, err := console.ConsoleFromFile(os.Stdout)
		if err != nil {
			if format == "auto" {
				return os.Stdout, "report", nil
			}
			return nil, "", errors.Wrapf(err, "unable to set interactive console")
		}
		return c, "interactive", nil
	case "json", "report":
		return os.Stdout, format, nil
	}
	return nil, "", errors.Errorf("unexpected format %s. must be auto, json, interactive or report", format)
}

func configureRest(kubeconfig string) (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = kubeconfig
	cmdConfig, err := loadingRules.Load()
	if err != nil {
		return nil, err
	}
	clientCfg := clientcmd.NewDefaultClientConfig(*cmdConfig, &clientcmd.ConfigOverrides{})
	return clientCfg.ClientConfig()
}

func collectLogsToStderr(cfg *rest.Config, ns string) {
	client, err := k8sclientset.NewForConfig(cfg)
	if err != nil {
		panic(err)
	}
	pods, err := client.CoreV1().Pods(ns).List(metav1.ListOptions{})
	if err != nil {
		panic(err)
	}
	for _, pod := range pods.Items {
		for _, cont := range pod.Status.ContainerStatuses {
			fmt.Fprintf(os.Stderr, "\nCurrent logs for %s/%s\n", pod.Name, cont.Name)
			data, err := client.CoreV1().Pods(ns).GetLogs(pod.Name, &coretypes.PodLogOptions{Container: cont.Name}).Stream()
			if err != nil {
				panic(err)
			}
			io.Copy(os.Stderr, data)
		}
	}
}

func ensureInstalled(config *rest.Config) error {
	stackclient, err := clientset.NewForConfig(config)
	if err != nil {
		return err
	}
	if _, err := stackclient.Stacks(metav1.NamespaceAll).List(metav1.ListOptions{}); err == nil {
		// installed
		return nil
	}

	tag := os.Getenv("TAG")
	if tag == "" {
		return errors.New("stacks API is not installed and TAG env var is not set. Cannot install")
	}

	k8sclient, err := k8sclientset.NewForConfig(config)
	if err != nil {
		return err
	}
	if _, err := k8sclient.CoreV1().Namespaces().Get("benchmark", metav1.GetOptions{}); err != nil {
		if kerrors.IsNotFound(err) {
			if _, err := k8sclient.CoreV1().Namespaces().Create(&coretypes.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "benchmark",
				},
			}); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	if err := install.Do(context.Background(), config,
		install.WithUnsafe(install.UnsafeOptions{
			OptionsCommon: install.OptionsCommon{
				Namespace:              "benchmark",
				Tag:                    tag,
				ReconciliationInterval: constants.DefaultFullSyncInterval,
			}}),
		install.WithObjectFilter(func(o runtime.Object) (bool, error) {
			switch v := o.(type) {
			case *appstypes.Deployment:
				// change from pull always to pull never (image is already loaded, and not yet on hub)
				// only apply to 1st container in POD (2nd container for API is etcd, and we might need to pull it)
				v.Spec.Template.Spec.Containers[0].ImagePullPolicy = coretypes.PullNever
			}
			return true, nil
		}),
	); err != nil {
		return err
	}
	if err = install.WaitNPods(config, "benchmark", 2, 2*time.Minute); err != nil {
		return err
	}
	return e2ewait.For(300, func() (bool, error) {
		_, err := stackclient.Stacks("default").List(metav1.ListOptions{})
		return err == nil, err
	})
}

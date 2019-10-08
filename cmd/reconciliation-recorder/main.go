package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/compose-on-kubernetes/api/client/clientset"
	"github.com/docker/compose-on-kubernetes/api/labels"
	"github.com/docker/compose-on-kubernetes/internal/controller"
	"github.com/docker/compose-on-kubernetes/internal/stackresources"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type options struct {
	kubeconfig string
	namespace  string
	outdir     string
}

func main() {
	opts := &options{}
	cmd := &cobra.Command{
		Use: "reconciliation-recorder [OPTIONS] [stack name [stack name...]]",
		RunE: func(c *cobra.Command, args []string) error {
			return run(args, opts)
		},
	}

	cmd.Flags().StringVar(&opts.kubeconfig, "kubeconfig", "", "kubeconfig path")
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "default", "namespace of the stack to capture")
	cmd.Flags().StringVarP(&opts.outdir, "out", "o", "./out", "output directory where to put assets")
	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}

func run(stacksToProceed []string, opts *options) error {
	if err := os.MkdirAll(opts.outdir, 0755); err != nil {
		return err
	}
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = opts.kubeconfig
	cmdConfig, err := loadingRules.Load()
	if err != nil {
		return err
	}
	clientCfg := clientcmd.NewDefaultClientConfig(*cmdConfig, &clientcmd.ConfigOverrides{})
	restCfg, err := clientCfg.ClientConfig()
	if err != nil {
		return err
	}
	stacks, err := clientset.NewForConfig(restCfg)
	if err != nil {
		return err
	}
	k8sclient, err := k8sclientset.NewForConfig(restCfg)
	if err != nil {
		return err
	}
	if len(stacksToProceed) == 0 {
		allStacks, err := stacks.ComposeLatest().Stacks(opts.namespace).List(metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, s := range allStacks.Items {
			stacksToProceed = append(stacksToProceed, s.Name)
		}
	}
	for _, name := range stacksToProceed {
		if err = processStack(stacks, k8sclient, name, opts.namespace, opts.outdir); err != nil {
			return err
		}
	}
	return nil
}

func processStack(stacks clientset.Interface, k8sclient k8sclientset.Interface, name, namespace, outdir string) error {
	stack, err := stacks.ComposeLatest().Stacks(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	services, err := k8sclient.CoreV1().Services(namespace).List(metav1.ListOptions{LabelSelector: labels.SelectorForStack(name)})
	if err != nil {
		return err
	}
	deployments, err := k8sclient.AppsV1().Deployments(namespace).List(metav1.ListOptions{LabelSelector: labels.SelectorForStack(name)})
	if err != nil {
		return err
	}
	daemonsets, err := k8sclient.AppsV1().DaemonSets(namespace).List(metav1.ListOptions{LabelSelector: labels.SelectorForStack(name)})
	if err != nil {
		return err
	}
	statefulsets, err := k8sclient.AppsV1().StatefulSets(namespace).List(metav1.ListOptions{LabelSelector: labels.SelectorForStack(name)})
	if err != nil {
		return err
	}
	var allResources []interface{}
	for _, r := range services.Items {
		local := r
		allResources = append(allResources, &local)
	}
	for _, r := range deployments.Items {
		local := r
		allResources = append(allResources, &local)
	}
	for _, r := range daemonsets.Items {
		local := r
		allResources = append(allResources, &local)
	}
	for _, r := range statefulsets.Items {
		local := r
		allResources = append(allResources, &local)
	}
	childrenState, err := stackresources.NewStackState(allResources...)
	if err != nil {
		return err
	}
	tc := &controller.TestCase{
		Stack:    stack,
		Children: childrenState,
	}
	payload, err := json.Marshal(tc)
	if err != nil {
		return err
	}
	path := filepath.Join(outdir, name+".json")
	return ioutil.WriteFile(path, payload, 0644)
}

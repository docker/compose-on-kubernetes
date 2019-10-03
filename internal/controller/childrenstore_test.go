package controller

import (
	"reflect"
	"testing"
	"time"

	"github.com/docker/compose-on-kubernetes/api/labels"
	"github.com/stretchr/testify/assert"
	appstypes "k8s.io/api/apps/v1"
	coretypes "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func objectMetaForStack(namespace, name, stackName string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
		Labels:    labels.ForService(stackName, name),
	}
}

func testDeploymentForStack(namespace, name, stackName string) interface{} {
	return &appstypes.Deployment{
		ObjectMeta: objectMetaForStack(namespace, name, stackName),
	}
}
func testStatefulsetForStack(namespace, name, stackName string) interface{} {
	return &appstypes.StatefulSet{
		ObjectMeta: objectMetaForStack(namespace, name, stackName),
	}
}
func testDaemonsetForStack(namespace, name, stackName string) interface{} {
	return &appstypes.DaemonSet{
		ObjectMeta: objectMetaForStack(namespace, name, stackName),
	}
}
func testServiceForStack(namespace, name, stackName string) interface{} {
	return &coretypes.Service{
		ObjectMeta: objectMetaForStack(namespace, name, stackName),
	}
}
func TestGetCurrentStackState(t *testing.T) {
	var withinStack []interface{}
	indexers := cache.Indexers{
		"by-stack": byStackIndexer,
	}
	deployments := cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc, indexers)
	statefulsets := cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc, indexers)
	daemonsets := cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc, indexers)
	services := cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc, indexers)

	d := testDeploymentForStack("test-ns", "dep1", "test-stack")
	deployments.Add(d)
	withinStack = append(withinStack, d)
	d = testDaemonsetForStack("test-ns", "daem1", "test-stack")
	daemonsets.Add(d)
	withinStack = append(withinStack, d)
	d = testStatefulsetForStack("test-ns", "stateful1", "test-stack")
	statefulsets.Add(d)
	withinStack = append(withinStack, d)
	d = testServiceForStack("test-ns", "svc1", "test-stack")
	services.Add(d)
	withinStack = append(withinStack, d)

	d = testDeploymentForStack("test-ns", "dep2", "other-stack")
	deployments.Add(d)
	d = testDaemonsetForStack("test-ns", "daem2", "other-stack")
	daemonsets.Add(d)
	d = testStatefulsetForStack("test-ns", "stateful2", "other-stack")
	statefulsets.Add(d)
	d = testServiceForStack("test-ns", "svc2", "other-stack")
	services.Add(d)

	d = testDeploymentForStack("other-ns", "dep3", "test-stack")
	deployments.Add(d)
	d = testDaemonsetForStack("other-ns", "daem3", "test-stack")
	daemonsets.Add(d)
	d = testStatefulsetForStack("other-ns", "stateful3", "test-stack")
	statefulsets.Add(d)
	d = testServiceForStack("other-ns", "svc3", "test-stack")
	services.Add(d)

	testee := &ChildrenListener{
		daemonsets:   daemonsets,
		deployments:  deployments,
		services:     services,
		statefulsets: statefulsets,
	}

	result, err := testee.getCurrentStackState("test-ns/test-stack")
	assert.NoError(t, err)
	flatten := result.FlattenResources()
	assert.Equal(t, len(withinStack), len(flatten))
	for _, item := range withinStack {
		found := false
		for _, other := range flatten {
			if reflect.DeepEqual(other, item) {
				found = true
				break
			}
		}
		assert.True(t, found)
	}
}

func TestRandomDuration(t *testing.T) {
	for ix := 0; ix < 10000; ix++ {
		result := randomDuration(time.Hour * 12)
		assert.True(t, result >= time.Hour*12)
		assert.True(t, result <= time.Hour*24)
	}
}

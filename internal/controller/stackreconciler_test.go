package controller

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sync"
	"testing"

	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	"github.com/docker/compose-on-kubernetes/internal/stackresources"
	"github.com/docker/compose-on-kubernetes/internal/stackresources/diff"
	. "github.com/docker/compose-on-kubernetes/internal/test/builders"
	"github.com/stretchr/testify/assert"
	coretypes "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type testChildrenStore struct {
	initialStackState *stackresources.StackState
}

func (s *testChildrenStore) getCurrentStackState(_ string) (*stackresources.StackState, error) {
	return s.initialStackState, nil
}

func newTestChildrenStore(objects ...interface{}) (*testChildrenStore, error) {
	state, err := stackresources.NewStackState(objects...)
	if err != nil {
		return nil, err
	}
	return &testChildrenStore{state}, err
}

type testStackStore struct {
	originalStack *latest.Stack
}

func (s *testStackStore) get(_ string) (*latest.Stack, error) {
	return s.originalStack, nil
}

func newTestStackStore(originalStack *latest.Stack) *testStackStore {
	return &testStackStore{originalStack: originalStack}
}

type testResourceUpdaterProvider struct {
	diffs                chan<- *diff.StackStateDiff
	statuses             chan<- *latest.Stack
	deleteChildResources chan<- struct{}
}

func (p *testResourceUpdaterProvider) getUpdater(stack *latest.Stack, _ bool) (resourceUpdater, error) {
	return &testResourceUpdater{
		diffs:                p.diffs,
		statuses:             p.statuses,
		stack:                stack,
		deleteChildResources: p.deleteChildResources,
	}, nil
}

func newTestResourceUpdaterProvider(diffs chan<- *diff.StackStateDiff,
	statuses chan<- *latest.Stack,
	deleteChildResources chan<- struct{}) *testResourceUpdaterProvider {
	return &testResourceUpdaterProvider{diffs: diffs, statuses: statuses, deleteChildResources: deleteChildResources}
}

type testResourceUpdater struct {
	diffs                chan<- *diff.StackStateDiff
	statuses             chan<- *latest.Stack
	deleteChildResources chan<- struct{}
	stack                *latest.Stack
}

func (u *testResourceUpdater) applyStackDiff(diff *diff.StackStateDiff) error {
	u.diffs <- diff
	return nil
}
func (u *testResourceUpdater) updateStackStatus(status latest.StackStatus) (*latest.Stack, error) {
	if u.stack.Status != nil && *u.stack.Status == status {
		return u.stack, nil
	}
	newStack := u.stack.Clone()
	newStack.Status = &status
	u.statuses <- newStack
	return newStack, nil
}

func (u *testResourceUpdater) deleteSecretsAndConfigMaps() error {
	u.deleteChildResources <- struct{}{}
	return nil
}

type reconciliationTestCaseResult struct {
	diffs                  []*diff.StackStateDiff
	statuses               []*latest.Stack
	childResourceDeletions int
}

func runReconcilierTestCase(originalStack *latest.Stack, defaultServiceType coretypes.ServiceType, operation func(*StackReconciler),
	originalState ...interface{}) (reconciliationTestCaseResult, error) {
	cache := &dummyOwnerCache{
		data: make(map[string]stackOwnerCacheEntry),
	}
	childrenStore, err := newTestChildrenStore(originalState...)
	if err != nil {
		return reconciliationTestCaseResult{}, err
	}
	stackStore := newTestStackStore(originalStack)
	chDiffs := make(chan *diff.StackStateDiff)
	chStatusUpdates := make(chan *latest.Stack)
	chChildrenDeletions := make(chan struct{})
	var (
		wg                        sync.WaitGroup
		producedDiffs             []*diff.StackStateDiff
		producedStatusUpdates     []*latest.Stack
		childrenResourceDeletions int
	)
	wg.Add(3)
	go func() {
		defer wg.Done()
		for d := range chDiffs {
			producedDiffs = append(producedDiffs, d)
		}
	}()
	go func() {
		defer wg.Done()
		for s := range chStatusUpdates {
			producedStatusUpdates = append(producedStatusUpdates, s)
		}
	}()
	go func() {
		defer wg.Done()
		for range chChildrenDeletions {
			childrenResourceDeletions++
		}
	}()
	resourceUpdater := newTestResourceUpdaterProvider(chDiffs, chStatusUpdates, chChildrenDeletions)
	testee, err := NewStackReconciler(stackStore, childrenStore, defaultServiceType, resourceUpdater, cache)
	if err != nil {
		close(chDiffs)
		close(chStatusUpdates)
		close(chChildrenDeletions)
		return reconciliationTestCaseResult{}, err
	}

	operation(testee)

	close(chDiffs)
	close(chStatusUpdates)
	close(chChildrenDeletions)
	wg.Wait()
	return reconciliationTestCaseResult{childResourceDeletions: childrenResourceDeletions, diffs: producedDiffs, statuses: producedStatusUpdates}, nil
}

func runReconciliationTestCase(originalStack *latest.Stack, defaultServiceType coretypes.ServiceType,
	originalState ...interface{}) (reconciliationTestCaseResult, error) {
	return runReconcilierTestCase(originalStack, defaultServiceType, func(testee *StackReconciler) {
		testee.reconcileStack(stackresources.ObjKey(originalStack.Namespace, originalStack.Name))
	}, originalState...)
}

func runRemoveStackTestCase(originalStack *latest.Stack, defaultServiceType coretypes.ServiceType,
	originalState ...interface{}) (reconciliationTestCaseResult, error) {
	return runReconcilierTestCase(originalStack, defaultServiceType, func(testee *StackReconciler) {
		testee.deleteStackChildren(originalStack)
	}, originalState...)
}

func TestRemoveChildren(t *testing.T) {
	originalStack := Stack("test", WithNamespace("test"))
	svc := Service(originalStack, "svc")
	dep := Deployment(originalStack, "dep")
	daemon := Daemonset(originalStack, "daemon")
	stateful := Statefulset(originalStack, "stateful")
	result, err := runRemoveStackTestCase(originalStack, coretypes.ServiceTypeLoadBalancer, svc, dep, daemon, stateful)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(result.statuses))
	assert.Equal(t, 1, len(result.diffs))
	assert.Equal(t, 0, len(result.diffs[0].DaemonsetsToAdd))
	assert.Equal(t, 0, len(result.diffs[0].DaemonsetsToUpdate))
	assert.Equal(t, 0, len(result.diffs[0].DeploymentsToAdd))
	assert.Equal(t, 0, len(result.diffs[0].DeploymentsToUpdate))
	assert.Equal(t, 0, len(result.diffs[0].ServicesToAdd))
	assert.Equal(t, 0, len(result.diffs[0].ServicesToUpdate))
	assert.Equal(t, 0, len(result.diffs[0].StatefulsetsToAdd))
	assert.Equal(t, 0, len(result.diffs[0].StatefulsetsToUpdate))
	assert.Equal(t, 1, len(result.diffs[0].DaemonsetsToDelete))
	assert.Equal(t, 1, len(result.diffs[0].DeploymentsToDelete))
	assert.Equal(t, 1, len(result.diffs[0].ServicesToDelete))
	assert.Equal(t, 1, len(result.diffs[0].StatefulsetsToDelete))
}

func TestCreate(t *testing.T) {
	originalStack := Stack("test",
		WithNamespace("test"),
		WithService("dep",
			Image("nginx")),
		WithService("daemon",
			Image("nginx"),
			Deploy(ModeGlobal),
		),
		WithService("stateful",
			Image("nginx"),
			WithVolume(Volume, Source("volumename"), Target("/data")),
		),
	)
	result, err := runReconciliationTestCase(originalStack, coretypes.ServiceTypeLoadBalancer)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(result.statuses))
	assert.Equal(t, statusProgressing(), *result.statuses[0].Status)
	assert.Equal(t, 1, len(result.diffs))
	assert.Equal(t, 1, len(result.diffs[0].DaemonsetsToAdd))
	assert.Equal(t, 0, len(result.diffs[0].DaemonsetsToUpdate))
	assert.Equal(t, 0, len(result.diffs[0].DaemonsetsToDelete))
	assert.Equal(t, 1, len(result.diffs[0].DeploymentsToAdd))
	assert.Equal(t, 0, len(result.diffs[0].DeploymentsToUpdate))
	assert.Equal(t, 0, len(result.diffs[0].DeploymentsToDelete))
	assert.Equal(t, 3, len(result.diffs[0].ServicesToAdd))
	assert.Equal(t, 0, len(result.diffs[0].ServicesToUpdate))
	assert.Equal(t, 0, len(result.diffs[0].ServicesToDelete))
	assert.Equal(t, 1, len(result.diffs[0].StatefulsetsToAdd))
	assert.Equal(t, 0, len(result.diffs[0].StatefulsetsToUpdate))
	assert.Equal(t, 0, len(result.diffs[0].StatefulsetsToDelete))

	daemon := &result.diffs[0].DaemonsetsToAdd[0]
	deployment := &result.diffs[0].DeploymentsToAdd[0]
	svc0 := &result.diffs[0].ServicesToAdd[0]
	svc1 := &result.diffs[0].ServicesToAdd[1]
	svc2 := &result.diffs[0].ServicesToAdd[2]
	statefulset := &result.diffs[0].StatefulsetsToAdd[0]

	// ensure owner refs populated
	assert.Equal(t, 1, len(daemon.OwnerReferences))
	assert.Equal(t, 1, len(deployment.OwnerReferences))
	assert.Equal(t, 1, len(svc0.OwnerReferences))
	assert.Equal(t, 1, len(svc1.OwnerReferences))
	assert.Equal(t, 1, len(svc2.OwnerReferences))
	assert.Equal(t, 1, len(statefulset.OwnerReferences))

	stack := result.statuses[0]

	// make sure re-reconcile does nothing
	result, err = runReconciliationTestCase(stack, coretypes.ServiceTypeLoadBalancer,
		daemon,
		deployment,
		svc0,
		svc1,
		svc2,
		statefulset)

	assert.NoError(t, err)
	assert.Equal(t, 0, len(result.statuses))
	assert.Equal(t, 1, len(result.diffs))
	assert.True(t, result.diffs[0].Empty())

	// update resources status, to simulate readyness
	daemon.Status.NumberUnavailable = 0
	deployment.Status.ReadyReplicas = 1
	statefulset.Status.ReadyReplicas = 1
	result, err = runReconciliationTestCase(stack, coretypes.ServiceTypeLoadBalancer,
		daemon,
		deployment,
		svc0,
		svc1,
		svc2,
		statefulset)

	assert.NoError(t, err)
	assert.Equal(t, 1, len(result.statuses))
	assert.Equal(t, statusAvailable(), *result.statuses[0].Status)
	assert.Equal(t, 1, len(result.diffs))
	assert.True(t, result.diffs[0].Empty())
}

func TestPendingDeletionStack(t *testing.T) {
	originalStack := Stack("test",
		WithNamespace("test"),
		WithService("dep",
			Image("nginx")),
		WithService("daemon",
			Image("nginx"),
			Deploy(ModeGlobal),
		),
		WithService("stateful",
			Image("nginx"),
			WithVolume(Volume, Source("volumename"), Target("/data")),
		),
	)
	deleteTS := metav1.Now()
	originalStack.DeletionTimestamp = &deleteTS
	result, err := runReconciliationTestCase(originalStack, coretypes.ServiceTypeLoadBalancer)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(result.statuses))
	assert.Equal(t, 1, len(result.diffs))
	assert.True(t, result.diffs[0].Empty())
	assert.Equal(t, 1, result.childResourceDeletions)
}

func TestReplayLogs(t *testing.T) {
	cases := []struct {
		fileName           string
		defaultServicetype coretypes.ServiceType
	}{
		{
			fileName:           "d4d-words.json",
			defaultServicetype: coretypes.ServiceTypeLoadBalancer,
		},
		{
			fileName:           "d4d-words-with-unexpected-webhook.json",
			defaultServicetype: coretypes.ServiceTypeLoadBalancer,
		},
		{
			fileName:           "d4d-with-statefulset.json",
			defaultServicetype: coretypes.ServiceTypeLoadBalancer,
		},
		{
			fileName:           "test-ucp-no-dtr.json",
			defaultServicetype: coretypes.ServiceTypeNodePort,
		},
		{
			fileName:           "test-ucp-dtr-with-dct.json",
			defaultServicetype: coretypes.ServiceTypeNodePort,
		},
		{
			fileName:           "test-various-port-ordering.json",
			defaultServicetype: coretypes.ServiceTypeLoadBalancer,
		},
	}

	for _, c := range cases {
		t.Run(c.fileName, func(t *testing.T) {
			data, err := ioutil.ReadFile(filepath.Join("testcases", c.fileName))
			assert.NoError(t, err)
			var tc TestCase
			assert.NoError(t, json.Unmarshal(data, &tc))
			result, err := runReconciliationTestCase(tc.Stack, c.defaultServicetype, tc.Children.FlattenResources()...)
			assert.NoError(t, err)
			assert.Equal(t, 0, len(result.statuses))
			assert.Equal(t, 1, len(result.diffs))
			assert.True(t, result.diffs[0].Empty())
			if !result.diffs[0].Empty() {
				for _, res := range result.diffs[0].DaemonsetsToUpdate {
					fmt.Printf("daemonset %v changed:\n", res.Name)
					fmt.Println("current:")
					current := tc.Children.Daemonsets[stackresources.ObjKey(res.Namespace, res.Name)]
					data, _ := json.MarshalIndent(&current.Spec, "", "  ")
					fmt.Println(string(data))
					fmt.Println("desired:")
					data, _ = json.MarshalIndent(&res.Spec, "", "  ")
					fmt.Println(string(data))
				}
				for _, res := range result.diffs[0].DeploymentsToUpdate {
					fmt.Printf("deployment %v changed:\n", res.Name)
					fmt.Println("current:")
					current := tc.Children.Deployments[stackresources.ObjKey(res.Namespace, res.Name)]
					data, _ := json.MarshalIndent(&current.Spec, "", "  ")
					fmt.Println(string(data))
					fmt.Println("desired:")
					data, _ = json.MarshalIndent(&res.Spec, "", "  ")
					fmt.Println(string(data))
				}
				for _, res := range result.diffs[0].StatefulsetsToUpdate {
					fmt.Printf("statefulset %v changed:\n", res.Name)
					fmt.Println("current:")
					current := tc.Children.Statefulsets[stackresources.ObjKey(res.Namespace, res.Name)]
					data, _ := json.MarshalIndent(&current.Spec, "", "  ")
					fmt.Println(string(data))
					fmt.Println("desired:")
					data, _ = json.MarshalIndent(&res.Spec, "", "  ")
					fmt.Println(string(data))
				}
				for _, res := range result.diffs[0].ServicesToUpdate {
					fmt.Printf("service %v changed:\n", res.Name)
					fmt.Println("current:")
					current := tc.Children.Services[stackresources.ObjKey(res.Namespace, res.Name)]
					data, _ := json.MarshalIndent(&current.Spec, "", "  ")
					fmt.Println(string(data))
					fmt.Println("desired:")
					data, _ = json.MarshalIndent(&res.Spec, "", "  ")
					fmt.Println(string(data))
				}
			}
		})
	}
}

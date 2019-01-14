package convert

import (
	"errors"
	"sort"
	"strconv"

	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	"github.com/docker/compose-on-kubernetes/api/labels"
	"github.com/docker/compose-on-kubernetes/internal/stackresources"
	log "github.com/sirupsen/logrus"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	expectedGenerationAnnotation = "com.docker.stack.expected-generation"
)

// IsStackDirty indicates if the stack is pending reconciliation
func IsStackDirty(stack *latest.Stack) bool {
	if stack.Status == nil {
		return true
	}
	return stack.Status.Phase == latest.StackReconciliationPending ||
		stack.Status.Phase == latest.StackFailure
}

// StackToStack converts a latest.Stack to a StackDefinition
func StackToStack(stack latest.Stack, strategy ServiceStrategy, original *stackresources.StackState) (*stackresources.StackState, error) {
	if stack.Spec == nil {
		return nil, errors.New("stack spec is nil")
	}
	composeServices := stack.Spec.Services
	// in future we might support stacks with no compose service but only helm or such deployments
	// then we'll need to update this code
	if len(composeServices) == 0 {
		return nil, errors.New("this stack has no service")
	}
	stackDirty := IsStackDirty(&stack)

	log.Debugf("Stack dirtyness check: %v\nStack object: %#v", stackDirty, stack)

	sort.Slice(composeServices, func(i int, j int) bool { return composeServices[i].Name < composeServices[j].Name })

	var resources []interface{}
	for _, srv := range composeServices {
		svcResources, err := toStackResources(stack.Name, stack.Namespace, srv, stack.Spec, strategy, original, stackDirty)
		if err != nil {
			return nil, err
		}

		resources = append(resources, svcResources...)
	}

	return stackresources.NewStackState(resources...)
}

// toStackService creates a Kubernetes stack service out of a swarm service.
func toStackResources(stackName, stackNamespace string, srv latest.ServiceConfig, configuration *latest.StackSpec,
	strategy ServiceStrategy, original *stackresources.StackState, stackDirty bool) ([]interface{}, error) {
	labelSelector := labels.ForService(stackName, srv.Name)
	objectMeta := objectMeta(srv, labelSelector, stackNamespace)

	var resources []interface{}
	headlessService, publishedService, randomPortsService := toServices(srv, objectMeta, labelSelector, strategy, original)
	if headlessService != nil {
		resources = append(resources, headlessService)
	}
	if publishedService != nil {
		resources = append(resources, publishedService)
	}
	if randomPortsService != nil {
		resources = append(resources, randomPortsService)
	}
	objKey := stackresources.ObjKey(objectMeta.Namespace, objectMeta.Name)

	if isGlobal(srv) {
		if hasPersistentVolumes(srv) {
			return nil, errors.New("using persistent volumes in a global service is not supported yet")
		}
		originalSvc := original.Daemonsets[objKey]
		if !stackDirty && generationMatchesExpected(originalSvc.ObjectMeta) {
			log.Debugf("Generation match for daemonset %s, skipping", objKey)
			resources = append(resources, &originalSvc)
		} else {
			podTemplate, err := toPodTemplate(srv, objectMeta.Labels, configuration, originalSvc.Spec.Template)
			if err != nil {
				return nil, err
			}
			res := toDaemonSet(objectMeta, podTemplate, labelSelector, originalSvc)
			setExpectedGeneration(originalSvc.ObjectMeta, &res.ObjectMeta, newSpecPair(&originalSvc.Spec, &res.Spec))
			resources = append(resources, res)
		}
	} else if hasPersistentVolumes(srv) {
		originalSvc := original.Statefulsets[objKey]
		if !stackDirty && generationMatchesExpected(originalSvc.ObjectMeta) {
			log.Debugf("Generation match for statefulset %s, skipping", objKey)
			resources = append(resources, &originalSvc)
		} else {
			podTemplate, err := toPodTemplate(srv, objectMeta.Labels, configuration, originalSvc.Spec.Template)
			if err != nil {
				return nil, err
			}
			res := toStatefulSet(srv, objectMeta, podTemplate, labelSelector, originalSvc)
			setExpectedGeneration(originalSvc.ObjectMeta, &res.ObjectMeta, newSpecPair(&originalSvc.Spec, &res.Spec))
			resources = append(resources, res)
		}
	} else {
		originalSvc := original.Deployments[objKey]
		if !stackDirty && generationMatchesExpected(originalSvc.ObjectMeta) {
			log.Debugf("Generation match for deployment %s, skipping", objKey)
			resources = append(resources, &originalSvc)
		} else {
			podTemplate, err := toPodTemplate(srv, objectMeta.Labels, configuration, originalSvc.Spec.Template)
			if err != nil {
				return nil, err
			}
			res := toDeployment(srv, objectMeta, podTemplate, labelSelector, originalSvc)
			// the deployment api also increment expectedGeneration on annotations changes
			// first we compare specs, and then we check that this could result in a modified annotations map
			setExpectedGeneration(originalSvc.ObjectMeta, &res.ObjectMeta, newSpecPair(&originalSvc.Spec, &res.Spec))
			setExpectedGeneration(originalSvc.ObjectMeta, &res.ObjectMeta, newSpecPair(originalSvc.Annotations, res.Annotations))
			resources = append(resources, res)
		}
	}

	return resources, nil
}

func objectMeta(srv latest.ServiceConfig, labels map[string]string, namespace string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      srv.Name,
		Labels:    mergeLabels(labels, srv.Deploy.Labels),
		Namespace: namespace,
	}
}

func mergeLabels(labelmaps ...map[string]string) map[string]string {
	m := map[string]string{}
	for _, l := range labelmaps {
		for key, value := range l {
			m[key] = value
		}
	}
	return m
}

func generationMatchesExpected(meta metav1.ObjectMeta) bool {
	if meta.Generation < 1 {
		return false
	}
	if meta.Annotations == nil {
		return false
	}
	expected, ok := meta.Annotations[expectedGenerationAnnotation]
	if !ok {
		return false
	}
	expectedValue, err := strconv.ParseInt(expected, 10, 64)
	if err != nil {
		return false
	}
	return expectedValue == meta.Generation
}

type specPair struct {
	original, desired interface{}
}

func newSpecPair(original, desired interface{}) specPair {
	return specPair{
		original: original,
		desired:  desired,
	}
}

func setExpectedGeneration(originalMeta metav1.ObjectMeta, meta *metav1.ObjectMeta, specsToCompare specPair) {
	if meta.Annotations == nil {
		meta.Annotations = make(map[string]string)
	}

	if !apiequality.Semantic.DeepEqual(specsToCompare.original, specsToCompare.desired) {
		meta.Annotations[expectedGenerationAnnotation] = strconv.FormatInt(originalMeta.Generation+1, 10)
	} else {
		meta.Annotations[expectedGenerationAnnotation] = strconv.FormatInt(originalMeta.Generation, 10)
	}
}

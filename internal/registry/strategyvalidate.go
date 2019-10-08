package registry

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/compose-on-kubernetes/internal/stackresources"

	composelabels "github.com/docker/compose-on-kubernetes/api/labels"
	"github.com/docker/compose-on-kubernetes/internal/conversions"
	"github.com/docker/compose-on-kubernetes/internal/convert"
	iv "github.com/docker/compose-on-kubernetes/internal/internalversion"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	coretypes "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// ValidateCollisions against existing objects
func ValidateCollisions(coreClient corev1.ServicesGetter, appsClient appsv1.AppsV1Interface, stack *iv.Stack) field.ErrorList {
	return validateCollisions(coreClient, appsClient)(nil, stack)
}
func validateCollisions(coreClient corev1.ServicesGetter, appsClient appsv1.AppsV1Interface) validateStep {
	return func(_ context.Context, stack *iv.Stack) field.ErrorList {
		if stack.Spec.Stack == nil {
			return field.ErrorList{}
		}

		stackDef, err := convertToStackDefinition(stack)
		if err != nil {
			// something was wrong elsewhere in the validation chain
			return field.ErrorList{}
		}

		var errs field.ErrorList
		for _, v := range stackDef.Services {
			svc, err := coreClient.Services(stack.Namespace).Get(v.Name, metav1.GetOptions{})
			if err == nil {
				errs = appendErrOnCollision(svc.ObjectMeta.Labels, "service", v.Name, stack.Name, errs)
			}
		}
		for _, v := range stackDef.Deployments {
			dep, err := appsClient.Deployments(stack.Namespace).Get(v.Name, metav1.GetOptions{})
			if err == nil {
				errs = appendErrOnCollision(dep.ObjectMeta.Labels, "deployment", v.Name, stack.Name, errs)
			}
		}
		for _, v := range stackDef.Statefulsets {
			ss, err := appsClient.StatefulSets(stack.Namespace).Get(v.Name, metav1.GetOptions{})
			if err == nil {
				errs = appendErrOnCollision(ss.ObjectMeta.Labels, "statefulset", v.Name, stack.Name, errs)
			}
		}
		for _, v := range stackDef.Daemonsets {
			ds, err := appsClient.DaemonSets(stack.Namespace).Get(v.Name, metav1.GetOptions{})
			if err == nil {
				errs = appendErrOnCollision(ds.ObjectMeta.Labels, "daemonset", v.Name, stack.Name, errs)
			}
		}
		return errs
	}
}

func appendErrOnCollision(labels map[string]string, kind string, name string, stackName string, errs field.ErrorList) field.ErrorList {
	res := errs
	if key, ok := labels[composelabels.ForStackName]; ok {
		if key != stackName {
			res = append(res, field.Duplicate(field.NewPath(stackName), fmt.Sprintf("%s %s already exists in stack %s", kind, name, key)))
		}
	} else {
		res = append(res, field.Duplicate(field.NewPath(stackName), fmt.Sprintf("%s %s already exists", kind, name)))
	}
	return res
}

// ValidateObjectNames validates object names
func ValidateObjectNames(stack *iv.Stack) field.ErrorList {
	return validateObjectNames()(nil, stack)
}
func validateObjectNames() validateStep {
	return func(_ context.Context, stack *iv.Stack) field.ErrorList {
		if stack == nil || stack.Spec.Stack == nil {
			return nil
		}
		errs := field.ErrorList{}
		for ix, svc := range stack.Spec.Stack.Services {
			result := validation.IsDNS1123Subdomain(svc.Name)
			if len(result) > 0 {
				errs = append(errs, field.Invalid(field.NewPath("spec", "stack", "services").Index(ix).Child("name"),
					svc.Name,
					"not a valid service name in Kubernetes: "+strings.Join(result, ", ")))
			}
			for i, volume := range svc.Volumes {
				// FIXME(vdemeester) deduplicate this with internal/convert
				volumename := fmt.Sprintf("mount-%d", i)
				if volume.Type == "volume" && volume.Source != "" {
					volumename = volume.Source
				}
				result = validation.IsDNS1123Subdomain(volumename)
				if len(result) > 0 {
					errs = append(errs, field.Invalid(field.NewPath("spec", "stack", "services").Index(ix).Child("volumes").Index(i),
						volumename,
						"not a valid volume name in Kubernetes: "+strings.Join(result, ", ")))
				}
			}
		}
		for secret := range stack.Spec.Stack.Secrets {
			result := validation.IsDNS1123Subdomain(secret)
			if len(result) > 0 {
				errs = append(errs, field.Invalid(field.NewPath("spec", "stack", "secrets").Child("secret"),
					secret,
					"not a valid secret name in Kubernetes: "+strings.Join(result, ", ")))
			}
		}
		return errs
	}
}

// ValidateDryRun validates that conversion to k8s objects works well
func ValidateDryRun(stack *iv.Stack) field.ErrorList {
	return validateDryRun()(nil, stack)
}
func validateDryRun() validateStep {
	return func(_ context.Context, stack *iv.Stack) field.ErrorList {
		if _, err := convertToStackDefinition(stack); err != nil {
			return field.ErrorList{
				field.Invalid(field.NewPath(stack.Name), nil, err.Error()),
			}
		}
		return nil
	}
}

func convertToStackDefinition(stack *iv.Stack) (*stackresources.StackState, error) {
	stackLatest, err := conversions.StackFromInternalV1alpha3(stack)
	if err != nil {
		return nil, errors.Wrap(err, "conversion to v1alpha3 failed")
	}
	strategy, err := convert.ServiceStrategyFor(coretypes.ServiceTypeLoadBalancer) // in that case, service strategy does not really matter
	if err != nil {
		log.Errorf("Failed to convert to stack: %s", err)
		if err != nil {
			return nil, errors.Wrap(err, "conversion to kube entities failed")
		}
	}
	sd, err := convert.StackToStack(*stackLatest, strategy, stackresources.EmptyStackState)
	if err != nil {
		log.Errorf("Failed to convert to stack: %s", err)
		if err != nil {
			return nil, errors.Wrap(err, "conversion to kube entities failed")
		}
	}
	return sd, nil
}

func validateCreationStatus() validateStep {
	return func(_ context.Context, stack *iv.Stack) field.ErrorList {
		if stack.Status != nil && stack.Status.Phase == iv.StackFailure {
			return field.ErrorList{
				field.Invalid(field.NewPath(stack.Name), nil, stack.Status.Message),
			}
		}
		return nil
	}
}

func validateStackNotNil() validateStep {
	return func(_ context.Context, stack *iv.Stack) field.ErrorList {
		if stack.Spec.Stack == nil {
			// in this case, the status should have been filled with error message
			msg := "stack is empty"
			if stack.Status != nil {
				msg = stack.Status.Message
			}
			return field.ErrorList{
				field.Invalid(field.NewPath(stack.Name), nil, msg),
			}
		}
		return nil
	}
}

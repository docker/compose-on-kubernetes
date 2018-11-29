package registry

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/docker/compose-on-kubernetes/internal/conversions"
	iv "github.com/docker/compose-on-kubernetes/internal/internalversion"
	"github.com/docker/compose-on-kubernetes/internal/parsing"
	log "github.com/sirupsen/logrus"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
)

func prepareStackOwnership() prepareStep {
	return func(ctx context.Context, _, stack *iv.Stack) error {
		if user, ok := genericapirequest.UserFrom(ctx); ok {
			stack.Spec.Owner.UserName = user.GetName()
			stack.Spec.Owner.Groups = user.GetGroups()
			stack.Spec.Owner.Extra = user.GetExtra()
			log.Debugf("Set stack owner to %s", stack.Spec.Owner.UserName)
			return nil
		}
		return errors.New("can't extract owner information from request")
	}
}

func setFieldsForV1beta1Update(oldStack, newStack *iv.Stack) {
	// mark stack as dirty if composfile has been updated
	// if not, avoid re-parsing the same compose file
	if newStack.Spec.ComposeFile != oldStack.Spec.ComposeFile {
		// composefile was changed, drop the stack (will be reconciled just after)
		newStack.Spec.Stack = nil
	} else {
		// The update likely dropped v1beta2 fields
		newStack.Spec.Stack = oldStack.Spec.Stack
	}
}
func setFieldsForV1beta2Update(oldStack, newStack *iv.Stack) {
	// check if composefile is still up to date
	// if the stack itself has been updated, we add a comment at compose file head
	// warning the user that it is not in sync anymore with the structured stack
	newStack.Spec.ComposeFile = oldStack.Spec.ComposeFile
	specSame := reflect.DeepEqual(newStack.Spec.Stack, oldStack.Spec.Stack)
	if !specSame && !strings.HasPrefix(newStack.Spec.ComposeFile, composeOutOfDate) && newStack.Spec.ComposeFile != "" {
		newStack.Spec.ComposeFile = composeOutOfDate + newStack.Spec.ComposeFile
	}
}

type fieldUpdatesFunc func(oldStack, newStack *iv.Stack)

var fieldUpdatesFuncs = map[APIVersion]fieldUpdatesFunc{
	APIV1beta1: setFieldsForV1beta1Update,
	APIV1beta2: setFieldsForV1beta2Update,
}

func prepareFieldsForUpdate(version APIVersion) prepareStep {
	if f, ok := fieldUpdatesFuncs[version]; ok {
		return func(_ context.Context, oldStack, newStack *iv.Stack) error {
			if oldStack != nil {
				f(oldStack, newStack)
			}
			return nil
		}
	}
	return func(_ context.Context, _, _ *iv.Stack) error {
		return fmt.Errorf("unknown APIVersion %q", version)
	}

}

func prepareStackFromComposefile(relaxedParsing bool) prepareStep {
	return func(_ context.Context, _, newStack *iv.Stack) error {
		if newStack.Spec.Stack == nil {
			// creation or update from composefile subresource or from v1beta1 API
			composeConfig, err := parsing.LoadStackData([]byte(newStack.Spec.ComposeFile), nil)
			if err != nil {
				if relaxedParsing {
					newStack.Status = &iv.StackStatus{
						Phase:   iv.StackFailure,
						Message: fmt.Sprintf("parsing error: %s", err),
					}
					return nil
				}
				return fmt.Errorf("parsing error: %s", err)
			}
			newStack.Spec.Stack = conversions.FromComposeConfig(composeConfig)
		}
		return nil
	}
}

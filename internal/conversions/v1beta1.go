package conversions

import (
	"github.com/docker/compose-on-kubernetes/api/compose/v1beta1"
	"github.com/docker/compose-on-kubernetes/internal/internalversion"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
)

// RegisterV1beta1Conversions adds conversion functions to the given scheme.
// Public to allow building arbitrary schemes.
func RegisterV1beta1Conversions(scheme *runtime.Scheme) error {
	if err := scheme.AddConversionFunc((*v1beta1.Owner)(nil), (*internalversion.Owner)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return ownerToInternalV1beta1(a.(*v1beta1.Owner), b.(*internalversion.Owner), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*internalversion.Owner)(nil), (*v1beta1.Owner)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return ownerFromInternalV1beta1(a.(*internalversion.Owner), b.(*v1beta1.Owner), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*v1beta1.Stack)(nil), (*internalversion.Stack)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return stackToInternalV1beta1(a.(*v1beta1.Stack), b.(*internalversion.Stack), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*internalversion.Stack)(nil), (*v1beta1.Stack)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return stackFromInternalV1beta1(a.(*internalversion.Stack), b.(*v1beta1.Stack), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*v1beta1.StackList)(nil), (*internalversion.StackList)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return stackListToInternalV1beta1(a.(*v1beta1.StackList), b.(*internalversion.StackList), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*internalversion.StackList)(nil), (*v1beta1.StackList)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return stackListFromInternalV1beta1(a.(*internalversion.StackList), b.(*v1beta1.StackList), scope)
	}); err != nil {
		return err
	}

	return nil
}

func ownerToInternalV1beta1(in *v1beta1.Owner, out *internalversion.Owner, _ conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	out.Owner = in.Owner
	return nil
}

func ownerFromInternalV1beta1(in *internalversion.Owner, out *v1beta1.Owner, _ conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	out.Owner = in.Owner
	return nil
}

func stackToInternalV1beta1(in *v1beta1.Stack, out *internalversion.Stack, _ conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	out.Spec.ComposeFile = in.Spec.ComposeFile
	out.Status = &internalversion.StackStatus{
		Message: in.Status.Message,
		Phase:   internalversion.StackPhase(in.Status.Phase),
	}
	return nil
}

func stackFromInternalV1beta1(in *internalversion.Stack, out *v1beta1.Stack, _ conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	out.Spec.ComposeFile = in.Spec.ComposeFile
	if in.Status != nil {
		out.Status.Message = in.Status.Message
		out.Status.Phase = v1beta1.StackPhase(in.Status.Phase)
	}
	return nil
}

func stackListToInternalV1beta1(in *v1beta1.StackList, out *internalversion.StackList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		inSlice, outSlice := &in.Items, &out.Items
		*outSlice = make([]internalversion.Stack, len(*inSlice))
		for i := range *inSlice {
			if err := stackToInternalV1beta1(&(*inSlice)[i], &(*outSlice)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func stackListFromInternalV1beta1(in *internalversion.StackList, out *v1beta1.StackList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		inSlice, outSlice := &in.Items, &out.Items
		*outSlice = make([]v1beta1.Stack, len(*inSlice))
		for i := range *inSlice {
			if err := stackFromInternalV1beta1(&(*inSlice)[i], &(*outSlice)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = make([]v1beta1.Stack, 0)
	}
	return nil
}

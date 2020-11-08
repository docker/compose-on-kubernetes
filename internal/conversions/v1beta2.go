package conversions

import (
	"github.com/docker/compose-on-kubernetes/api/compose/v1alpha3"
	"github.com/docker/compose-on-kubernetes/api/compose/v1beta2"
	"github.com/docker/compose-on-kubernetes/internal/internalversion"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
)

// RegisterV1beta2Conversions adds conversion functions to the given scheme.
// Public to allow building arbitrary schemes.
func RegisterV1beta2Conversions(scheme *runtime.Scheme) error {
	if err := scheme.AddConversionFunc((*v1beta2.Owner)(nil), (*internalversion.Owner)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return ownerToInternalV1beta2(a.(*v1beta2.Owner), b.(*internalversion.Owner), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*internalversion.Owner)(nil), (*v1beta2.Owner)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return ownerFromInternalV1beta2(a.(*internalversion.Owner), b.(*v1beta2.Owner), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*v1beta2.Stack)(nil), (*internalversion.Stack)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return stackToInternalV1beta2(a.(*v1beta2.Stack), b.(*internalversion.Stack), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*internalversion.Stack)(nil), (*v1beta2.Stack)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return stackFromInternalV1beta2(a.(*internalversion.Stack), b.(*v1beta2.Stack), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*v1beta2.StackList)(nil), (*internalversion.StackList)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return stackListToInternalV1beta2(a.(*v1beta2.StackList), b.(*internalversion.StackList), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*internalversion.StackList)(nil), (*v1beta2.StackList)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return stackListFromInternalV1beta2(a.(*internalversion.StackList), b.(*v1beta2.StackList), scope)
	}); err != nil {
		return err
	}

	return nil
}

func ownerToInternalV1beta2(in *v1beta2.Owner, out *internalversion.Owner, _ conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	out.Owner = in.Owner
	return nil
}

func ownerFromInternalV1beta2(in *internalversion.Owner, out *v1beta2.Owner, _ conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	out.Owner = in.Owner
	return nil
}

func stackToInternalV1beta2(in *v1beta2.Stack, out *internalversion.Stack, s conversion.Scope) error {
	var alpha3 v1alpha3.Stack
	if err := v1alpha3.Convert_v1beta2_Stack_To_v1alpha3_Stack(in, &alpha3, s); err != nil {
		return err
	}
	return stackToInternalV1alpha3(&alpha3, out, s)
}

func stackFromInternalV1beta2(in *internalversion.Stack, out *v1beta2.Stack, s conversion.Scope) error {
	var alpha3 v1alpha3.Stack
	if err := stackFromInternalV1alpha3(in, &alpha3, s); err != nil {
		return err
	}
	return v1alpha3.Convert_v1alpha3_Stack_To_v1beta2_Stack(&alpha3, out, s)
}

func stackListToInternalV1beta2(in *v1beta2.StackList, out *internalversion.StackList, s conversion.Scope) error {
	var alpha3 v1alpha3.StackList
	if err := v1alpha3.Convert_v1beta2_StackList_To_v1alpha3_StackList(in, &alpha3, s); err != nil {
		return err
	}
	return stackListToInternalV1alpha3(&alpha3, out, s)
}

func stackListFromInternalV1beta2(in *internalversion.StackList, out *v1beta2.StackList, s conversion.Scope) error {
	var alpha3 v1alpha3.StackList
	if err := stackListFromInternalV1alpha3(in, &alpha3, s); err != nil {
		return err
	}
	return v1alpha3.Convert_v1alpha3_StackList_To_v1beta2_StackList(&alpha3, out, s)
}

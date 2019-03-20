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
	return scheme.AddConversionFuncs(
		ownerToInternalV1beta2,
		ownerFromInternalV1beta2,
		stackToInternalV1beta2,
		stackFromInternalV1beta2,
		stackListToInternalV1beta2,
		stackListFromInternalV1beta2,
	)
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

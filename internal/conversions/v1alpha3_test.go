package conversions

import (
	"testing"

	"github.com/docker/compose-on-kubernetes/api/compose/impersonation"
	"github.com/docker/compose-on-kubernetes/api/compose/v1alpha3"
	"github.com/docker/compose-on-kubernetes/internal/internalversion"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestRegisterV1alpha3Conversions(t *testing.T) {
	scheme := runtime.NewScheme()
	RegisterV1alpha3Conversions(scheme)

	v1alpha3Owner := &v1alpha3.Owner{
		ObjectMeta: metav1.ObjectMeta{
			Name: "owner",
		},
		Owner: impersonation.Config{
			UserName: "foo",
		},
	}
	internalversionOwner := &internalversion.Owner{}
	scheme.Convert(v1alpha3Owner, internalversionOwner, nil)
	assert.Equal(t, "owner", internalversionOwner.ObjectMeta.Name)
	assert.Equal(t, "foo", internalversionOwner.Owner.UserName)

	internalversionOwner.Owner.UserName = "bar"
	scheme.Convert(internalversionOwner, v1alpha3Owner, nil)
	assert.Equal(t, "bar", v1alpha3Owner.Owner.UserName)
}

package install

import (
	"testing"

	"github.com/stretchr/testify/assert"
	k8sVersion "k8s.io/apimachinery/pkg/version"
)

func TestCheckVersion(t *testing.T) {
	assert.NoError(t, checkVersion(&k8sVersion.Info{Major: "1", Minor: "8"}, "1.8"))
	assert.NoError(t, checkVersion(&k8sVersion.Info{Major: "1", Minor: "8+"}, "1.8"))
	assert.NoError(t, checkVersion(&k8sVersion.Info{Major: "1", Minor: "8.1"}, "1.8"))
	assert.NoError(t, checkVersion(&k8sVersion.Info{Major: "1", Minor: "9"}, "1.8"))
	assert.NoError(t, checkVersion(&k8sVersion.Info{Major: "2", Minor: "0"}, "1.8"))
}

func TestCheckInvalidVersion(t *testing.T) {
	assert.EqualError(t, checkVersion(&k8sVersion.Info{Major: "1", Minor: "7"}, "1.8"), "unsupported server version: 1.7 < 1.8")
	assert.EqualError(t, checkVersion(&k8sVersion.Info{Major: "1", Minor: "7+"}, "1.8"), "unsupported server version: 1.7+ < 1.8")
	assert.EqualError(t, checkVersion(&k8sVersion.Info{Major: "1", Minor: "8-beta1"}, "1.8"), "unsupported server version: 1.8-beta1 < 1.8")

	assert.EqualError(t, checkVersion(&k8sVersion.Info{Major: "X", Minor: "Y"}, "1.8"), "unsupported server version: X.Y: Invalid Semantic Version")
}

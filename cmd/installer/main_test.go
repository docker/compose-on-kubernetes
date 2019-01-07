package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1types "k8s.io/api/core/v1"
)

// TestParsePullPolicyValid verifies that parsing a valid pull policy works
func TestParsePullPolicyValid(t *testing.T) {
	for _, pp := range []corev1types.PullPolicy{corev1types.PullAlways, corev1types.PullNever, corev1types.PullIfNotPresent} {
		res, err := parsePullPolicy(string(pp))
		assert.NoError(t, err)
		assert.Equal(t, res, pp)
	}
}

// TestParsePullPolicyInValid verifies that parsing an invalid pull policy
// triggers an error
func TestParsePullPolicyInValid(t *testing.T) {
	for _, pp := range []string{"always", "invalid", ""} {
		res, err := parsePullPolicy(string(pp))
		assert.EqualError(t, err, fmt.Sprintf("invalid pull policy: % q", pp))
		assert.Empty(t, res)
	}
}

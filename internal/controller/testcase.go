package controller

import (
	"github.com/docker/compose-on-kubernetes/api/compose/v1beta2"
	"github.com/docker/compose-on-kubernetes/internal/stackresources"
)

// TestCase is a serializable type used to combine a stack and its children for a record & replay test scenario
type TestCase struct {
	Stack    *v1beta2.Stack
	Children *stackresources.StackState
}

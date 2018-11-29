package rollout

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFind(t *testing.T) {
	revisions := StackRevisions{{2, "spec1"}, {4, "spec2"}}

	found, present := revisions.Find(4)

	assert.True(t, present)
	assert.Equal(t, uint64(4), found.Revision)
	assert.Equal(t, "spec2", found.Spec)
}

func TestFindNotFound(t *testing.T) {
	revisions := StackRevisions{}

	_, present := revisions.Find(42)

	assert.False(t, present)
}

func TestLast(t *testing.T) {
	revisions := StackRevisions{{2, "spec1"}, {4, "spec2"}}

	last, present := revisions.Last()

	assert.True(t, present)
	assert.Equal(t, uint64(4), last.Revision)
	assert.Equal(t, "spec2", last.Spec)
}

func TestLastEmpty(t *testing.T) {
	revisions := StackRevisions{}

	_, found := revisions.Last()

	assert.False(t, found)
}

func TestAdd(t *testing.T) {
	revisions := StackRevisions{{2, "spec1"}, {4, "spec2"}}

	updated := revisions.Add("spec4")

	assert.Equal(t, StackRevisions{{2, "spec1"}, {4, "spec2"}, {5, "spec4"}}, updated)
}

func TestAddFirst(t *testing.T) {
	revisions := StackRevisions{}

	updated := revisions.Add("spec4")

	assert.Equal(t, StackRevisions{{1, "spec4"}}, updated)
}

func TestPatchForRevisionsWithoutAnnotations(t *testing.T) {
	patch, err := PatchForRevisions(nil, nil)

	assert.NoError(t, err)
	assert.Equal(t, "[]", string(patch))
}

func TestPatchForRevisionAnnotations(t *testing.T) {
	patch, err := PatchForRevisions(
		StackRevisions{{1, "spec1"}, {2, "spec2"}},
		StackRevisions{{2, "spec2"}, {3, "spec3"}},
	)

	assert.NoError(t, err)
	assert.Equal(t, `[`+
		`{"op":"remove","path":"/metadata/annotations/stack.compose.docker.com/revision-1"},`+
		`{"op":"add","path":"/metadata/annotations","value":{"stack.compose.docker.com/revision-3":"spec3"}}`+
		`]`,
		string(patch),
	)
}

func TestPatchForRevisionFail(t *testing.T) {
	_, err := PatchForRevisions(
		StackRevisions{{1, "spec"}},
		StackRevisions{{1, "NEW"}},
	)

	assert.Error(t, err)
}

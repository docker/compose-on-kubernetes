package rollout

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	apiv1beta2 "github.com/docker/compose-on-kubernetes/api/compose/v1beta2"
	"github.com/docker/compose-on-kubernetes/internal/patch"
)

const annotationPrefix = "stack.compose.docker.com/revision-"

// StackRevision is an indexed stack spec revision.
type StackRevision struct {
	Revision uint64
	Spec     string
}

// StackRevisions is a list of revisions.
type StackRevisions []StackRevision

// Revisions returns all the past revisions of the given stack.
func Revisions(stack *apiv1beta2.Stack) StackRevisions {
	var revisions []StackRevision

	for k, v := range stack.GetAnnotations() {
		if !strings.HasPrefix(k, annotationPrefix) {
			continue
		}

		revision, err := strconv.Atoi(k[len(annotationPrefix):])
		if err != nil {
			continue
		}

		revisions = append(revisions, StackRevision{
			Revision: uint64(revision),
			Spec:     v,
		})
	}

	sort.Slice(revisions, func(i, j int) bool { return revisions[i].Revision < revisions[j].Revision })

	return revisions
}

// Add adds a revision to a list.
func (r StackRevisions) Add(spec string) StackRevisions {
	var mostRecentRevision uint64
	last, present := r.Last()
	if present {
		mostRecentRevision = last.Revision
	}

	return append(r, StackRevision{
		Revision: mostRecentRevision + 1,
		Spec:     spec,
	})
}

// Find returns the revision for the given index.
func (r StackRevisions) Find(revision uint64) (StackRevision, bool) {
	for _, rev := range r {
		if rev.Revision == revision {
			return rev, true
		}
	}

	return StackRevision{}, false
}

// Last returns the last revision.
func (r StackRevisions) Last() (StackRevision, bool) {
	if len(r) == 0 {
		return StackRevision{}, false
	}

	return r[len(r)-1], true
}

// PatchForRevisions compute the JSONPatch diff between two stack revision lists.
func PatchForRevisions(current, updated StackRevisions) ([]byte, error) {
	patch := patch.New()

	for _, rev := range current {
		if _, present := updated.Find(rev.Revision); !present {
			patch = patch.Remove(fmt.Sprintf("/metadata/annotations/%s%d", annotationPrefix, rev.Revision))
		}
	}

	for _, rev := range updated {
		if r, present := current.Find(rev.Revision); !present {
			patch = patch.AddKV("/metadata/annotations", fmt.Sprintf("%s%d", annotationPrefix, rev.Revision), rev.Spec)
		} else if r.Spec != rev.Spec {
			return nil, fmt.Errorf("Changing a revision (%d) is not supported", rev.Revision)
		}
	}

	return patch.ToJSON()
}

package patch

import "encoding/json"

// Patch describes a JSONPatch operations.
type Patch []operation

type operation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

// New creates a new patch.
func New() Patch {
	return Patch{}
}

// Replace adds a "replace" operation to the patch.
func (p Patch) Replace(path string, value interface{}) Patch {
	return append(p, operation{
		Op:    "replace",
		Path:  path,
		Value: value,
	})
}

// Remove adds a "remove" operation to the patch.
func (p Patch) Remove(path string) Patch {
	return append(p, operation{
		Op:   "remove",
		Path: path,
	})
}

// Add adds a "Add" operation to the patch.
func (p Patch) Add(path string, value interface{}) Patch {
	return append(p, operation{
		Op:    "add",
		Path:  path,
		Value: value,
	})
}

// AddKV adds a "add" operation to the patch.
func (p Patch) AddKV(path, key string, value interface{}) Patch {
	return append(p, operation{
		Op:   "add",
		Path: path,
		Value: map[string]interface{}{
			key: value,
		},
	})
}

// ToJSON converts the patch to json.
func (p Patch) ToJSON() ([]byte, error) {
	return json.Marshal(p)
}

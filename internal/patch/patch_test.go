package patch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToJSON(t *testing.T) {
	buf, err := New().
		Replace("/path1", "value").
		Remove("/path2").
		AddKV("/path3", "key", "value").
		ToJSON()

	assert.NoError(t, err)
	assert.EqualValues(t, `[`+
		`{"op":"replace","path":"/path1","value":"value"},`+
		`{"op":"remove","path":"/path2"},`+
		`{"op":"add","path":"/path3","value":{"key":"value"}}`+
		`]`, buf)
}

func TestToJSONEmpty(t *testing.T) {
	buf, err := New().ToJSON()

	assert.NoError(t, err)
	assert.EqualValues(t, "[]", buf)
}

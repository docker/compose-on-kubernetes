package install

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenTagShouldOuptutAtMost63Chars(t *testing.T) {
	res := tagForCustomImages("foo", "bar")
	assert.True(t, len(res) <= 63, "%d is too long for a kubernetes label", len(res))
}

package parsing

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVeryLargeStillLegitComposefile(t *testing.T) {
	data, err := ioutil.ReadFile("very-large-composefile.yml")
	assert.NoError(t, err)
	err = validateYAML(data)
	assert.NoError(t, err)
}

func TestYamlBomb(t *testing.T) {
	data := []byte(`version: "3"
services: &services ["lol","lol","lol","lol","lol","lol","lol","lol","lol"]
b: &b [*services,*services,*services,*services,*services,*services,*services,*services,*services]
c: &c [*b,*b,*b,*b,*b,*b,*b,*b,*b]
d: &d [*c,*c,*c,*c,*c,*c,*c,*c,*c]
e: &e [*d,*d,*d,*d,*d,*d,*d,*d,*d]
f: &f [*e,*e,*e,*e,*e,*e,*e,*e,*e]
g: &g [*f,*f,*f,*f,*f,*f,*f,*f,*f]
h: &h [*g,*g,*g,*g,*g,*g,*g,*g,*g]
i: &i [*h,*h,*h,*h,*h,*h,*h,*h,*h]`)
	err := validateYAML(data)
	assert.EqualError(t, err, "yaml: exceeded max number of decoded values (100000)")
}

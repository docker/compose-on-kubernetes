package parsing

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
	_, err := LoadStackData(data, nil)
	assert.EqualError(t, err, "yaml: document contains excessive aliasing")
}

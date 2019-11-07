package apiserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

func supportsProtobufCodec(c serializer.CodecFactory) bool {
	for _, v := range c.SupportedMediaTypes() {
		if v.MediaType == runtime.ContentTypeProtobuf {
			return true
		}
	}
	return false
}

func TestRemoveProtobuf(t *testing.T) {
	codecs := serializer.NewCodecFactory(Scheme)
	assert.True(t, supportsProtobufCodec(codecs))
	removeProtobufMediaType(&codecs)
	assert.False(t, supportsProtobufCodec(codecs))
}

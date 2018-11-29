package parsing

import (
	"bytes"

	"github.com/docker/cli/cli/compose/loader"
	composetypes "github.com/docker/cli/cli/compose/types"
	"gopkg.in/yaml.v2"
)

const maxDecodedValues = 100000 // tested with yaml-bomb case, yields about 4MB consumption before error, and seems more than large enough for compose cases
// This mainly guards from YAML bombs
func validateYAML(buf []byte) error {
	d := yaml.NewDecoder(bytes.NewBuffer(buf), yaml.WithLimitDecodedValuesCount(maxDecodedValues))
	var v map[interface{}]interface{}
	return d.Decode(&v)
}

// LoadStackData loads a stack from its []byte representation
func LoadStackData(binary []byte, env map[string]string) (*composetypes.Config, error) {
	if err := validateYAML(binary); err != nil {
		return nil, err
	}
	parsed, err := loader.ParseYAML(binary)
	if err != nil {
		return nil, err
	}
	return loader.Load(composetypes.ConfigDetails{
		WorkingDir: ".",
		ConfigFiles: []composetypes.ConfigFile{
			{
				Config: parsed,
			},
		},
		Environment: env,
	})
}

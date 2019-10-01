package parsing

import (
	"github.com/docker/cli/cli/compose/loader"
	composetypes "github.com/docker/cli/cli/compose/types"
)

// LoadStackData loads a stack from its []byte representation
func LoadStackData(binary []byte, env map[string]string) (*composetypes.Config, error) {
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

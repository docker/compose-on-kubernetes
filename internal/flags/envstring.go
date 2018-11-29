package flags

import (
	"flag"
	"os"

	"github.com/spf13/pflag"
)

type envString struct {
	isSet         bool
	defaultValue  string
	explicitValue string
	envVarName    string
}

// Flag represents the flag registration
type Flag interface {
	String() string
}

func (v *envString) String() string {
	if v.isSet {
		return v.explicitValue
	}
	if envValue, ok := os.LookupEnv(v.envVarName); ok {
		return envValue
	}
	return v.defaultValue
}

func (v *envString) Set(value string) error {
	v.isSet = true
	v.explicitValue = value
	return nil
}

func (v *envString) Type() string {
	return "string"
}

// EnvStringCobra register an environment-backed string variable to the flagset
func EnvStringCobra(flags *pflag.FlagSet, name, defaultValue, envVarName, usage string) *pflag.Flag {
	return flags.VarPF(&envString{defaultValue: defaultValue, envVarName: envVarName}, name, "", usage)
}

// EnvString register an environment-backed string variable to the go runtime flag set
func EnvString(name, defaultValue, envVarName, usage string) Flag {
	f := &envString{defaultValue: defaultValue, envVarName: envVarName}
	flag.Var(f, name, usage)
	return f
}

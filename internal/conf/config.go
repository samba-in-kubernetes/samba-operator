package conf

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// OperatorConfig is a type holding general configuration values.
// Most of the operator code that needs to reference configuration
// should do so via this type.
type OperatorConfig struct {
	SmbdContainerImage string `mapstructure:"smbd-container-image"`
	SmbdContainerName  string `mapstructure:"smbd-container-name"`
}

// Source is how external configuration sources populate the operator config.
type Source struct {
	v    *viper.Viper
	fset *pflag.FlagSet
}

// NewSource creates a new Source based on default configuration values.
func NewSource() *Source {
	v := viper.New()
	v.SetDefault("smbd-container-image", "quay.io/samba.org/samba-server:latest")
	v.SetDefault("smbd-container-name", "samba")
	return &Source{v: v}
}

// Flags returns a pflag FlagSet populated with flags based on the default
// configuration. If used, flags allow changing configuration values on
// the CLI.
// Once parsed these flags act as a configuration source.
func (s *Source) Flags() *pflag.FlagSet {
	if s.fset != nil {
		return s.fset
	}
	s.fset = pflag.NewFlagSet("conf", pflag.ExitOnError)
	for _, k := range s.v.AllKeys() {
		s.fset.String(k, "",
			fmt.Sprintf("Specify the %q configuration parameter", k))
	}
	return s.fset
}

// Read a new OperatorConfig from all available sources.
func (s *Source) Read() (*OperatorConfig, error) {
	v := s.v

	// we look in /etc/samba-operator and the working dir for
	// yaml/toml/etc config files (none are required)
	v.AddConfigPath("/etc/samba-operator")
	v.AddConfigPath(".")
	v.SetConfigName("samba-operator")
	err := v.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	// we automatically pull from the environment
	v.SetEnvPrefix("SAMBA_OP")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()

	// use cli flags if available
	if s.fset != nil {
		v.BindPFlags(s.fset)
	}

	// we isolate config handling to this package. thus we marshal
	// our config to the public OperatorConfig type and return that.
	c := &OperatorConfig{}
	if err := v.Unmarshal(c); err != nil {
		return nil, err
	}
	return c, nil
}

// Package conf defines the operator's configuration parameters.
package conf

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	// ClusterTypeDefault defines the default value for cluster type
	ClusterTypeDefault = "default"
	// ClusterTypeOpenShift defines the type-name for OpenShift clusters
	ClusterTypeOpenShift = "openshift"
)

// DefaultOperatorConfig holds the default values of OperatorConfig.
var DefaultOperatorConfig = OperatorConfig{
	SmbdContainerImage:        "quay.io/samba.org/samba-server:v0.5",
	SmbdMetricsContainerImage: "quay.io/samba.org/samba-metrics:v0.5",
	SvcWatchContainerImage:    "quay.io/samba.org/svcwatch:v0.3",
	SmbdContainerName:         "samba",
	WinbindContainerName:      "wb",
	WorkingNamespace:          "",
	SambaDebugLevel:           "",
	StatePVCSize:              "1Gi",
	ClusterSupport:            "",
	SmbServicePort:            445,
	SmbdPort:                  445,
	MetricsExporterMode:       "disabled",
	ImagePullPolicy:           "IfNotPresent",
	DefaultNodeSelector:       "",
	ClusterType:               "",
}

// OperatorConfig is a type holding general configuration values.
// Most of the operator code that needs to reference configuration
// should do so via this type.
type OperatorConfig struct {
	// SmbdContainerImage can be used to select alternate container sources.
	SmbdContainerImage string `mapstructure:"smbd-container-image"`
	// SmbdMetricsContainerImage can be used to select alternate
	// metrics-exporter container sources.
	SmbdMetricsContainerImage string `mapstructure:"smbd-metrics-container-image"`
	// SvcWatchContainerImage can be used to select alternate container image
	// for the service watch utility.
	SvcWatchContainerImage string `mapstructure:"svc-watch-container-image"`
	// SmbdContainerName can be used to set the name of the primary container,
	// the one running smbd, in the pod.
	SmbdContainerName string `mapstructure:"smbd-container-name"`
	// WinbindContainerName can be used to the the name of the container
	// running winbind.
	WinbindContainerName string `mapstructure:"winbind-container-name"`
	// WorkingNamespace defines the namespace the operator will (generally)
	// make changes in.
	WorkingNamespace string `mapstructure:"working-namespace"`
	// SambaDebugLevel can be used to set debugging level for samba
	// components in deployed containers.
	SambaDebugLevel string `mapstructure:"samba-debug-level"`
	// StatePVCSize is a (string) value that indicates how large the operator
	// should request shared state (not data!) PVCs.
	StatePVCSize string `mapstructure:"state-pvc-size"`
	// ClusterSupport is a (string) value that indicates if the operator
	// will be allowed to set up clustered instances.
	ClusterSupport string `mapstructure:"cluster-support"`
	// SmbServicePort is an (integer) value that defines the port number which
	// the kubernetes service exports
	SmbServicePort int `mapstructure:"smb-service-port"`
	// SmbdPort is an (integer) value that defines the port number on which
	// smbd binds and serve.
	SmbdPort int `mapstructure:"smbd-port"`
	// ServiceAccountName is a (string) which overrides the default service
	// account associated with child pods. Required in OpenShift.
	ServiceAccountName string `mapstructure:"service-account-name"`
	// MetricsExporterMode is a (string) flag which indicates if and how the
	// operator should run metrics-exporter container within samba-server pod.
	// Valid values are "enabled", "disabled" or empty string (default).
	MetricsExporterMode string `mapstructure:"metrics-exporter-mode"`
	// PodName is a (string) which defines the currnt operator pod name.
	PodName string `mapstructure:"pod-name"`
	// PodNamespace is a (string) which defines the currnt operator namespace.
	PodNamespace string `mapstructure:"pod-namespace"`
	// PodIP is a (string) which defines the currnt pod cluster-ip.
	PodIP string `mapstructure:"pod-ip"`
	// ImagePullPolicy is a (string) value which defines the image-download
	// strategy of samba containers.
	ImagePullPolicy string `mapstructure:"image-pull-policy"`
	// DefaultNodeSelector is a string value containing JSON which defines
	// a set of key-value pairs that will be used for all default node
	// selection. If left blank, internal defaults will be used.
	DefaultNodeSelector string `mapstructure:"default-node-selector"`
	// ClusterType is a string which defines the type of underlying K8S
	// cluster (minikube, OpenShift etc). If not provided, the operator will
	// try to figure it out.
	ClusterType string `mapstructure:"cluster-type"`
}

// Validate the OperatorConfig returning an error if the config is not
// directly usable by the operator. This may occur if certain required
// values are unset or invalid.
func (oc *OperatorConfig) Validate() error {
	// Ensure that WorkingNamespace is set. We don't default it to anything.
	// It must be passed in, typically by the operator's own pod spec.
	if oc.WorkingNamespace == "" {
		return fmt.Errorf(
			"WorkingNamespace value [%s] invalid", oc.WorkingNamespace)
	}
	if oc.SmbServicePort <= 0 {
		return fmt.Errorf(
			"SmbPort value [%d] invalid", oc.SmbServicePort)
	}
	if oc.SmbdPort <= 0 {
		return fmt.Errorf(
			"SmbPort value [%d] invalid", oc.SmbdPort)
	}
	return nil
}

// Source is how external configuration sources populate the operator config.
type Source struct {
	v    *viper.Viper
	fset *pflag.FlagSet
}

// NewSource creates a new Source based on default configuration values.
func NewSource() *Source {
	d := DefaultOperatorConfig
	v := viper.New()
	v.SetDefault("smbd-container-image", d.SmbdContainerImage)
	v.SetDefault("smbd-metrics-container-image", d.SmbdMetricsContainerImage)
	v.SetDefault("smbd-container-name", d.SmbdContainerName)
	v.SetDefault("winbind-container-name", d.WinbindContainerName)
	v.SetDefault("working-namespace", d.WorkingNamespace)
	v.SetDefault("svc-watch-container-image", d.SvcWatchContainerImage)
	v.SetDefault("samba-debug-level", d.SambaDebugLevel)
	v.SetDefault("state-pvc-size", d.StatePVCSize)
	v.SetDefault("cluster-support", d.ClusterSupport)
	v.SetDefault("smb-service-port", d.SmbServicePort)
	v.SetDefault("smbd-port", d.SmbdPort)
	v.SetDefault("service-account-name", d.ServiceAccountName)
	v.SetDefault("metrics-exporter-mode", d.MetricsExporterMode)
	v.SetDefault("pod-name", d.PodName)
	v.SetDefault("pod-namespace", d.PodNamespace)
	v.SetDefault("pod-ip", d.PodIP)
	v.SetDefault("image-pull-policy", d.ImagePullPolicy)
	v.SetDefault("default-node-selector", d.DefaultNodeSelector)
	v.SetDefault("cluster-type", d.ClusterType)
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
		err = v.BindPFlags(s.fset)
		if err != nil {
			return nil, err
		}
	}

	// we isolate config handling to this package. thus we marshal
	// our config to the public OperatorConfig type and return that.
	c := &OperatorConfig{}
	if err := v.Unmarshal(c); err != nil {
		return nil, err
	}
	return c, nil
}

module github.com/samba-in-kubernetes/samba-operator

go 1.16

require (
	github.com/go-logr/logr v0.4.0
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.46.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.7.0
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2
	sigs.k8s.io/controller-runtime v0.10.1
)

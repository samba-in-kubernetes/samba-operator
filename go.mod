module github.com/samba-in-kubernetes/samba-operator

go 1.13

require (
	github.com/go-logr/logr v0.1.0
	github.com/obnoxxx/samba-operator v0.0.0-20210119105432-74d59e4ea3bd
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.3.2
	k8s.io/api v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v0.18.6
	sigs.k8s.io/controller-runtime v0.6.2
)

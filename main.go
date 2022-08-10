/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Command to start the samba-operator.
package main

import (
	"os"
	goruntime "runtime"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	flag "github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
	"github.com/samba-in-kubernetes/samba-operator/controllers"
	"github.com/samba-in-kubernetes/samba-operator/internal/conf"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")

	// Version of the software at compile time.
	Version = "(unset)"
	// CommitID of the revision used to compile the software.
	CommitID = "(unset)"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(rbacv1.AddToScheme(scheme))
	utilruntime.Must(monitoringv1.AddToScheme(scheme))

	utilruntime.Must(sambaoperatorv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	confSource := conf.NewSource()
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(
		&metricsAddr,
		"metrics-addr",
		":8080",
		"The address the metric endpoint binds to.")
	flag.BoolVar(
		&enableLeaderElection,
		"enable-leader-election",
		false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active "+
			"controller manager.")
	flag.CommandLine.AddFlagSet(confSource.Flags())
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	setupLog.Info("Initializing Manager",
		"ProgramName", os.Args[0],
		"Version", Version,
		"CommitID", CommitID,
		"GoVersion", goruntime.Version(),
	)

	if err := conf.Load(confSource); err != nil {
		setupLog.Error(err, "unable to configure")
		os.Exit(1)
	}

	if err := conf.Get().Validate(); err != nil {
		setupLog.Error(err, "invalid configuration", "config", conf.Get())
		os.Exit(1)
	}
	setupLog.Info("loaded configuration successfully", "config", conf.Get())

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "b60bd080.samba.org",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.SmbShareReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("SmbShare"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(
			err,
			"unable to create controller",
			"controller", "SmbShare")
		os.Exit(1)
	}
	if err = (&controllers.SmbSecurityConfigReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("SmbSecurityConfig"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(
			err,
			"unable to create controller",
			"controller", "SmbSecurityConfig")
		os.Exit(1)
	}
	if err = (&controllers.SmbCommonConfigReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("SmbCommonConfig"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(
			err,
			"unable to create controller",
			"controller", "SmbCommonConfig")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

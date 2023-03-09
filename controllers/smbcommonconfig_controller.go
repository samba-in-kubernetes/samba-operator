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

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
	"github.com/samba-in-kubernetes/samba-operator/internal/conf"
	"github.com/samba-in-kubernetes/samba-operator/internal/resources"
)

// SmbCommonConfigReconciler reconciles a SmbCommonConfig object
type SmbCommonConfigReconciler struct {
	client.Client
	Log         logr.Logger
	ClusterType string
}

//revive:disable kubebuilder directives

// nolint:lll
// +kubebuilder:rbac:groups=samba-operator.samba.org,resources=smbcommonconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=samba-operator.samba.org,resources=smbcommonconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods;endpoints;services;namespaces,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,verbs=get;list;use
// +kubebuilder:rbac:groups=security.openshift.io,resourceNames=samba,resources=securitycontextconstraints,verbs=get;list;create;update
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;create;update
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors;prometheusrules,verbs=get;list;watch;create;update

//revive:enable

// Reconcile SmbCommonConfig resources.
func (r *SmbCommonConfigReconciler) Reconcile(
	ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// ---
	log := r.Log.WithValues("smbcommonconfig", req.NamespacedName)

	// Process OpenShift logic in one of two states:
	// 1) Unknown cluster type due to first-time reconcile
	// 2) Known to be running over OpenShift by in-memory cached state from
	//    previous reconcile loop.
	if r.ClusterType != "" && r.ClusterType != conf.ClusterTypeOpenShift {
		return ctrl.Result{}, nil
	}

	mgr := resources.NewOpenShiftManager(r.Client, log, conf.Get())
	res := mgr.Process(ctx, req.NamespacedName)
	err := res.Err()
	if res.Requeue() {
		return ctrl.Result{Requeue: true}, err
	}

	// Cache in-memory cluster-type to avoid extra network round-trips in next
	// reconcile phase.
	if r.ClusterType == "" {
		r.Log.Info("Saving discovered cluster type",
			"ClusterType", mgr.ClusterType)
		r.ClusterType = mgr.ClusterType
	}

	return ctrl.Result{}, err
}

// SetupWithManager sets up resource management.
func (r *SmbCommonConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sambaoperatorv1alpha1.SmbCommonConfig{}).
		Complete(r)
}

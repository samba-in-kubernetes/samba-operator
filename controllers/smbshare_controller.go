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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
	"github.com/samba-in-kubernetes/samba-operator/internal/resources"
)

// SmbShareReconciler reconciles a SmbShare object
type SmbShareReconciler struct {
	client.Client
	Log      logr.Logger
	recorder record.EventRecorder
}

//revive:disable kubebuilder directives

// nolint:lll
// +kubebuilder:rbac:groups=samba-operator.samba.org,resources=smbshares,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=samba-operator.samba.org,resources=smbshares/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=samba-operator.samba.org,resources=smbshares/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=create
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;delete

//revive:enable

// Reconcile SmbShare resources.
func (r *SmbShareReconciler) Reconcile(
	ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// ---
	reqLogger := r.Log.WithValues("smbshare", req.NamespacedName)
	reqLogger.Info("Reconciling SmbShare")

	smbShareManager := resources.NewSmbShareManager(
		r, r.Scheme(), r.recorder, reqLogger)
	res := smbShareManager.Process(ctx, req.NamespacedName)
	err := res.Err()
	if res.Requeue() {
		return ctrl.Result{Requeue: true}, err
	}
	return ctrl.Result{}, err
}

func (r *SmbShareReconciler) setRecorder(mgr ctrl.Manager) {
	if r.recorder == nil {
		r.recorder = mgr.GetEventRecorderFor("smbshare-controller")
	}
}

// SetupWithManager sets up resource management.
func (r *SmbShareReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.setRecorder(mgr)
	return ctrl.NewControllerManagedBy(mgr).
		For(&sambaoperatorv1alpha1.SmbShare{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}

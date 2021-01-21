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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
	"github.com/samba-in-kubernetes/samba-operator/internal/resources"
)

// SmbPvcReconciler reconciles a SmbPvc object
type SmbPvcReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=samba-operator.samba.org,resources=smbpvcs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=samba-operator.samba.org,resources=smbpvcs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=samba-operator.samba.org,resources=smbservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete

// Reconcile reads that state of the cluster for a SmbPvc object and makes changes based on the state read
func (r *SmbPvcReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	reqLogger := r.Log.WithValues("smbpvc", req.NamespacedName)
	reqLogger.Info("Reconciling SmbPvc")

	smbPvcManager := resources.NewSmbPvcManager(r, r.Scheme, reqLogger)
	res := smbPvcManager.Update(ctx, req.NamespacedName)
	err := res.Err()
	if res.Requeue() {
		return ctrl.Result{Requeue: true}, err
	}
	return ctrl.Result{}, err
}

// SetupWithManager sets up resource management.
func (r *SmbPvcReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sambaoperatorv1alpha1.SmbPvc{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&sambaoperatorv1alpha1.SmbService{}).
		Complete(r)
}

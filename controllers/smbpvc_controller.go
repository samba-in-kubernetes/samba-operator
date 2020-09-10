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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sambaoperatorv1alpha1 "github.com/obnoxxx/samba-operator/api/v1alpha1"
)

// SmbPvcReconciler reconciles a SmbPvc object
type SmbPvcReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=samba-operator.samba.org,resources=smbpvcs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=samba-operator.samba.org,resources=smbpvcs/status,verbs=get;update;patch

// Reconcile reads that state of the cluster for a SmbPvc object and makes changes based on the state read
func (r *SmbPvcReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	reqLogger := r.Log.WithValues("smbpvc", req.NamespacedName)
	reqLogger.Info("Reconciling SmbPvc")

	// Fetch the SmbPvc instance
	instance := &sambaoperatorv1alpha1.SmbPvc{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	pvcname := instance.Name + "-pvc"
	svcname := instance.Name + "-svc"

	// create PVC as desired

	// Check if the pvc already exists, if not create a new one
	foundPvc := &corev1.PersistentVolumeClaim{}
	err = r.Get(ctx, types.NamespacedName{Name: pvcname, Namespace: instance.Namespace}, foundPvc)
	if err != nil && errors.IsNotFound(err) {
		// not found - define a new pvc
		pvc := r.pvcForSmbPvc(instance, pvcname)
		reqLogger.Info("Creating a new Pvc", "Pvc.Name", pvc.Name)
		err = r.Create(ctx, pvc)
		if err != nil {
			reqLogger.Error(err, "Failed to create new PVC", "pvc.Namespace", pvc.Namespace, "pvc.Name", pvc.Name)
			return ctrl.Result{}, err
		}
		// Pvc created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get PVC")
		return ctrl.Result{}, err
	}

	// create an smbservice on top of the PVC

	foundSvc := &sambaoperatorv1alpha1.SmbService{}
	err = r.Get(ctx, types.NamespacedName{Name: svcname, Namespace: instance.Namespace}, foundSvc)
	if err != nil && errors.IsNotFound(err) {
		svc := r.svcForSmbPvc(instance, svcname, pvcname)
		reqLogger.Info("Creating a new SmbService", "pvc.Name", pvcname)
		err = r.Create(ctx, svc)
		if err != nil {
			reqLogger.Error(err, "Failed to create new SmbService", "svc.Namespace", svc.Namespace, "svc.Name", svc.Name)
			return ctrl.Result{}, err
		}
		// Svc created successfullt - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get PVC")
		return ctrl.Result{}, err
	}

	// all is in shape - don't requeue
	return ctrl.Result{}, nil
}

func (r *SmbPvcReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sambaoperatorv1alpha1.SmbPvc{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&sambaoperatorv1alpha1.SmbService{}).
		Complete(r)
}

func (r *SmbPvcReconciler) pvcForSmbPvc(s *sambaoperatorv1alpha1.SmbPvc, pvcname string) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcname,
			Namespace: s.Namespace,
			//Labels:       pvcLabels,
			//Annotations:  pvcTemplate.Annotations,
		},
		Spec: *s.Spec.Pvc,
	}

	// set the smbpvc instance as the owner and controller
	ctrl.SetControllerReference(s, pvc, r.Scheme)

	return pvc
}

func (r *SmbPvcReconciler) svcForSmbPvc(s *sambaoperatorv1alpha1.SmbPvc, svcname string, pvcname string) *sambaoperatorv1alpha1.SmbService {
	svc := &sambaoperatorv1alpha1.SmbService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcname,
			Namespace: s.Namespace,
			//Labels:       pvcLabels,
			//Annotations:  pvcTemplate.Annotations,
		},
		Spec: sambaoperatorv1alpha1.SmbServiceSpec{
			PvcName: pvcname,
		},
	}

	// set the smbpvc instance as the owner and controller
	ctrl.SetControllerReference(s, svc, r.Scheme)

	return svc
}

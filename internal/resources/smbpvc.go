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

package resources

import (
	"context"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// SmbPvcManager is used to manage SmbService resources.
type SmbPvcManager struct {
	client client.Client
	scheme *runtime.Scheme
	logger Logger
}

// NewSmbPvcManager creates a SmbPvcManager.
func NewSmbPvcManager(client client.Client, scheme *runtime.Scheme, logger Logger) *SmbPvcManager {
	return &SmbPvcManager{
		client: client,
		scheme: scheme,
		logger: logger,
	}
}

// Update the managed resources on CR change.
func (m *SmbPvcManager) Update(ctx context.Context, nsname types.NamespacedName) Result {
	// Fetch the SmbPvc instance
	instance := &sambaoperatorv1alpha1.SmbPvc{}
	err := m.client.Get(ctx, nsname, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return Done
		}
		// Error reading the object - requeue the request.
		return Result{err: err}
	}

	pvcname := instance.Name + "-pvc"
	svcname := instance.Name + "-svc"

	// create PVC as desired

	// Check if the pvc already exists, if not create a new one
	foundPvc := &corev1.PersistentVolumeClaim{}
	err = m.client.Get(ctx, types.NamespacedName{Name: pvcname, Namespace: instance.Namespace}, foundPvc)
	if err != nil && errors.IsNotFound(err) {
		// not found - define a new pvc
		pvc := m.pvcForSmbPvc(instance, pvcname)
		m.logger.Info("Creating a new Pvc", "Pvc.Name", pvc.Name)
		err = m.client.Create(ctx, pvc)
		if err != nil {
			m.logger.Error(err, "Failed to create new PVC", "pvc.Namespace", pvc.Namespace, "pvc.Name", pvc.Name)
			return Result{err: err}
		}
		// Pvc created successfully - return and requeue
		return Requeue
	} else if err != nil {
		m.logger.Error(err, "Failed to get PVC")
		return Result{err: err}
	}

	// create an smbservice on top of the PVC

	foundSvc := &sambaoperatorv1alpha1.SmbService{}
	err = m.client.Get(ctx, types.NamespacedName{Name: svcname, Namespace: instance.Namespace}, foundSvc)
	if err != nil && errors.IsNotFound(err) {
		svc := m.svcForSmbPvc(instance, svcname, pvcname)
		m.logger.Info("Creating a new SmbService", "pvc.Name", pvcname)
		err = m.client.Create(ctx, svc)
		if err != nil {
			m.logger.Error(err, "Failed to create new SmbService", "svc.Namespace", svc.Namespace, "svc.Name", svc.Name)
			return Result{err: err}
		}
		// Svc created successfullt - return and requeue
		return Requeue
	} else if err != nil {
		m.logger.Error(err, "Failed to get PVC")
		return Result{err: err}
	}

	// all is in shape - don't requeue
	return Done
}

func (m *SmbPvcManager) pvcForSmbPvc(s *sambaoperatorv1alpha1.SmbPvc, pvcname string) *corev1.PersistentVolumeClaim {
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
	controllerutil.SetControllerReference(s, pvc, m.scheme)

	return pvc
}

func (m *SmbPvcManager) svcForSmbPvc(s *sambaoperatorv1alpha1.SmbPvc, svcname string, pvcname string) *sambaoperatorv1alpha1.SmbService {
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
	controllerutil.SetControllerReference(s, svc, m.scheme)

	return svc
}

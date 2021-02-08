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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
	"github.com/samba-in-kubernetes/samba-operator/internal/conf"
)

// SmbShareManager is used to manage SmbShare resources.
type SmbShareManager struct {
	client client.Client
	scheme *runtime.Scheme
	logger Logger
}

// NewSmbShareManager creates a SmbShareManager.
func NewSmbShareManager(client client.Client, scheme *runtime.Scheme, logger Logger) *SmbShareManager {
	return &SmbShareManager{
		client: client,
		scheme: scheme,
		logger: logger,
	}
}

// Update should be called when a SmbShare resource changes.
func (m *SmbShareManager) Update(ctx context.Context, nsname types.NamespacedName) Result {
	instance := &sambaoperatorv1alpha1.SmbShare{}
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
	m.logger.Info("Updating state for SmbShare",
		"name", instance.Name,
		"UID", instance.UID)

	cm, created, err := getOrCreateConfigMap(ctx, m.client, instance.Namespace)
	if err != nil {
		return Result{err: err}
	} else if created {
		m.logger.Info("Created config map")
		return Requeue
	}
	planner, changed, err := m.updateConfiguration(ctx, cm, instance)
	if err != nil {
		return Result{err: err}
	} else if changed {
		m.logger.Info("Updated config map")
		return Requeue
	}

	if shareNeedsPvc(instance) {
		pvc, created, err := m.getOrCreatePvc(
			ctx, instance, instance.Namespace)
		if err != nil {
			return Result{err: err}
		} else if created {
			m.logger.Info("Created PVC")
			return Requeue
		}
		// if name is unset in the YAML, set it here
		instance.Spec.Storage.Pvc.Name = pvc.Name
	}

	deployment, created, err := m.getOrCreateDeployment(
		ctx, planner, instance.Namespace)
	if err != nil {
		return Result{err: err}
	} else if created {
		// Deployment created successfully - return and requeue
		m.logger.Info("Created deployment")
		return Requeue
	}

	resized, err := m.updateDeploymentSize(ctx, deployment)
	if err != nil {
		return Result{err: err}
	} else if resized {
		m.logger.Info("Resized deployment")
		return Requeue
	}

	m.logger.Info("Done updating SmbShare resources")
	return Done
}

func (m *SmbShareManager) getOrCreateDeployment(ctx context.Context,
	planner *sharePlanner, ns string) (
	*appsv1.Deployment, bool, error) {
	// Check if the deployment already exists, if not create a new one
	found := &appsv1.Deployment{}
	err := m.client.Get(
		ctx,
		types.NamespacedName{
			Name:      planner.SmbShare.Name,
			Namespace: ns,
		},
		found)
	if err == nil {
		return found, false, nil
	}

	if errors.IsNotFound(err) {
		// not found - define a new deployment
		dep := m.deploymentForSmbShare(planner, ns)
		m.logger.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = m.client.Create(ctx, dep)
		if err != nil {
			m.logger.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return dep, false, err
		}
		// Deployment created successfully
		return dep, true, nil
	}
	m.logger.Error(err, "Failed to get Deployment")
	return nil, false, err
}

func (m *SmbShareManager) getOrCreatePvc(ctx context.Context,
	smbShare *sambaoperatorv1alpha1.SmbShare, ns string) (
	*corev1.PersistentVolumeClaim, bool, error) {
	// Check if the pvc already exists, if not create it
	pvc := &corev1.PersistentVolumeClaim{}
	err := m.client.Get(
		ctx,
		types.NamespacedName{
			Name:      pvcName(smbShare),
			Namespace: smbShare.Namespace,
		},
		pvc)
	if err == nil {
		return pvc, false, nil
	}

	if errors.IsNotFound(err) {
		// not found - define a new pvc
		pvc = m.pvcForSmbShare(smbShare, ns)
		m.logger.Info("Creating a new Pvc", "Pvc.Name", pvc.Name)
		err = m.client.Create(ctx, pvc)
		if err != nil {
			m.logger.Error(err, "Failed to create new PVC", "pvc.Namespace", pvc.Namespace, "pvc.Name", pvc.Name)
			return pvc, false, err
		}
		// Pvc created successfully
		return pvc, true, nil
	}
	m.logger.Error(err, "Failed to get PVC")
	return nil, false, err
}

func (m *SmbShareManager) updateDeploymentSize(ctx context.Context,
	deployment *appsv1.Deployment) (bool, error) {
	// Ensure the deployment size is the same as the spec
	var size int32 = 1
	if *deployment.Spec.Replicas != size {
		deployment.Spec.Replicas = &size
		err := m.client.Update(ctx, deployment)
		if err != nil {
			m.logger.Error(err, "Failed to update Deployment", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
			return false, err
		}
		// Spec updated
		return true, nil
	}

	return false, nil
}

// deploymentForSmbShare returns a smbshare deployment object
func (m *SmbShareManager) deploymentForSmbShare(planner *sharePlanner, ns string) *appsv1.Deployment {
	// TODO: it is not the best to be grabbing the global conf this "deep" in
	// the operator, but rather than refactor everything at once, we at least
	// stop using hard coded parameters.
	cfg := conf.Get()
	// labels - do I need them?
	dep := buildDeployment(cfg, planner, planner.SmbShare.Spec.Storage.Pvc.Name, ns)
	// set the smbshare instance as the owner and controller
	controllerutil.SetControllerReference(planner.SmbShare, dep, m.scheme)
	return dep
}

func (m *SmbShareManager) pvcForSmbShare(
	s *sambaoperatorv1alpha1.SmbShare, ns string) *corev1.PersistentVolumeClaim {
	// build a new pvc
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName(s),
			Namespace: ns,
		},
		Spec: *s.Spec.Storage.Pvc.Spec,
	}

	// set the smb share instance as the owner and controller
	controllerutil.SetControllerReference(s, pvc, m.scheme)

	return pvc
}

func pvcName(s *sambaoperatorv1alpha1.SmbShare) string {
	if s.Spec.Storage.Pvc.Name != "" {
		return s.Spec.Storage.Pvc.Name
	}
	return s.Name + "-pvc"
}

func shareNeedsPvc(s *sambaoperatorv1alpha1.SmbShare) bool {
	return s.Spec.Storage.Pvc != nil && s.Spec.Storage.Pvc.Spec != nil
}

func (m *SmbShareManager) updateConfiguration(
	ctx context.Context,
	cm *corev1.ConfigMap, s *sambaoperatorv1alpha1.SmbShare) (*sharePlanner, bool, error) {
	// extract config from map
	cc, err := getContainerConfig(cm)
	if err != nil {
		m.logger.Error(err, "unable to read samba container config")
		return nil, false, err
	}
	// extract config from map
	planner := newSharePlanner(s, cc)
	changed, err := planner.update()
	if err != nil {
		m.logger.Error(err, "unable to update samba container config")
		return nil, false, err
	}
	if !changed {
		return planner, false, nil
	}
	err = setContainerConfig(cm, planner.Config)
	if err != nil {
		m.logger.Error(err, "unable to set container config in config map")
		return nil, false, err
	}
	err = m.client.Update(ctx, cm)
	if err != nil {
		m.logger.Error(err, "failed to update config map")
		return nil, false, err
	}
	return planner, true, nil
}

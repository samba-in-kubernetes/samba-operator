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

	sambaoperatorv1alpha1 "github.com/obnoxxx/samba-operator/api/v1alpha1"
	"github.com/obnoxxx/samba-operator/internal/conf"
)

// SmbServiceManager is used to manage SmbService resources.
type SmbServiceManager struct {
	client client.Client
	scheme *runtime.Scheme
	logger Logger
}

// NewSmbServiceManager creates a SmbServiceManager.
func NewSmbServiceManager(client client.Client, scheme *runtime.Scheme, logger Logger) *SmbServiceManager {
	return &SmbServiceManager{
		client: client,
		scheme: scheme,
		logger: logger,
	}
}

// Update should be called when a SmbService resource changes.
func (m *SmbServiceManager) Update(ctx context.Context, nsname types.NamespacedName) Result {
	instance := &sambaoperatorv1alpha1.SmbService{}
	err := m.client.Get(ctx, nsname, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return Done
		}
		// Error reading the object - requeue the request.
		return Done
	}

	if (instance.Spec.Pvc.Name != "" && instance.Spec.Pvc.Spec != nil) ||
		(instance.Spec.Pvc.Name == "" && instance.Spec.Pvc.Spec == nil) {
		// TODO: I didn't see a way to make mutually exclusive options via
		// the kubebuilder annotations in the docs. This may be a situation
		// where an admission webhook is needed. That will prevent "invalid"
		// CRDs from being created. But I'm in favor of keeping double checks
		// like this in the main logic.
		return Result{err: ErrInvalidPvc}
	}
	pvcname := types.NamespacedName{
		Namespace: instance.Namespace,
		Name:      instance.Spec.Pvc.Name,
	}
	if instance.Spec.Pvc.Spec != nil {
		pvcname.Name = instance.Name + "-pvc"
		_, created, err := m.createPvcIfMissing(ctx, instance, pvcname)
		if err != nil {
			return Result{err: err}
		}
		if created {
			return Requeue
		}
	}

	depname := types.NamespacedName{
		Name:      instance.Name,
		Namespace: instance.Namespace,
	}
	dep, created, err := m.createDeploymentIfMissing(
		ctx, instance, depname, pvcname)
	if err != nil {
		return Result{err: err}
	}
	if created {
		return Requeue
	}

	// Ensure the deployment size is the same as the spec
	var size int32 = 1
	if *dep.Spec.Replicas != size {
		dep.Spec.Replicas = &size
		err = m.client.Update(ctx, dep)
		if err != nil {
			m.logger.Error(err, "Failed to update Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return Result{err: err}
		}
		// Spec updated - return and requeue
		return Requeue
	}

	return Done
}

// deploymentForSmbService returns a smbservice deployment object
func (m *SmbServiceManager) deploymentForSmbService(s *sambaoperatorv1alpha1.SmbService, nsname, pvcname types.NamespacedName) *appsv1.Deployment {
	// TODO: it is not the best to be grabbing the global conf this "deep" in
	// the operator, but rather than refactor everything at once, we at least
	// stop using hard coded parameters.
	cfg := conf.Get()
	// labels - do I need them?
	labels := labelsForSmbService(s.Name)
	volname := pvcname.Name + "-smb"
	var size int32 = 1

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsname.Name,
			Namespace: nsname.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &size,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{{
						Name: volname,
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: pvcname.Name,
							},
						},
					}},
					Containers: []corev1.Container{{
						Image: cfg.SmbdContainerImage,
						Name:  cfg.SmbdContainerName,
						//NEEDED? - Command: []string{"cmd", "arg", "arg", "..."},
						Ports: []corev1.ContainerPort{{
							ContainerPort: 139,
							Name:          "smb-netbios",
						}, {
							ContainerPort: 445,
							Name:          "smb",
						}},
						VolumeMounts: []corev1.VolumeMount{{
							MountPath: "/share",
							Name:      volname,
						}},
					}},
				},
			},
		},
	}

	// set the smbservice instance as the owner and controller
	controllerutil.SetControllerReference(s, dep, m.scheme)
	return dep
}

// labelsForSmbService returns the labels for selecting the resources
// belonging to the given smbservice CR name.
func labelsForSmbService(name string) map[string]string {
	return map[string]string{
		"app":           "smbservice",
		"smbservice_cr": name,
	}
}

func (m *SmbServiceManager) createDeploymentIfMissing(
	ctx context.Context,
	s *sambaoperatorv1alpha1.SmbService,
	nsname, pvcname types.NamespacedName) (*appsv1.Deployment, bool, error) {
	// Check if the deployment already exists, if not create a new one
	dep := &appsv1.Deployment{}
	err := m.client.Get(ctx, nsname, dep)
	if err == nil {
		return dep, false, nil
	} else if !errors.IsNotFound(err) {
		m.logger.Error(err, "Failed to get Deployment", nsname)
		return nil, false, err
	}
	// not found - define a new deployment
	dep = m.deploymentForSmbService(s, nsname, pvcname)
	m.logger.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
	err = m.client.Create(ctx, dep)
	if err != nil {
		m.logger.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		return nil, false, err
	}
	return dep, true, nil
}

func (m *SmbServiceManager) pvcFromSmbService(s *sambaoperatorv1alpha1.SmbService, nsname types.NamespacedName) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsname.Name,
			Namespace: nsname.Namespace,
			//Labels:       pvcLabels,
			//Annotations:  pvcTemplate.Annotations,
		},
		Spec: *s.Spec.Pvc.Spec,
	}
	// set the instance as the owner and controller
	controllerutil.SetControllerReference(s, pvc, m.scheme)
	return pvc
}

func (m *SmbServiceManager) createPvcIfMissing(
	ctx context.Context,
	s *sambaoperatorv1alpha1.SmbService,
	nsname types.NamespacedName) (*corev1.PersistentVolumeClaim, bool, error) {
	// Check if the pvc already exists, if not create a new one
	pvc := &corev1.PersistentVolumeClaim{}
	err := m.client.Get(ctx, nsname, pvc)
	if err == nil {
		return pvc, false, nil
	} else if !errors.IsNotFound(err) {
		m.logger.Error(err, "Failed to get PVC", nsname)
		return nil, false, err
	}
	// not found - define a new pvc
	pvc = m.pvcFromSmbService(s, nsname)
	m.logger.Info("Creating a new Pvc", "Pvc.Name", pvc.Name)
	err = m.client.Create(ctx, pvc)
	if err != nil {
		m.logger.Error(err, "Failed to create new PVC", "pvc.Namespace", pvc.Namespace, "pvc.Name", pvc.Name)
		return nil, false, err
	}
	return pvc, true, nil
}

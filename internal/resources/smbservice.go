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

	smbservicev1alpha1 "github.com/obnoxxx/samba-operator/pkg/apis/smbservice/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
	instance := &smbservicev1alpha1.SmbService{}
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

	// Check if the deployment already exists, if not create a new one
	found := &appsv1.Deployment{}
	err = m.client.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// not found - define a new deployment
		dep := m.deploymentForSmbService(instance)
		m.logger.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = m.client.Create(ctx, dep)
		if err != nil {
			m.logger.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return Result{err: err}
		}
		// Deployment created successfully - return and requeue
		return Requeue
	} else if err != nil {
		m.logger.Error(err, "Failed to get Deployment")
		return Result{err: err}
	}

	// Ensure the deployment size is the same as the spec
	var size int32 = 1
	if *found.Spec.Replicas != size {
		found.Spec.Replicas = &size
		err = m.client.Update(ctx, found)
		if err != nil {
			m.logger.Error(err, "Failed to update Deployment", "Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)
			return Result{err: err}
		}
		// Spec updated - return and requeue
		return Requeue
	}

	return Done
}

// deploymentForSmbService returns a smbservice deployment object
func (m *SmbServiceManager) deploymentForSmbService(s *smbservicev1alpha1.SmbService) *appsv1.Deployment {
	// labels - do I need them?
	labels := labelsForSmbService(s.Name)
	smb_volume := s.Spec.PvcName + "-smb"
	var size int32 = 1

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: s.Name,
			// TODO: get namespace from pvc
			Namespace: "default",
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
						Name: smb_volume,
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: s.Spec.PvcName,
							},
						},
					}},
					Containers: []corev1.Container{{
						Image: "quay.io/obnox/samba-centos8:latest",
						Name:  "samba",
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
							Name:      smb_volume,
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

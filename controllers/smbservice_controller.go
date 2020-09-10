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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sambaoperatorv1alpha1 "github.com/obnoxxx/samba-operator/api/v1alpha1"
)

// SmbServiceReconciler reconciles a SmbService object
type SmbServiceReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=samba-operator.samba.org,resources=smbservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=samba-operator.samba.org,resources=smbservices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list

func (r *SmbServiceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	reqLogger := r.Log.WithValues("smbservice", req.NamespacedName)
	reqLogger.Info("Reconciling SmbService")

	// Fetch the SmbService instance
	instance := &sambaoperatorv1alpha1.SmbService{}
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

	// Check if the deployment already exists, if not create a new one
	found := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// not found - define a new deployment
		dep := r.deploymentForSmbService(instance, instance.Namespace)
		reqLogger.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.Create(ctx, dep)
		if err != nil {
			reqLogger.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	// Ensure the deployment size is the same as the spec
	var size int32 = 1
	if *found.Spec.Replicas != size {
		found.Spec.Replicas = &size
		err = r.Update(ctx, found)
		if err != nil {
			reqLogger.Error(err, "Failed to update Deployment", "Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)
			return ctrl.Result{}, err
		}
		// Spec updated - return and requeue
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

func (r *SmbServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sambaoperatorv1alpha1.SmbService{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}

// deploymentForSmbService returns a smbservice deployment object
func (r *SmbServiceReconciler) deploymentForSmbService(s *sambaoperatorv1alpha1.SmbService, ns string) *appsv1.Deployment {
	// labels - do I need them?
	labels := labelsForSmbService(s.Name)
	smb_volume := s.Spec.PvcName + "-smb"
	var size int32 = 1

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: ns,
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
	ctrl.SetControllerReference(s, dep, r.Scheme)
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

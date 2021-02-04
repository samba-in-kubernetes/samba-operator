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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
	"github.com/samba-in-kubernetes/samba-operator/internal/conf"
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

	// Check if the deployment already exists, if not create a new one
	found := &appsv1.Deployment{}
	err = m.client.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// not found - define a new deployment
		dep := m.deploymentForSmbService(instance, instance.Namespace)
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
func (m *SmbServiceManager) deploymentForSmbService(s *sambaoperatorv1alpha1.SmbService, ns string) *appsv1.Deployment {
	// TODO: it is not the best to be grabbing the global conf this "deep" in
	// the operator, but rather than refactor everything at once, we at least
	// stop using hard coded parameters.
	cfg := conf.Get()
	// labels - do I need them?
	dep := buildDeployment(cfg, s, ns)
	// set the smbservice instance as the owner and controller
	controllerutil.SetControllerReference(s, dep, m.scheme)
	return dep
}

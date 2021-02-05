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

	deployment, created, err := m.getOrCreateDeployment(
		ctx, instance, instance.Namespace)
	if err != nil {
		return Result{err: err}
	} else if created {
		// Deployment created successfully - return and requeue
		return Requeue
	}

	resized, err := m.updateDeploymentSize(ctx, deployment)
	if err != nil {
		return Result{err: err}
	} else if resized {
		return Requeue
	}

	return Done
}

func (m *SmbShareManager) getOrCreateDeployment(ctx context.Context,
	smbShare *sambaoperatorv1alpha1.SmbShare, ns string) (
	*appsv1.Deployment, bool, error) {
	// Check if the deployment already exists, if not create a new one
	found := &appsv1.Deployment{}
	err := m.client.Get(
		ctx,
		types.NamespacedName{
			Name:      smbShare.Name,
			Namespace: ns,
		},
		found)
	if err == nil {
		return found, false, nil
	}

	if errors.IsNotFound(err) {
		// not found - define a new deployment
		dep := m.deploymentForSmbShare(smbShare, ns)
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
func (m *SmbShareManager) deploymentForSmbShare(s *sambaoperatorv1alpha1.SmbShare, ns string) *appsv1.Deployment {
	// TODO: it is not the best to be grabbing the global conf this "deep" in
	// the operator, but rather than refactor everything at once, we at least
	// stop using hard coded parameters.
	cfg := conf.Get()
	// labels - do I need them?
	dep := buildDeployment(cfg, s.Name, s.Spec.Storage.Pvc.Name, ns)
	// set the smbshare instance as the owner and controller
	controllerutil.SetControllerReference(s, dep, m.scheme)
	return dep
}

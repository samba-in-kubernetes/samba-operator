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
	"k8s.io/client-go/tools/record"
	rtclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
	"github.com/samba-in-kubernetes/samba-operator/internal/conf"
)

const shareFinalizer = "samba-operator.samba.org/shareFinalizer"

// SmbShareManager is used to manage SmbShare resources.
type SmbShareManager struct {
	client   rtclient.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
	logger   Logger
	cfg      *conf.OperatorConfig
}

// NewSmbShareManager creates a SmbShareManager.
func NewSmbShareManager(
	client rtclient.Client,
	scheme *runtime.Scheme,
	recorder record.EventRecorder,
	logger Logger) *SmbShareManager {
	// ---
	return &SmbShareManager{
		client:   client,
		scheme:   scheme,
		recorder: recorder,
		logger:   logger,
		cfg:      conf.Get(),
	}
}

// Process is called by the controller on any type of reconciliation.
func (m *SmbShareManager) Process(
	ctx context.Context,
	nsname types.NamespacedName) Result {
	// fetch our resource to determine what to do next
	instance := &sambaoperatorv1alpha1.SmbShare{}
	err := m.client.Get(ctx, nsname, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found. Not a fatal error.
			return Done
		}
		m.logger.Error(
			err,
			"Failed to get SmbShare",
			"SmbShare.Namespace", nsname.Namespace,
			"SmbShare.Name", nsname.Name)
		return Result{err: err}
	}

	// now that we have the resource. determine if its live or pending deletion
	if instance.GetDeletionTimestamp() != nil {
		// its being deleted
		if controllerutil.ContainsFinalizer(instance, shareFinalizer) {
			// and our finalizer is present
			return m.Finalize(ctx, instance)
		}
		return Done
	}
	// resource is alive
	return m.Update(ctx, instance)
}

// Update should be called when a SmbShare resource changes.
func (m *SmbShareManager) Update(
	ctx context.Context,
	instance *sambaoperatorv1alpha1.SmbShare) Result {
	// ---
	m.logger.Info(
		"Updating state for SmbShare",
		"SmbShare.Namespace", instance.Namespace,
		"SmbShare.Name", instance.Name,
		"SmbShare.UID", instance.UID)

	changed, err := m.addFinalizer(ctx, instance)
	if err != nil {
		return Result{err: err}
	} else if changed {
		m.logger.Info("Added finalizer")
		return Requeue
	}

	// assign the share to a Server Group. Currently we only support 1:1
	// shares to servers & it simply reflects the name of the resource.
	changed, err = m.setServerGroup(ctx, instance)
	if err != nil {
		return Result{err: err}
	} else if changed {
		m.logger.Info("Updated server group")
		return Requeue
	}

	destNamespace := instance.Namespace
	cm, created, err := m.getOrCreateConfigMap(ctx, instance, destNamespace)
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
			ctx, instance, destNamespace)
		if err != nil {
			return Result{err: err}
		} else if created {
			m.logger.Info("Created PVC")
			m.recorder.Eventf(instance,
				EventNormal,
				ReasonCreatedPersistentVolumeClaim,
				"Created PVC %s for SmbShare", pvc.Name)
			return Requeue
		}
		// if name is unset in the YAML, set it here
		instance.Spec.Storage.Pvc.Name = pvc.Name
	}

	deployment, created, err := m.getOrCreateDeployment(
		ctx, planner, destNamespace)
	if err != nil {
		return Result{err: err}
	} else if created {
		// Deployment created successfully - return and requeue
		m.logger.Info("Created deployment")
		m.recorder.Eventf(instance,
			EventNormal,
			ReasonCreatedDeployment,
			"Created deployment %s for SmbShare", deployment.Name)
		return Requeue
	}

	resized, err := m.updateDeploymentSize(ctx, deployment)
	if err != nil {
		return Result{err: err}
	} else if resized {
		m.logger.Info("Resized deployment")
		return Requeue
	}

	_, created, err = m.getOrCreateService(
		ctx, planner, destNamespace)
	if err != nil {
		return Result{err: err}
	} else if created {
		m.logger.Info("Created service")
		return Requeue
	}

	m.logger.Info("Done updating SmbShare resources")
	return Done
}

// Finalize should be called when there's a finalizer on the resource
// and we need to do some cleanup.
func (m *SmbShareManager) Finalize(
	ctx context.Context,
	instance *sambaoperatorv1alpha1.SmbShare) Result {
	// ---
	destNamespace := instance.Namespace
	cm, err := m.getConfigMap(ctx, instance, destNamespace)
	if err == nil {
		// previously, we kept one configmap for many SmbShares but have moved
		// away from that however, just to be safe, we're retaining the finalizer
		// and check that the config is OK to remove in the case that we need to
		// share the config map across other/multiple resources in the future.
		_, changed, err := m.updateConfiguration(ctx, cm, instance)
		if err != nil {
			return Result{err: err}
		} else if changed {
			m.logger.Info("Updated config map during Finalize")
			return Requeue
		}
	} else if err != nil && !errors.IsNotFound(err) {
		return Result{err: err}
	}

	m.logger.Info("Removing finalizer")
	controllerutil.RemoveFinalizer(instance, shareFinalizer)
	err = m.client.Update(ctx, instance)
	if err != nil {
		return Result{err: err}
	}
	return Done
}

func (m *SmbShareManager) getOrCreateDeployment(
	ctx context.Context,
	planner *sharePlanner,
	ns string) (*appsv1.Deployment, bool, error) {
	// Check if the deployment already exists, if not create a new one
	found := &appsv1.Deployment{}
	err := m.client.Get(
		ctx,
		types.NamespacedName{
			Name:      planner.instanceName(),
			Namespace: ns,
		},
		found)
	if err == nil {
		return found, false, nil
	}

	if errors.IsNotFound(err) {
		// not found - define a new deployment
		// labels - do I need them?
		dep := buildDeployment(
			m.cfg, planner, planner.SmbShare.Spec.Storage.Pvc.Name, ns)
		// set the smbshare instance as the owner and controller
		err = controllerutil.SetControllerReference(
			planner.SmbShare, dep, m.scheme)
		if err != nil {
			m.logger.Error(
				err,
				"Failed to set controller reference",
				"SmbShare.Namespace", planner.SmbShare.Namespace,
				"SmbShare.Name", planner.SmbShare.Name,
				"Deployment.Namespace", dep.Namespace,
				"Deployment.Name", dep.Name)
			return dep, false, err
		}
		m.logger.Info(
			"Creating a new Deployment",
			"SmbShare.Namespace", planner.SmbShare.Namespace,
			"SmbShare.Name", planner.SmbShare.Name,
			"Deployment.Namespace", dep.Namespace,
			"Deployment.Name", dep.Name)
		err = m.client.Create(ctx, dep)
		if err != nil {
			m.logger.Error(
				err,
				"Failed to create new Deployment",
				"SmbShare.Namespace", planner.SmbShare.Namespace,
				"SmbShare.Name", planner.SmbShare.Name,
				"Deployment.Namespace", dep.Namespace,
				"Deployment.Name", dep.Name)
			return dep, false, err
		}
		// Deployment created successfully
		return dep, true, nil
	}
	m.logger.Error(
		err,
		"Failed to get Deployment",
		"SmbShare.Namespace", planner.SmbShare.Namespace,
		"SmbShare.Name", planner.SmbShare.Name,
		"Deployment.Namespace", dep.Namespace,
		"Deployment.Name", dep.Name)
	return nil, false, err
}

func (m *SmbShareManager) getOrCreatePvc(
	ctx context.Context,
	smbShare *sambaoperatorv1alpha1.SmbShare,
	ns string) (*corev1.PersistentVolumeClaim, bool, error) {
	// Check if the pvc already exists, if not create it
	pvc := &corev1.PersistentVolumeClaim{}
	err := m.client.Get(
		ctx,
		types.NamespacedName{
			Name:      pvcName(smbShare),
			Namespace: ns,
		},
		pvc)
	if err == nil {
		return pvc, false, nil
	}

	if errors.IsNotFound(err) {
		// not found - define a new pvc
		pvc = &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName(smbShare),
				Namespace: ns,
			},
			Spec: *smbShare.Spec.Storage.Pvc.Spec,
		}
		// set the smb share instance as the owner and controller
		err = controllerutil.SetControllerReference(
			smbShare, pvc, m.scheme)
		if err != nil {
			m.logger.Error(
				err,
				"Failed to set controller reference",
				"SmbShare.Namespace", smbShare.Namespace,
				"SmbShare.Name", smbShare.Name,
				"PersistentVolumeClaim.Namespace", pvc.Namespace,
				"PersistentVolumeClaim.Name", pvc.Name)
			return pvc, false, err
		}
		m.logger.Info(
			"Creating a new PVC",
			"SmbShare.Namespace", smbShare.Namespace,
			"SmbShare.Name", smbShare.Name,
			"PersistentVolumeClaim.Namespace", pvc.Namespace,
			"PersistentVolumeClaim.Name", pvc.Name)
		err = m.client.Create(ctx, pvc)
		if err != nil {
			m.logger.Error(
				err,
				"Failed to create new PVC",
				"SmbShare.Namespace", smbShare.Namespace,
				"SmbShare.Name", smbShare.Name,
				"PersistentVolumeClaim.Namespace", pvc.Namespace,
				"PersistentVolumeClaim.Name", pvc.Name)
			return pvc, false, err
		}
		// Pvc created successfully
		return pvc, true, nil
	}
	m.logger.Error(
		err,
		"Failed to get PVC",
		"SmbShare.Namespace", smbShare.Namespace,
		"SmbShare.Name", smbShare.Name,
		"PersistentVolumeClaim.Namespace", pvc.Namespace,
		"PersistentVolumeClaim.Name", pvc.Name)
	return nil, false, err
}

func (m *SmbShareManager) getOrCreateService(
	ctx context.Context, planner *sharePlanner, ns string) (
	*corev1.Service, bool, error) {
	// Check if the service already exists, if not create a new one
	found := &corev1.Service{}
	err := m.client.Get(
		ctx,
		types.NamespacedName{
			Name:      planner.instanceName(),
			Namespace: ns,
		},
		found)
	if err == nil {
		return found, false, nil
	}

	if errors.IsNotFound(err) {
		// not found - define a new deployment
		svc := newServiceForSmb(planner, ns)
		// set the smbshare instance as the owner and controller
		err = controllerutil.SetControllerReference(
			planner.SmbShare, svc, m.scheme)
		if err != nil {
			m.logger.Error(
				err,
				"Failed to set controller reference",
				"SmbShare.Namespace", planner.SmbShare.Namespace,
				"SmbShare.Name", planner.SmbShare.Name,
				"Service.Namespace", svc.Namespace,
				"Service.Name", svc.Name)
			return svc, false, err
		}
		m.logger.Info("Creating a new Service",
			"SmbShare.Namespace", planner.SmbShare.Namespace,
			"SmbShare.Name", planner.SmbShare.Name,
			"Service.Namespace", svc.Namespace,
			"Service.Name", svc.Name)
		err = m.client.Create(ctx, svc)
		if err != nil {
			m.logger.Error(
				err,
				"Failed to create new Service",
				"SmbShare.Namespace", planner.SmbShare.Namespace,
				"SmbShare.Name", planner.SmbShare.Name,
				"Service.Namespace", svc.Namespace,
				"Service.Name", svc.Name)
			return svc, false, err
		}
		// Deployment created successfully
		return svc, true, nil
	}
	m.logger.Error(
		err,
		"Failed to get Service",
		"SmbShare.Namespace", planner.SmbShare.Namespace,
		"SmbShare.Name", planner.SmbShare.Name,
		"Service.Namespace", svc.Namespace,
		"Service.Name", svc.Name)
	return nil, false, err
}

func (m *SmbShareManager) getOrCreateConfigMap(
	ctx context.Context,
	smbShare *sambaoperatorv1alpha1.SmbShare,
	ns string) (
	*corev1.ConfigMap, bool, error) {
	// create a temporary planner based solely on the SmbShare
	// this is used for the name generating function & consistency
	planner := newSharePlanner(
		InstanceConfiguration{
			SmbShare:     smbShare,
			GlobalConfig: m.cfg,
		},
		nil)
	// fetch the existing config, if available
	found := &corev1.ConfigMap{}
	cmKey := types.NamespacedName{
		Name:      planner.instanceName(),
		Namespace: ns,
	}
	err := m.client.Get(ctx, cmKey, found)
	if err == nil {
		return found, false, nil
	}

	if !errors.IsNotFound(err) {
		// unexpected errror!
		m.logger.Error(
			err,
			"Failed to get ConfigMap",
			"SmbShare.Namespace", planner.SmbShare.Namespace,
			"SmbShare.Name", planner.SmbShare.Name,
			"ConfigMap.Namespace", cmKey.Namespace,
			"ConfigMap.Name", cmKey.Name)
		return nil, false, err
	}

	cm, err := newDefaultConfigMap(cmKey.Name, cmKey.Namespace)
	if err != nil {
		m.logger.Error(
			err,
			"Failed to generate default ConfigMap",
			"SmbShare.Namespace", planner.SmbShare.Namespace,
			"SmbShare.Name", planner.SmbShare.Name,
			"ConfigMap.Namespace", cm.Namespace,
			"ConfigMap.Name", cm.Name)
		return cm, false, err
	}
	// set the smbshare instance as the owner and controller
	err = controllerutil.SetControllerReference(
		planner.SmbShare, cm, m.scheme)
	if err != nil {
		m.logger.Error(
			err,
			"Failed to set controller reference",
			"SmbShare.Namespace", planner.SmbShare.Namespace,
			"SmbShare.Name", planner.SmbShare.Name,
			"ConfigMap.Namespace", cm.Namespace,
			"ConfigMap.Name", cm.Name)
		return cm, false, err
	}
	err = m.client.Create(ctx, cm)
	if err != nil {
		m.logger.Error(
			err,
			"Failed to create new ConfigMap",
			"SmbShare.Namespace", planner.SmbShare.Namespace,
			"SmbShare.Name", planner.SmbShare.Name,
			"ConfigMap.Namespace", cm.Namespace,
			"ConfigMap.Name", cm.Name)
		return cm, false, err
	}
	// Deployment created successfully
	return cm, true, nil
}

func (m *SmbShareManager) updateDeploymentSize(
	ctx context.Context,
	deployment *appsv1.Deployment) (bool, error) {
	// Ensure the deployment size is the same as the spec
	var size int32 = 1
	if *deployment.Spec.Replicas != size {
		deployment.Spec.Replicas = &size
		err := m.client.Update(ctx, deployment)
		if err != nil {
			m.logger.Error(
				err,
				"Failed to update Deployment",
				"Deployment.Namespace", deployment.Namespace,
				"Deployment.Name", deployment.Name)
			return false, err
		}
		// Spec updated
		return true, nil
	}

	return false, nil
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
	cm *corev1.ConfigMap,
	s *sambaoperatorv1alpha1.SmbShare) (*sharePlanner, bool, error) {
	// extract config from map
	cc, err := getContainerConfig(cm)
	if err != nil {
		m.logger.Error(err, "unable to read samba container config")
		return nil, false, err
	}
	isDeleting := s.GetDeletionTimestamp() != nil
	if isDeleting {
		m.logger.Info(
			"ConfigMap is being deleted - returning minimal planner")
		planner := newSharePlanner(
			InstanceConfiguration{
				SmbShare:     s,
				GlobalConfig: m.cfg,
			},
			nil)
		return planner, false, nil
	}
	security, err := m.getSecurityConfig(ctx, s)
	if err != nil {
		m.logger.Error(err, "failed to get SmbSecurityConfig")
		return nil, false, err
	}
	common, err := m.getCommonConfig(ctx, s)
	if err != nil {
		m.logger.Error(err, "failed to get SmbCommonConfig")
		return nil, false, err
	}

	// extract config from map
	var changed bool
	planner := newSharePlanner(
		InstanceConfiguration{
			SmbShare:       s,
			SecurityConfig: security,
			CommonConfig:   common,
			GlobalConfig:   m.cfg,
		},
		cc)
	changed, err = planner.update()
	if err != nil {
		m.logger.Error(err, "unable to update samba container config")
		return nil, false, err
	}
	if !changed {
		// nothing changed between the planner and the config stored in the cm
		// we can just return now as no changes need to be applied to the cm
		return planner, false, nil
	}
	err = setContainerConfig(cm, planner.ConfigState)
	if err != nil {
		m.logger.Error(
			err,
			"unable to set container config in ConfigMap",
			"ConfigMap.Namespace", cm.Namespace,
			"ConfigMap.Name", cm.Name)
		return nil, false, err
	}
	err = m.client.Update(ctx, cm)
	if err != nil {
		m.logger.Error(
			err,
			"failed to update ConfigMap",
			"ConfigMap.Namespace", cm.Namespace,
			"ConfigMap.Name", cm.Name)
		return nil, false, err
	}
	return planner, true, nil
}

func (m *SmbShareManager) addFinalizer(
	ctx context.Context, s *sambaoperatorv1alpha1.SmbShare) (bool, error) {
	// ---
	if controllerutil.ContainsFinalizer(s, shareFinalizer) {
		return false, nil
	}
	controllerutil.AddFinalizer(s, shareFinalizer)
	return true, m.client.Update(ctx, s)
}

func (m *SmbShareManager) getSecurityConfig(
	ctx context.Context, s *sambaoperatorv1alpha1.SmbShare) (
	*sambaoperatorv1alpha1.SmbSecurityConfig, error) {
	// check if the share specifies a security config
	if s.Spec.SecurityConfig == "" {
		return nil, nil
	}

	nsname := types.NamespacedName{
		Name:      s.Spec.SecurityConfig,
		Namespace: s.Namespace,
	}
	security := &sambaoperatorv1alpha1.SmbSecurityConfig{}
	err := m.client.Get(ctx, nsname, security)
	if err != nil {
		return nil, err
	}
	return security, nil
}

func (m *SmbShareManager) getCommonConfig(
	ctx context.Context, s *sambaoperatorv1alpha1.SmbShare) (
	*sambaoperatorv1alpha1.SmbCommonConfig, error) {
	// check if the share specifies a common config
	if s.Spec.CommonConfig == "" {
		return nil, nil
	}

	nsname := types.NamespacedName{
		Name:      s.Spec.CommonConfig,
		Namespace: s.Namespace,
	}
	cconfig := &sambaoperatorv1alpha1.SmbCommonConfig{}
	err := m.client.Get(ctx, nsname, cconfig)
	if err != nil {
		return nil, err
	}
	return cconfig, nil
}

func (m *SmbShareManager) getConfigMap(
	ctx context.Context,
	smbShare *sambaoperatorv1alpha1.SmbShare,
	ns string) (
	*corev1.ConfigMap, error) {
	// create a temporary planner based solely on the SmbShare
	// this is used for the name generating function & consistency
	planner := newSharePlanner(
		InstanceConfiguration{
			SmbShare:     smbShare,
			GlobalConfig: m.cfg,
		},
		nil)
	// fetch the existing config, if available
	found := &corev1.ConfigMap{}
	cmKey := types.NamespacedName{
		Name:      planner.instanceName(),
		Namespace: ns,
	}
	err := m.client.Get(ctx, cmKey, found)
	return found, err
}

func (m *SmbShareManager) setServerGroup(
	ctx context.Context, s *sambaoperatorv1alpha1.SmbShare) (bool, error) {
	// check to see if there's already a group for this
	if s.Status.ServerGroup != "" {
		// already assigned, nothing extra to do
		return false, nil
	}

	// NOTE: currently the ServerGroup is only assigned the exact name of the
	// resource. In the future this may change if/when multiple SmbShares can
	// be hosted by one smbd pod.
	s.Status.ServerGroup = s.ObjectMeta.Name
	return true, m.client.Status().Update(ctx, s)
}

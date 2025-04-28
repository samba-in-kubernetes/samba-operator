// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	kresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types" // nolint:typecheck
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
	pln "github.com/samba-in-kubernetes/samba-operator/internal/planner"
)

func (m *SmbShareManager) getExistingDeployment(
	ctx context.Context,
	planner *pln.Planner,
	ns string) (*appsv1.Deployment, error) {
	// ---
	depKey := types.NamespacedName{
		Name:      planner.InstanceName(),
		Namespace: ns,
	}
	found := &appsv1.Deployment{}
	err := m.client.Get(ctx, depKey, found)
	if err == nil {
		return found, nil
	}

	if !errors.IsNotFound(err) {
		// unexpected error!
		m.logger.Error(
			err,
			"Failed to get Deployment",
			"SmbShare.Namespace", planner.SmbShare.Namespace,
			"SmbShare.Name", planner.SmbShare.Name,
			"Deployment.Namespace", depKey.Namespace,
			"Deployment.Name", depKey.Name)
		return nil, err
	}
	// was not found
	return nil, nil
}

func (m *SmbShareManager) getOrCreateDeployment(
	ctx context.Context,
	planner *pln.Planner,
	ns string) (*appsv1.Deployment, bool, error) {
	// Check if the deployment already exists, if not create a new one
	found, err := m.getExistingDeployment(ctx, planner, ns)
	if err != nil {
		return nil, false, err
	}
	if found != nil {
		return found, false, nil
	}

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

func (m *SmbShareManager) getOrCreateStatePVC(
	ctx context.Context,
	planner *pln.Planner,
	ns string) (*corev1.PersistentVolumeClaim, bool, error) {
	// ---
	name := sharedStatePVCName(planner)
	squant, err := kresource.ParseQuantity(
		planner.GlobalConfig.StatePVCSize)
	if err != nil {
		return nil, false, err
	}
	spec := &corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{
			corev1.ReadWriteMany,
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: squant,
			},
		},
	}

	if planner.CommonConfig.Spec.StatePVSCName != "" {
		spec.StorageClassName = &planner.CommonConfig.Spec.StatePVSCName
	}

	pvc, cr, err := m.getOrCreateGenericPVC(
		ctx, planner.SmbShare, spec, name, ns)
	if err != nil {
		m.logger.Error(err, "Error establishing shared state PVC")
	}
	return pvc, cr, err
}

func (m *SmbShareManager) getOrCreatePvc(
	ctx context.Context,
	smbShare *sambaoperatorv1alpha1.SmbShare,
	ns string) (*corev1.PersistentVolumeClaim, bool, error) {
	// ---
	name := pvcName(smbShare)
	spec := smbShare.Spec.Storage.Pvc.Spec
	pvc, cr, err := m.getOrCreateGenericPVC(
		ctx, smbShare, spec, name, ns)
	if err != nil {
		m.logger.Error(err, "Error establishing data PVC")
	}
	return pvc, cr, err
}

func (m *SmbShareManager) getExistingPVC(
	ctx context.Context,
	name, ns string) (*corev1.PersistentVolumeClaim, error) {
	// ---
	pvc := &corev1.PersistentVolumeClaim{}
	pvcKey := types.NamespacedName{
		Name:      name,
		Namespace: ns,
	}
	err := m.client.Get(ctx, pvcKey, pvc)
	if err == nil {
		return pvc, nil
	}

	if !errors.IsNotFound(err) {
		// unexpected error!
		m.logger.Error(
			err,
			"Failed to get PVC",
			"PersistentVolumeClaim.Namespace", pvcKey.Namespace,
			"PersistentVolumeClaim.Name", pvcKey.Name)
		return nil, err
	}
	return nil, nil
}

func (m *SmbShareManager) getOrCreateGenericPVC(
	ctx context.Context,
	smbShare *sambaoperatorv1alpha1.SmbShare,
	spec *corev1.PersistentVolumeClaimSpec,
	name, ns string) (*corev1.PersistentVolumeClaim, bool, error) {
	// Check if the pvc already exists, if not create it
	pvc, err := m.getExistingPVC(ctx, name, ns)
	if err != nil {
		return nil, false, err
	}
	if pvc != nil {
		return pvc, false, nil
	}

	// not found - define a new pvc
	pvc = &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: *spec,
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

func (m *SmbShareManager) getOrCreateService(
	ctx context.Context, planner *pln.Planner, ns string) (
	*corev1.Service, bool, error) {
	// Check if the service already exists, if not create a new one
	found := &corev1.Service{}
	svcKey := types.NamespacedName{
		Name:      planner.InstanceName(),
		Namespace: ns,
	}
	err := m.client.Get(ctx, svcKey, found)
	if err == nil {
		return found, false, nil
	}

	if !errors.IsNotFound(err) {
		// unexpected error!
		m.logger.Error(
			err,
			"Failed to get Service",
			"SmbShare.Namespace", planner.SmbShare.Namespace,
			"SmbShare.Name", planner.SmbShare.Name,
			"Service.Namespace", svcKey.Namespace,
			"Service.Name", svcKey.Name)
		return nil, false, err
	}

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

func (m *SmbShareManager) getOrCreateConfigMap(
	ctx context.Context,
	smbShare *sambaoperatorv1alpha1.SmbShare,
	ns string) (
	*corev1.ConfigMap, bool, error) {
	// create a temporary planner based solely on the SmbShare
	// this is used for the name generating function & consistency
	planner := pln.New(
		pln.InstanceConfiguration{
			SmbShare:     smbShare,
			GlobalConfig: m.cfg,
		},
		nil)
	// fetch the existing config, if available
	found := &corev1.ConfigMap{}
	cmKey := types.NamespacedName{
		Name:      planner.InstanceName(),
		Namespace: ns,
	}
	err := m.client.Get(ctx, cmKey, found)
	if err == nil {
		return found, false, nil
	}

	if !errors.IsNotFound(err) {
		// unexpected error!
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

func (m *SmbShareManager) getExistingStatefulSet(
	ctx context.Context,
	planner *pln.Planner,
	ns string) (*appsv1.StatefulSet, error) {
	// ---
	found := &appsv1.StatefulSet{}
	ssKey := types.NamespacedName{
		Name:      planner.InstanceName(),
		Namespace: ns,
	}
	err := m.client.Get(ctx, ssKey, found)
	if err == nil {
		return found, nil
	}

	if !errors.IsNotFound(err) {
		// unexpected error
		m.logger.Error(
			err,
			"Failed to get StatefulSet",
			"SmbShare.Namespace", planner.SmbShare.Namespace,
			"SmbShare.Name", planner.SmbShare.Name,
			"SatefulSet.Namespace", ssKey.Namespace,
			"SatefulSet.Name", ssKey.Name)
		return nil, err
	}
	return nil, nil
}

func (m *SmbShareManager) getOrCreateStatefulSet(
	ctx context.Context,
	planner *pln.Planner,
	ns string) (*appsv1.StatefulSet, bool, error) {
	// Check if the ss already exists, if not create a new one
	found, err := m.getExistingStatefulSet(ctx, planner, ns)
	if err != nil {
		return nil, false, err
	}
	if found != nil {
		return found, false, nil
	}

	// not found - define a new stateful set
	ss := buildStatefulSet(
		planner,
		planner.SmbShare.Spec.Storage.Pvc.Name,
		sharedStatePVCName(planner),
		ns)
	// set the smbshare instance as the owner/controller
	err = controllerutil.SetControllerReference(
		planner.SmbShare, ss, m.scheme)
	if err != nil {
		m.logger.Error(
			err,
			"Failed to set controller reference",
			"SmbShare.Namespace", planner.SmbShare.Namespace,
			"SmbShare.Name", planner.SmbShare.Name,
			"StatefulSet.Namespace", ss.Namespace,
			"StatefulSet.Name", ss.Name)
		return ss, false, err
	}
	m.logger.Info(
		"Creating a new StatefulSet",
		"SmbShare.Namespace", planner.SmbShare.Namespace,
		"SmbShare.Name", planner.SmbShare.Name,
		"StatefulSet.Namespace", ss.Namespace,
		"StatefulSet.Name", ss.Name,
		"StatefulSet.Replicas", ss.Spec.Replicas)
	err = m.client.Create(ctx, ss)
	if err != nil {
		m.logger.Error(
			err,
			"Failed to create new StatefulSet",
			"SmbShare.Namespace", planner.SmbShare.Namespace,
			"SmbShare.Name", planner.SmbShare.Name,
			"StatefulSet.Namespace", ss.Namespace,
			"StatefulSet.Name", ss.Name)
		return ss, false, err
	}
	return ss, true, err
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

func (m *SmbShareManager) getSmbShareByName(
	ctx context.Context,
	name types.NamespacedName) (*sambaoperatorv1alpha1.SmbShare, error) {
	// ---
	smbshare := &sambaoperatorv1alpha1.SmbShare{}
	err := m.client.Get(ctx, name, smbshare)
	if err != nil {
		return nil, err
	}
	return smbshare, nil
}

func (m *SmbShareManager) getConfigMap(
	ctx context.Context,
	smbShare *sambaoperatorv1alpha1.SmbShare,
	ns string) (
	*corev1.ConfigMap, error) {
	// create a temporary planner based solely on the SmbShare
	// this is used for the name generating function & consistency
	planner := pln.New(
		pln.InstanceConfiguration{
			SmbShare:     smbShare,
			GlobalConfig: m.cfg,
		},
		nil)
	// fetch the existing config, if available
	found := &corev1.ConfigMap{}
	cmKey := types.NamespacedName{
		Name:      planner.InstanceName(),
		Namespace: ns,
	}
	err := m.client.Get(ctx, cmKey, found)
	return found, err
}

func (m *SmbShareManager) getShareInstance(
	ctx context.Context,
	s *sambaoperatorv1alpha1.SmbShare) (pln.InstanceConfiguration, error) {
	// ---
	var shareInstance pln.InstanceConfiguration
	security, err := m.getSecurityConfig(ctx, s)
	if err != nil {
		m.logger.Error(err, "failed to get SmbSecurityConfig")
		return shareInstance, err
	}
	common, err := m.getCommonConfig(ctx, s)
	if err != nil {
		m.logger.Error(err, "failed to get SmbCommonConfig")
		return shareInstance, err
	}
	shareInstance = pln.InstanceConfiguration{
		SmbShare:       s,
		SecurityConfig: security,
		CommonConfig:   common,
		GlobalConfig:   m.cfg,
	}
	return shareInstance, nil
}

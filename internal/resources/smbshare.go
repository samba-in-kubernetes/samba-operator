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
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	kresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types" // nolint:typecheck
	"k8s.io/client-go/tools/record"
	rtclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
	"github.com/samba-in-kubernetes/samba-operator/internal/conf"
	pln "github.com/samba-in-kubernetes/samba-operator/internal/planner"
)

const shareFinalizer = "samba-operator.samba.org/shareFinalizer"

const (
	serverBackend    = "samba-operator.samba.org/serverBackend"
	clusteredBackend = "clustered:ctdb/statefulset"
	standardBackend  = "standard:deployment"
)

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
// nolint:funlen
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

	// assign the share to a Server Group. The server group represents
	// the resources needed to create samba servers (possibly a cluster)
	// and prerequisite resources. The serverGroup name used to name
	// many (all?) of these resources.
	changed, err = m.setServerGroup(ctx, instance)
	if err != nil {
		return Result{err: err}
	} else if changed {
		m.logger.Info("Updated server group")
		return Requeue
	}

	var planner *pln.Planner
	if p, result := m.updateConfigMap(ctx, instance); !result.Yield() {
		// p and result only exist within the scope of the if-statement. This
		// is done to keep the result out of the function scope. But to reuse
		// the planner, we need to assign p to the func scoped var
		planner = p
	} else {
		return result
	}

	if shareNeedsPvc(instance) {
		if result := m.updatePVC(ctx, instance); result.Yield() {
			return result
		}
	}

	hasBackend := instance.Annotations[serverBackend] != ""
	if !hasBackend {
		if result := m.updateBackend(ctx, planner); result.Yield() {
			return result
		}
	} else {
		if result := m.validateBackend(planner); result.Yield() {
			return result
		}
	}

	if planner.IsClustered() {
		if result := m.updateClusteredState(ctx, planner); result.Yield() {
			return result
		}
	} else {
		if result := m.updateNonClusteredState(ctx, planner); result.Yield() {
			return result
		}
	}

	if result := m.updateSmbService(ctx, planner); result.Yield() {
		return result
	}

	if result := m.updateMetricsService(ctx, planner); result.Yield() {
		return result
	}

	if result := m.updateMetricsServiceMonitor(ctx, planner); result.Yield() {
		return result
	}

	m.logger.Info("Done updating SmbShare resources")
	return Done
}

func (m *SmbShareManager) updateConfigMap(
	ctx context.Context,
	smbshare *sambaoperatorv1alpha1.SmbShare) (*pln.Planner, Result) {
	// ---
	destNamespace := smbshare.Namespace
	cm, created, err := m.getOrCreateConfigMap(ctx, smbshare, destNamespace)
	if err != nil {
		return nil, Result{err: err}
	} else if created {
		m.logger.Info("Created config map")
		return nil, Requeue
	}
	changed, err := m.claimOwnership(ctx, smbshare, cm)
	if err != nil {
		return nil, Result{err: err}
	} else if changed {
		m.logger.Info("Updated config map ownership")
		return nil, Requeue
	}
	planner, changed, err := m.updateConfiguration(ctx, cm, smbshare)
	if err != nil {
		return nil, Result{err: err}
	} else if changed {
		m.logger.Info("Updated config map")
		return nil, Requeue
	}
	return planner, Done
}

func (m *SmbShareManager) updatePVC(
	ctx context.Context,
	smbshare *sambaoperatorv1alpha1.SmbShare) Result {
	// ---
	pvc, created, err := m.getOrCreatePvc(
		ctx, smbshare, smbshare.Namespace)
	if err != nil {
		return Result{err: err}
	} else if created {
		m.logger.Info("Created PVC")
		m.recorder.Eventf(smbshare,
			EventNormal,
			ReasonCreatedPersistentVolumeClaim,
			"Created PVC %s for SmbShare", pvc.Name)
		return Requeue
	}
	// if name is unset in the YAML, set it here
	smbshare.Spec.Storage.Pvc.Name = pvc.Name
	return Done
}

func (m *SmbShareManager) updateBackend(
	ctx context.Context,
	planner *pln.Planner) Result {
	// ---
	var (
		err      error
		smbshare = planner.SmbShare
	)
	if smbshare.Annotations == nil {
		smbshare.Annotations = map[string]string{}
	}
	if planner.IsClustered() {
		smbshare.Annotations[serverBackend] = clusteredBackend
	} else {
		smbshare.Annotations[serverBackend] = standardBackend
	}
	m.logger.Info("Setting backend",
		"SmbShare.Namespace", smbshare.Namespace,
		"SmbShare.Name", smbshare.Name,
		"SmbShare.UID", smbshare.UID,
		"backend", smbshare.Annotations[serverBackend])
	err = m.client.Update(ctx, smbshare)
	if err != nil {
		m.logger.Error(
			err,
			"Failed to update SmbShare",
			"SmbShare.Namespace", smbshare.Namespace,
			"SmbShare.Name", smbshare.Name,
			"SmbShare.UID", smbshare.UID)
		return Result{err: err}
	}
	return Requeue
}

func (m *SmbShareManager) validateBackend(
	planner *pln.Planner) Result {
	// ---
	var (
		err      error
		smbshare = planner.SmbShare
	)
	// As of today, the system is too immature to try and trivially
	// reconcile a change in availability mode. We use the previously
	// recorded backend to check if a change is being made after the
	// "point of no return".
	// In the future we should certainly revisit this and support
	// intelligent methods to handle changes like this.
	b := smbshare.Annotations[serverBackend]
	if planner.IsClustered() && b != clusteredBackend {
		err = fmt.Errorf(
			"Can not convert SmbShare to clustered instance."+
				" Current backend: %s",
			b)
		m.logger.Error(
			err,
			"Backend inconsistency detected",
			"SmbShare.Namespace", smbshare.Namespace,
			"SmbShare.Name", smbshare.Name,
			"SmbShare.UID", smbshare.UID)
		return Result{err: err}
	}
	if !planner.IsClustered() && b != standardBackend {
		err = fmt.Errorf(
			"Can not convert SmbShare to non-clustered instance."+
				" Current backend: %s",
			b)
		m.logger.Error(
			err,
			"Backend inconsistency detected",
			"SmbShare.Namespace", smbshare.Namespace,
			"SmbShare.Name", smbshare.Name,
			"SmbShare.UID", smbshare.UID)
		return Result{err: err}
	}
	return Done
}

func (m *SmbShareManager) updateClusteredState(
	ctx context.Context,
	planner *pln.Planner) Result {
	// ---
	var err error
	if !planner.MayCluster() {
		err = fmt.Errorf(
			"CTDB clustering not enabled in ClusterSupport: %v",
			planner.GlobalConfig.ClusterSupport)
		m.logger.Error(err, "Clustering support is not enabled")
		return Result{err: err}
	}
	_, created, err := m.getOrCreateStatePVC(
		ctx, planner, planner.SmbShare.Namespace)
	if err != nil {
		return Result{err: err}
	} else if created {
		m.logger.Info("Created shared state PVC")
		return Requeue
	}

	statefulSet, created, err := m.getOrCreateStatefulSet(
		ctx, planner, planner.SmbShare.Namespace)
	if err != nil {
		return Result{err: err}
	} else if created {
		// StatefulSet created successfully - return and requeue
		m.logger.Info("Created StatefulSet")
		m.recorder.Eventf(planner.SmbShare,
			EventNormal,
			ReasonCreatedStatefulSet,
			"Created stateful set %s for SmbShare", statefulSet.Name)
		return Requeue
	}

	changed, err := m.claimOwnership(ctx, planner.SmbShare, statefulSet)
	if err != nil {
		return Result{err: err}
	} else if changed {
		m.logger.Info("Updated stateful set ownership")
		return Requeue
	}

	resized, err := m.updateStatefulSetSize(
		ctx, statefulSet,
		int32(planner.SmbShare.Spec.Scaling.MinClusterSize))
	if err != nil {
		return Result{err: err}
	} else if resized {
		m.logger.Info("Resized statefulSet")
		return Requeue
	}
	return Done
}

func (m *SmbShareManager) updateNonClusteredState(
	ctx context.Context,
	planner *pln.Planner) Result {
	// ---
	var err error
	deployment, created, err := m.getOrCreateDeployment(
		ctx, planner, planner.SmbShare.Namespace)
	if err != nil {
		return Result{err: err}
	} else if created {
		// Deployment created successfully - return and requeue
		m.logger.Info("Created deployment")
		m.recorder.Eventf(planner.SmbShare,
			EventNormal,
			ReasonCreatedDeployment,
			"Created deployment %s for SmbShare", deployment.Name)
		return Requeue
	}

	changed, err := m.claimOwnership(ctx, planner.SmbShare, deployment)
	if err != nil {
		return Result{err: err}
	} else if changed {
		m.logger.Info("Updated deployment ownership")
		return Requeue
	}

	resized, err := m.updateDeploymentSize(ctx, deployment)
	if err != nil {
		return Result{err: err}
	} else if resized {
		m.logger.Info("Resized deployment")
		return Requeue
	}
	return Done
}

func (m *SmbShareManager) updateSmbService(
	ctx context.Context,
	planner *pln.Planner) Result {
	// ---
	svc, created, err := m.getOrCreateService(
		ctx, planner, planner.SmbShare.Namespace)
	if err != nil {
		return Result{err: err}
	} else if created {
		m.logger.Info("Created service")
		return Requeue
	}

	changed, err := m.claimOwnership(ctx, planner.SmbShare, svc)
	if err != nil {
		return Result{err: err}
	} else if changed {
		m.logger.Info("Updated service ownership")
		return Requeue
	}

	return Done
}

// TODO needs ownership?
func (m *SmbShareManager) updateMetricsService(
	ctx context.Context,
	planner *pln.Planner) Result {
	// ---
	_, created, err := m.getOrCreateMetricsService(
		ctx, planner, planner.SmbShare.Namespace)
	if err != nil {
		return Result{err: err}
	} else if created {
		m.logger.Info("Created metrics service")
		return Requeue
	}
	return Done
}

// TODO needs ownership?
func (m *SmbShareManager) updateMetricsServiceMonitor(
	ctx context.Context,
	planner *pln.Planner) Result {
	// ---
	_, created, err := m.getOrCreateMetricsServiceMonitor(
		ctx, planner, planner.SmbShare.Namespace)
	if err != nil {
		return Result{err: err}
	} else if created {
		m.logger.Info("Created metrics servicemonitor")
		return Requeue
	}
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
	} else if !errors.IsNotFound(err) {
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
	planner *pln.Planner,
	ns string) (*appsv1.Deployment, bool, error) {
	// Check if the deployment already exists, if not create a new one
	depKey := types.NamespacedName{
		Name:      planner.InstanceName(),
		Namespace: ns,
	}
	found := &appsv1.Deployment{}
	err := m.client.Get(ctx, depKey, found)
	if err == nil {
		return found, false, nil
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
		return nil, false, err
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

func (m *SmbShareManager) getOrCreateGenericPVC(
	ctx context.Context,
	smbShare *sambaoperatorv1alpha1.SmbShare,
	spec *corev1.PersistentVolumeClaimSpec,
	name, ns string) (*corev1.PersistentVolumeClaim, bool, error) {
	// Check if the pvc already exists, if not create it
	pvc := &corev1.PersistentVolumeClaim{}
	pvcKey := types.NamespacedName{
		Name:      name,
		Namespace: ns,
	}
	err := m.client.Get(ctx, pvcKey, pvc)
	if err == nil {
		return pvc, false, nil
	}

	if !errors.IsNotFound(err) {
		// unexpected error!
		m.logger.Error(
			err,
			"Failed to get PVC",
			"SmbShare.Namespace", smbShare.Namespace,
			"SmbShare.Name", smbShare.Name,
			"PersistentVolumeClaim.Namespace", pvcKey.Namespace,
			"PersistentVolumeClaim.Name", pvcKey.Name)
		return nil, false, err
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

func (m *SmbShareManager) getOrCreateStatefulSet(
	ctx context.Context,
	planner *pln.Planner,
	ns string) (*appsv1.StatefulSet, bool, error) {
	// Check if the ss already exists, if not create a new one
	found := &appsv1.StatefulSet{}
	ssKey := types.NamespacedName{
		Name:      planner.InstanceName(),
		Namespace: ns,
	}
	err := m.client.Get(ctx, ssKey, found)
	if err == nil {
		return found, false, nil
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
		return nil, false, err
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

func (m *SmbShareManager) updateStatefulSetSize(
	ctx context.Context,
	statefulSet *appsv1.StatefulSet,
	size int32) (bool, error) {
	// Ensure the StatefulSet size is the same as the spec
	if *statefulSet.Spec.Replicas < size {
		statefulSet.Spec.Replicas = &size
		err := m.client.Update(ctx, statefulSet)
		if err != nil {
			m.logger.Error(
				err,
				"Failed to update StatefulSet",
				"StatefulSet.Namespace", statefulSet.Namespace,
				"StatefulSet.Name", statefulSet.Name)
			return false, err
		}
		return true, nil
	}
	return false, nil
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

func sharedStatePVCName(planner *pln.Planner) string {
	return planner.InstanceName() + "-state"
}

func shareNeedsPvc(s *sambaoperatorv1alpha1.SmbShare) bool {
	return s.Spec.Storage.Pvc != nil && s.Spec.Storage.Pvc.Spec != nil
}

func (m *SmbShareManager) updateConfiguration(
	ctx context.Context,
	cm *corev1.ConfigMap,
	s *sambaoperatorv1alpha1.SmbShare) (*pln.Planner, bool, error) {
	// extract config from map
	cc, err := getContainerConfig(cm)
	if err != nil {
		m.logger.Error(err, "unable to read samba container config")
		return nil, false, err
	}
	otherShares, err := ownerSharesExcluding(cm, s)
	if err != nil {
		m.logger.Error(err, "unable to get shares owning config map")
		return nil, false, err
	}

	isDeleting := s.GetDeletionTimestamp() != nil
	if isDeleting {
		m.logger.Info(
			"SmbShare is being deleted - using a minimal planner")
		planner := pln.New(
			pln.InstanceConfiguration{
				SmbShare:     s,
				GlobalConfig: m.cfg,
			},
			nil)
		return planner, false, nil
	}

	shareInstance, err := m.getShareInstance(ctx, s)
	if err != nil {
		return nil, false, err
	}

	if len(otherShares) > 0 {
		// this server group will be hosting > 1 share, but we must
		// first pass our sanity checks
		other, err := m.getSmbShareByName(ctx, otherShares[0])
		if err != nil {
			return nil, false, err
		}
		otherInstance, err := m.getShareInstance(ctx, other)
		if err != nil {
			return nil, false, err
		}
		if err = pln.CheckCompatible(shareInstance, otherInstance); err != nil {
			m.recorder.Event(
				s,
				EventWarning,
				ReasonInvalidConfiguration,
				err.Error())
			return nil, false, err
		}
	}

	// extract config from map
	var changed bool
	planner := pln.New(shareInstance, cc)
	changed, err = planner.Update()
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

func (m *SmbShareManager) setServerGroup(
	ctx context.Context, s *sambaoperatorv1alpha1.SmbShare) (bool, error) {
	// check to see if there's already a group for this
	if s.Status.ServerGroup != "" {
		// already assigned, nothing extra to do
		return false, nil
	}

	// if the share's scaling.groupMode option allows >1 share per
	// serverGroup instance, we allow the group name to be supplied
	// by the scaling.group option.
	// In other cases it's based on the name of the SmbShare resource.
	serverGroup := s.ObjectMeta.Name
	mode := pln.GroupModeNever
	reqGroupName := ""
	if s.Spec.Scaling != nil {
		mode = pln.GroupMode(s.Spec.Scaling.GroupMode)
		reqGroupName = s.Spec.Scaling.Group
	}

	switch mode {
	case pln.GroupModeNever, pln.GroupModeUnset:
		if reqGroupName != "" {
			msg := "a group name may not be specified when groupMode is 'never'"
			m.recorder.Event(
				s,
				EventWarning,
				ReasonInvalidConfiguration,
				msg)
			return false, fmt.Errorf(msg)
		}
	case pln.GroupModeExplicit:
		if reqGroupName == "" {
			msg := "a group name is required when groupMode is 'explicit'"
			m.recorder.Event(
				s,
				EventWarning,
				ReasonInvalidConfiguration,
				msg)
			return false, fmt.Errorf(msg)
		}
		serverGroup = reqGroupName
	default:
		msg := "invalid group mode"
		m.recorder.Event(
			s,
			EventWarning,
			ReasonInvalidConfiguration,
			msg)
		return false, fmt.Errorf(msg)
	}

	s.Status.ServerGroup = serverGroup
	return true, m.client.Status().Update(ctx, s)
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

func (m *SmbShareManager) claimOwnership(
	ctx context.Context,
	s *sambaoperatorv1alpha1.SmbShare,
	obj rtclient.Object) (bool, error) {
	// ---
	gvk, err := apiutil.GVKForObject(s, m.scheme)
	if err != nil {
		return false, err
	}
	refs := obj.GetOwnerReferences()
	for _, ref := range refs {
		refgv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return false, err
		}
		if gvk.Group == refgv.Group && gvk.Kind == ref.Kind && s.GetName() == ref.Name {
			// found it!  return false to indicate no changes
			return false, nil
		}
	}
	oref := metav1.OwnerReference{
		APIVersion: gvk.GroupVersion().String(),
		Kind:       gvk.Kind,
		UID:        s.GetUID(),
		Name:       s.GetName(),
	}
	refs = append(refs, oref)
	obj.SetOwnerReferences(refs)
	return true, m.client.Update(ctx, obj)
}

// transferOwnership away from the specified SmbShare.
func (m *SmbShareManager) transferOwnership(
	ctx context.Context,
	obj rtclient.Object,
	previous *sambaoperatorv1alpha1.SmbShare) Result {
	// ---
	refs, err := smbShareOwnerRefs(obj)
	if err != nil {
		m.logger.Error(err, "Failed to get share owner references")
		return Result{err: err}
	}
	refs = excludeOwnerRefs(refs, previous.GetName(), previous.GetUID())
	if len(refs) == 0 {
		m.logger.Info("Object has no other possible owners", "Object", obj)
		return Done
	}
	for _, ref := range refs {
		if ref.Controller != nil && *ref.Controller {
			m.logger.Info(
				"Previous owner is not controller-owner. No transfer needed.",
				"Object",
				obj,
				"controllerOwner.Name",
				ref.Name,
			)
			return Done
		}
	}

	// no share in the owner refs is a controlling owner.
	// find the first valid ref and make it an owner
	var chosenRef metav1.OwnerReference
	for _, ref := range refs {
		name := types.NamespacedName{
			Namespace: obj.GetNamespace(),
			Name:      ref.Name,
		}
		s, err := m.getSmbShareByName(ctx, name)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			m.logger.Error(
				err,
				"Failed to fetch alternative owner",
				"Object",
				obj,
				"OtherOwner.Name",
				name,
			)
			return Result{err: err}
		}
		if s.GetDeletionTimestamp() == nil {
			chosenRef = ref
			break
		}
	}

	if chosenRef.Name == "" {
		// nothing valid was found.
		m.logger.Info("No new valid owners found: skipping ownership transfer",
			"Object",
			obj,
		)
		return Done
	}
	m.logger.Info("Chose a new controller-owner share",
		"Object",
		obj,
		"newControllerOwner.Name",
		chosenRef.Name,
		"newControllerOwner.UID",
		chosenRef.UID,
	)
	changeControllerOwnerTo(obj, &chosenRef)
	if err := m.client.Update(ctx, obj); err != nil {
		m.logger.Error(err, "Failed to update ownership", "Object", obj)
		return Result{err: err}
	}
	m.logger.Info(
		"Updated controlling ownership",
		"Object",
		obj,
		"NewControllerOwner.Name",
		chosenRef.Name,
	)
	return Requeue
}

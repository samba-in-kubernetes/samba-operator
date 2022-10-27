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
	}
	if changed {
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
	}
	if changed {
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
	}
	if created {
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
	}
	if changed {
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
	}
	if created {
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
	}
	if created {
		m.logger.Info("Created shared state PVC")
		return Requeue
	}

	statefulSet, created, err := m.getOrCreateStatefulSet(
		ctx, planner, planner.SmbShare.Namespace)
	if err != nil {
		return Result{err: err}
	}
	if created {
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
	}
	if resized {
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
	}
	if created {
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
	}
	if resized {
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
	}
	if created {
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
	}
	if created {
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
	}
	if created {
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
	if result := m.finalizeConfigMap(ctx, instance); result.Yield() {
		return result
	}

	if result := m.finalizeServerResources(ctx, instance); result.Yield() {
		return result
	}

	// If the share defined the PVC we'll transfer ownership.  It's probably
	// not a best practice to embed the pvc definition and group multiple
	// shares under one server instance, but we should at least handle it
	// sanely in case it is done.
	if shareNeedsPvc(instance) {
		if result := m.finalizeDataPVC(ctx, instance); result.Yield() {
			return result
		}
	}

	m.logger.Info("Removing finalizer")
	controllerutil.RemoveFinalizer(instance, shareFinalizer)
	err := m.client.Update(ctx, instance)
	if err != nil {
		return Result{err: err}
	}
	return Done
}

func (m *SmbShareManager) finalizeConfigMap(
	ctx context.Context,
	smbshare *sambaoperatorv1alpha1.SmbShare) Result {
	// ---
	destNamespace := smbshare.Namespace
	cm, err := m.getConfigMap(ctx, smbshare, destNamespace)
	if err == nil {
		if result := m.transferOwnership(ctx, cm, smbshare); result.Yield() {
			return result
		}
		// if this configmap is used by >1 share we will need to prune
		// the old share from the configuration.
		changed, err := m.pruneConfiguration(ctx, cm, smbshare)
		if err != nil {
			return Result{err: err}
		}
		if changed {
			m.logger.Info("Updated config map during Finalize")
			return Requeue
		}
	} else if !errors.IsNotFound(err) {
		return Result{err: err}
	}
	return Done
}

func (m *SmbShareManager) finalizeServerResources(
	ctx context.Context,
	smbshare *sambaoperatorv1alpha1.SmbShare) Result {
	// ---
	destNamespace := smbshare.Namespace
	shareInstance := pln.InstanceConfiguration{
		SmbShare:     smbshare,
		GlobalConfig: m.cfg,
	}
	// create a minimal planner
	planner := pln.New(shareInstance, nil)

	if planner.IsClustered() {
		ss, err := m.getExistingStatefulSet(ctx, planner, destNamespace)
		if err != nil {
			return Result{err: err}
		}
		if ss != nil {
			if result := m.transferOwnership(ctx, ss, smbshare); result.Yield() {
				return result
			}
		}

		sspvc, err := m.getExistingPVC(ctx, sharedStatePVCName(planner), destNamespace)
		if err != nil {
			return Result{err: err}
		}
		if sspvc != nil {
			if result := m.transferOwnership(ctx, sspvc, smbshare); result.Yield() {
				return result
			}
		}
	} else {
		deployment, err := m.getExistingDeployment(ctx, planner, destNamespace)
		if err != nil {
			return Result{err: err}
		}
		if deployment != nil {
			if result := m.transferOwnership(ctx, deployment, smbshare); result.Yield() {
				return result
			}
		}
	}
	return Done
}

func (m *SmbShareManager) finalizeDataPVC(
	ctx context.Context,
	smbshare *sambaoperatorv1alpha1.SmbShare) Result {
	// ---
	destNamespace := smbshare.Namespace
	name := pvcName(smbshare)
	pvc, err := m.getExistingPVC(ctx, name, destNamespace)
	if err != nil {
		return Result{err: err}
	}
	if pvc != nil {
		if result := m.transferOwnership(ctx, pvc, smbshare); result.Yield() {
			return result
		}
	}
	return Done
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
		err := fmt.Errorf(
			"updateConfiguration called for deleted SmbShare: %s",
			s.Name)
		return nil, false, err
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

func (m *SmbShareManager) pruneConfiguration(
	ctx context.Context,
	cm *corev1.ConfigMap,
	s *sambaoperatorv1alpha1.SmbShare) (bool, error) {
	// ---
	cc, err := getContainerConfig(cm)
	if err != nil {
		m.logger.Error(err, "unable to read samba container config")
		return false, err
	}

	isDeleting := s.GetDeletionTimestamp() != nil
	if !isDeleting {
		err := fmt.Errorf(
			"pruneConfiguration called for SmbShare not being deleted: %s",
			s.Name)
		return false, err
	}

	var changed bool
	// currently, the global/common/security settings have no impact on
	// pruning the config. We always assume if a share is in the map it
	// had matching settings. So we can skip fetching those resources,
	// which is nice esp. if they have already been deleted.
	shareInstance := pln.InstanceConfiguration{
		SmbShare:     s,
		GlobalConfig: m.cfg,
	}
	planner := pln.New(shareInstance, cc)
	changed, err = planner.Prune()
	if err != nil {
		m.logger.Error(err, "unable to update samba container config")
		return false, err
	}
	if !changed {
		// nothing changed between the planner and the config stored in the cm
		// we can just return now as no changes need to be applied to the cm
		return false, nil
	}
	err = setContainerConfig(cm, planner.ConfigState)
	if err != nil {
		m.logger.Error(
			err,
			"unable to set container config in ConfigMap",
			"ConfigMap.Namespace", cm.Namespace,
			"ConfigMap.Name", cm.Name)
		return false, err
	}
	err = m.client.Update(ctx, cm)
	if err != nil {
		m.logger.Error(
			err,
			"failed to update ConfigMap",
			"ConfigMap.Namespace", cm.Namespace,
			"ConfigMap.Name", cm.Name)
		return false, err
	}
	return true, nil
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

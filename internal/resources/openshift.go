// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	rtclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
	"github.com/samba-in-kubernetes/samba-operator/internal/conf"
)

// ClusterType reperesnts the sub-kind of k8s cluster type.
type ClusterType string

const (
	// ClusterTypeDefault defines the default value for cluster type
	ClusterTypeDefault = "default"
	// ClusterTypeOpenshift defines the type-name for OpenShift clusters
	ClusterTypeOpenshift = "openshift"
)

const (
	openshiftFinalizer        = "samba-operator.samba.org/openshiftFinalizer"
	serviceAccountName        = "samba"
	sccRoleName               = "samba-scc-role"
	sccRoleBindingName        = "samba-scc-rolebinding"
	metricsRoleName           = "samba-metrics-role"
	metricsRoleBindingName    = "samba-metrics-rolebinding"
	clusterMonitoringLabelKey = "openshift.io/cluster-monitoring"
)

// OpenShiftManager is used to manage OpenShift's related resources: SCC,
// ServiceAccount (if not exists) and enable Role and RoleBinding referencing
// to OpenShift's 'samba' SCC.
type OpenShiftManager struct {
	client      rtclient.Client
	logger      logr.Logger
	cfg         *conf.OperatorConfig
	ClusterType ClusterType
}

// NewOpenShiftManager creates a ServiceAccountManager instance
func NewOpenShiftManager(
	clnt rtclient.Client,
	log logr.Logger,
	cfg *conf.OperatorConfig) *OpenShiftManager {
	return &OpenShiftManager{
		client: clnt,
		logger: log,
		cfg:    cfg,
	}
}

// Process is called by the controller on reconciliation of
// SmbCommonConfig
func (m *OpenShiftManager) Process(
	ctx context.Context,
	nsname types.NamespacedName) Result {
	// Do-nothing if not on OpenShift
	m.resolveClusterType(ctx)
	if m.ClusterType != ClusterTypeOpenshift {
		return Done
	}

	// require resource
	cc := &sambaoperatorv1alpha1.SmbCommonConfig{}
	err := m.client.Get(ctx, nsname, cc)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found. Not a fatal error.
			return Done
		}
		m.logger.Error(err,
			"Failed to get SmbCommonConfig",
			"SmbCommonConfig.Namespace", nsname.Namespace,
			"SmbCommonConfig.Name", nsname.Name)
		return Result{err: err}
	}

	// now that we have the resource. determine if its live or pending deletion
	if cc.GetDeletionTimestamp() != nil {
		// its being deleted
		if controllerutil.ContainsFinalizer(cc, openshiftFinalizer) {
			// and our finalizer is present
			return m.Finalize(ctx, cc)
		}
		return Done
	}
	// resource is alive
	return m.Update(ctx, cc)
}

// Cache cluster type
func (m *OpenShiftManager) resolveClusterType(ctx context.Context) {
	if IsOpenShiftCluster(ctx, m.client, m.cfg) {
		m.ClusterType = ClusterTypeOpenshift
	} else {
		m.ClusterType = ClusterTypeDefault
	}
}

// Update should be called when a SmbCommonConfig resource changes.
// nolint:funlen
func (m *OpenShiftManager) Update(
	ctx context.Context,
	cc *sambaoperatorv1alpha1.SmbCommonConfig) Result {
	// ---
	m.logger.Info(
		"Updating state for SmbCommonConfig on OpenShift",
		"SmbCommonConfig.Namespace", cc.Namespace,
		"SmbCommonConfig.Name", cc.Name,
		"SmbCommonConfig.UID", cc.UID)

	changed, err := m.addFinalizer(ctx, cc)
	if err != nil {
		return Result{err: err}
	}
	if changed {
		m.logger.Info("Added OpenShift finalizer")
		return Requeue
	}

	ns, updated, err := m.getOrUpdateNamespace(ctx, cc.Namespace)
	if err != nil {
		return Result{err: err}
	}
	if updated {
		m.logger.Info("Updated Namespace labels",
			"Namespace.Name", ns.Name,
			"Namespace.Labels", ns.Labels)
		return Requeue
	}

	sa, created, err := m.getOrCreateServiceAccount(ctx)
	if err != nil {
		return Result{err: err}
	}
	if created {
		m.logger.Info("Created ServiceAccount",
			"ServiceAccount.Namespace", sa.Namespace,
			"ServiceAccount.Name", sa.Name)
		return Requeue
	}

	sccRole, created, err := m.getOrCreateSCCRole(ctx)
	if err != nil {
		return Result{err: err}
	}
	if created {
		m.logger.Info("Created SCC Role",
			"SCCRole.Namespace", sccRole.Namespace,
			"SCCRole.Name", sccRole.Name)
		return Requeue
	}

	sccRoleBind, created, err := m.getOrCreateSCCRoleBinding(ctx, sa, sccRole)
	if err != nil {
		return Result{err: err}
	}
	if created {
		m.logger.Info("Created SCC RoleBinding",
			"SCCRoleBinding.Namespace", sccRoleBind.Namespace,
			"SCCRoleBinding.Name", sccRoleBind.Name)
		return Requeue
	}

	metricsRole, created, err := m.getOrCreateMetricsRole(ctx)
	if err != nil {
		return Result{err: err}
	}
	if created {
		m.logger.Info("Created Metrics Role",
			"MetricsRole.Namespace", metricsRole.Namespace,
			"MetricsRole.Name", metricsRole.Name)
		return Requeue
	}

	metricsRoleBind, created, err :=
		m.getOrCreateMetricsRoleBinding(ctx, metricsRole)
	if err != nil {
		return Result{err: err}
	}
	if created {
		m.logger.Info("Created Metrics RoleBinding",
			"MetricsRoleBinding.Namespace", metricsRoleBind.Namespace,
			"MetricsRoleBinding.Name", metricsRoleBind.Name)
		return Requeue
	}

	m.logger.Info("Done updating SmbCommonConfig resources on OpenShift",
		"Namespace.Name", ns.Name,
		"ServiceAccount.Name", sa.Name,
		"SCCRole.Namespace", sccRole.Namespace,
		"SCCRole.Name", sccRole.Name,
		"MetricsRoleBinding.Namespace", metricsRoleBind.Namespace,
		"MetricsRoleBinding.Name", metricsRoleBind.Name)

	return Done
}

// Finalize should be called when there's a finalizer on the resource
// and we need to do some cleanup.
func (m *OpenShiftManager) Finalize(
	ctx context.Context,
	cc *sambaoperatorv1alpha1.SmbCommonConfig) Result {
	// ---
	m.logger.Info(
		"Finalize state for SmbCommonConfig on OpenShift",
		"SmbCommonConfig.Namespace", cc.Namespace,
		"SmbCommonConfig.Name", cc.Name,
		"SmbCommonConfig.UID", cc.UID)

	metricsRoleBind, metricsRoleBindKey, err := m.getMetricsRoleBinding(ctx)
	if err == nil {
		err = m.client.Delete(ctx, metricsRoleBind, &rtclient.DeleteOptions{})
		if err != nil {
			m.logger.Error(err,
				"Failed to finalize Metrics RoleBinding",
				"key", metricsRoleBindKey)
			return Result{err: err}
		}
		m.logger.Info("Deleted Metrics RoleBinding", "key", metricsRoleBindKey)
		return Requeue
	} else if !errors.IsNotFound(err) {
		m.logger.Error(err, "Failed to get Metrics RoleBinding",
			"key", metricsRoleBindKey)
	}

	metricsRole, metricsRoleKey, err := m.getMetricsRole(ctx)
	if err == nil {
		err = m.client.Delete(ctx, metricsRole, &rtclient.DeleteOptions{})
		if err != nil {
			m.logger.Error(err,
				"Failed to finalize Metrics Role", "key", metricsRoleKey)
			return Result{err: err}
		}
		m.logger.Info("Deleted Metrics Role", "key", metricsRoleKey)
		return Requeue
	} else if !errors.IsNotFound(err) {
		m.logger.Error(err, "Failed to get Metrics Role",
			"key", metricsRoleKey)
	}

	sccRoleBind, sccRoleBindKey, err := m.getSCCRoleBinding(ctx)
	if err == nil {
		err = m.client.Delete(ctx, sccRoleBind, &rtclient.DeleteOptions{})
		if err != nil {
			m.logger.Error(err,
				"Failed to finalize SCC RoleBinding", "key", sccRoleBindKey)
			return Result{err: err}
		}
		m.logger.Info("Deleted SCC RoleBinding", "key", sccRoleBindKey)
		return Requeue
	} else if !errors.IsNotFound(err) {
		m.logger.Error(err, "Failed to get SCC RoleBinding",
			"key", sccRoleBindKey)
	}

	sccRole, sccRoleKey, err := m.getSCCRole(ctx)
	if err == nil {
		err = m.client.Delete(ctx, sccRole, &rtclient.DeleteOptions{})
		if err != nil {
			m.logger.Error(err,
				"Failed to finalize SCC Role", "key", sccRoleKey)
			return Result{err: err}
		}
		m.logger.Info("Deleted SCC Role", "key", sccRoleKey)
		return Requeue
	} else if !errors.IsNotFound(err) {
		m.logger.Error(err, "Failed to get SCC Role", "key", sccRoleKey)
	}

	sa, saKey, err := m.getServiceAccount(ctx)
	if err == nil {
		err = m.client.Delete(ctx, sa, &rtclient.DeleteOptions{})
		if err != nil {
			m.logger.Error(err,
				"Failed to finalize ServiceAccount", "key", saKey)
			return Result{err: err}
		}
		m.logger.Info("Deleted ServiceAccount", "key", saKey)
		return Requeue
	} else if !errors.IsNotFound(err) {
		m.logger.Error(err, "Failed to get ServiceAccount", "key", saKey)
	}

	m.logger.Info("Removing OpenShift finalizer")
	controllerutil.RemoveFinalizer(cc, openshiftFinalizer)
	err = m.client.Update(ctx, cc)
	if err != nil {
		return Result{err: err}
	}
	return Done
}

// getOrUpdateNamespace expects operator's namespace to exist with proper
// labels for OpenShift deployment
func (m *OpenShiftManager) getOrUpdateNamespace(
	ctx context.Context, nsname string) (*corev1.Namespace, bool, error) {
	// ---
	nsCurr, err := m.getNamespace(ctx, nsname)
	if err != nil {
		m.logger.Error(err, "Failed to get Namespace", "nsname", nsname)
		return nil, false, err
	}
	labelKey := clusterMonitoringLabelKey
	labelVal := "true"
	if nsCurr.Labels[labelKey] == labelVal {
		return nsCurr, false, nil // OK
	}
	nsWant := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   nsname,
			Labels: nsCurr.Labels,
		},
	}
	nsCurr.Spec.DeepCopyInto(&nsWant.Spec)
	nsWant.Labels[labelKey] = labelVal
	err = m.client.Update(ctx, nsWant)
	if err != nil {
		m.logger.Error(err, "Failed to update Namespace", "nsname", nsname)
		return nil, false, err
	}
	return nsWant, true, nil
}

func (m *OpenShiftManager) getNamespace(
	ctx context.Context, nsname string) (*corev1.Namespace, error) {
	// ---
	nsKey := types.NamespacedName{Name: nsname}
	ns := &corev1.Namespace{}
	err := m.client.Get(ctx, nsKey, ns)
	if err != nil {
		return nil, err
	}
	return ns, err
}

func (m *OpenShiftManager) getOrCreateServiceAccount(
	ctx context.Context) (*corev1.ServiceAccount, bool, error) {
	// ---
	saCurr, saKey, err := m.getServiceAccount(ctx)
	if err == nil {
		return saCurr, false, nil // OK
	}
	if !errors.IsNotFound(err) {
		m.logger.Error(err, "Failed to get ServiceAccount", "key", saKey)
		return nil, false, err
	}
	saWant := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: saKey.Namespace,
			Name:      saKey.Name,
		},
	}
	err = m.client.Create(ctx, saWant, &rtclient.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			m.logger.Info("ServiceAccount already exists", "key", saKey)
		} else {
			m.logger.Error(err, "Failed to create ServiceAccount",
				"key", saKey)
		}
		return nil, false, err
	}
	return saWant, true, nil
}

func (m *OpenShiftManager) getServiceAccount(
	ctx context.Context) (*corev1.ServiceAccount, types.NamespacedName, error) {
	// ---
	saKey := m.serviceAccountNsname()
	sa := &corev1.ServiceAccount{}
	err := m.client.Get(ctx, saKey, sa)
	if err != nil {
		return nil, saKey, err
	}
	return sa, saKey, nil
}

func (m *OpenShiftManager) serviceAccountNsname() types.NamespacedName {
	if len(m.cfg.ServiceAccountName) > 0 {
		return types.NamespacedName{Name: m.cfg.ServiceAccountName}
	}
	return types.NamespacedName{
		Namespace: m.cfg.PodNamespace,
		Name:      serviceAccountName,
	}
}

func (m *OpenShiftManager) getOrCreateSCCRole(
	ctx context.Context) (*rbacv1.Role, bool, error) {
	// ---
	roleCurr, roleKey, err := m.getSCCRole(ctx)
	if err == nil {
		return roleCurr, false, err // OK
	}
	if !errors.IsNotFound(err) {
		m.logger.Error(err, "Failed to get SCC Role", "key", roleKey)
		return nil, false, err
	}
	roleWant := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: roleKey.Namespace,
			Name:      roleKey.Name,
		},
		Rules: []rbacv1.PolicyRule{
			{
				// TODO: ask John if he prefers import openshift for those vals
				APIGroups:     []string{"security.openshift.io"},
				Resources:     []string{"securitycontextconstraints"},
				ResourceNames: []string{"samba"},
				Verbs:         []string{"use"},
			},
		},
	}
	err = m.client.Create(ctx, roleWant, &rtclient.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			m.logger.Info("SCC Role already exists", "key", roleKey)
		} else {
			m.logger.Error(err, "Failed to create SCC Role", "key", roleKey)
		}
		return nil, false, err
	}
	return roleWant, true, nil
}

func (m *OpenShiftManager) getSCCRole(
	ctx context.Context) (*rbacv1.Role, types.NamespacedName, error) {
	// ---
	roleKey := m.sccRoleNsname()
	role := &rbacv1.Role{}
	err := m.client.Get(ctx, roleKey, role)
	if err != nil {
		return nil, roleKey, err
	}
	return role, roleKey, nil
}

func (m *OpenShiftManager) getOrCreateSCCRoleBinding(
	ctx context.Context,
	sa *corev1.ServiceAccount,
	role *rbacv1.Role) (*rbacv1.RoleBinding, bool, error) {
	// ---
	roleBindCurr, roleBindKey, err := m.getSCCRoleBinding(ctx)
	if err == nil {
		return roleBindCurr, false, err // OK
	}
	if !errors.IsNotFound(err) {
		m.logger.Error(err, "Failed to get RoleBinding", "key", roleBindKey)
		return nil, false, err
	}
	roleBindWant := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: roleBindKey.Namespace,
			Name:      roleBindKey.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      sa.GetName(),
				Namespace: sa.GetNamespace(),
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "Role",
			Name: role.GetName(),
		},
	}
	err = m.client.Create(ctx, roleBindWant, &rtclient.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			m.logger.Info("SCC RoleBinding already exists", "key", roleBindKey)
		} else {
			m.logger.Error(err, "Failed to create RoleBinding",
				"key", roleBindKey)
		}
		return nil, false, err
	}
	return roleBindWant, true, nil
}

func (m *OpenShiftManager) getSCCRoleBinding(
	ctx context.Context) (*rbacv1.RoleBinding, types.NamespacedName, error) {
	// ---
	roleBindKey := m.sccRoleBindingNsname()
	roleBind := &rbacv1.RoleBinding{}
	err := m.client.Get(ctx, roleBindKey, roleBind)
	if err != nil {
		return nil, roleBindKey, err
	}
	return roleBind, roleBindKey, nil
}

func (m *OpenShiftManager) addFinalizer(
	ctx context.Context, o rtclient.Object) (bool, error) {
	// ---
	if controllerutil.ContainsFinalizer(o, openshiftFinalizer) {
		return false, nil
	}
	controllerutil.AddFinalizer(o, openshiftFinalizer)
	return true, m.client.Update(ctx, o)
}

func (m *OpenShiftManager) sccRoleNsname() types.NamespacedName {
	return types.NamespacedName{
		Namespace: m.cfg.PodNamespace,
		Name:      sccRoleName,
	}
}

func (m *OpenShiftManager) sccRoleBindingNsname() types.NamespacedName {
	return types.NamespacedName{
		Namespace: m.cfg.PodNamespace,
		Name:      sccRoleBindingName,
	}
}

func (m *OpenShiftManager) getOrCreateMetricsRole(
	ctx context.Context) (*rbacv1.Role, bool, error) {
	// ---
	roleCurr, roleKey, err := m.getMetricsRole(ctx)
	if err == nil {
		return roleCurr, false, err // OK
	}
	if !errors.IsNotFound(err) {
		m.logger.Error(err, "Failed to get Metrics Role", "key", roleKey)
		return nil, false, err
	}
	roleWant := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: roleKey.Namespace,
			Name:      roleKey.Name,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"services", "endpoints", "pods"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
	err = m.client.Create(ctx, roleWant, &rtclient.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			m.logger.Info("Metrics Role already exists", "key", roleKey)
		} else {
			m.logger.Error(err, "Failed to create Metrics Role",
				"key", roleKey)
		}
		return nil, false, err
	}
	return roleWant, true, nil
}

func (m *OpenShiftManager) getMetricsRole(
	ctx context.Context) (*rbacv1.Role, types.NamespacedName, error) {
	// ---
	roleKey := m.metricsRoleNsname()
	role := &rbacv1.Role{}
	err := m.client.Get(ctx, roleKey, role)
	if err != nil {
		return nil, roleKey, err
	}
	return role, roleKey, nil
}

func (m *OpenShiftManager) getOrCreateMetricsRoleBinding(
	ctx context.Context,
	role *rbacv1.Role) (*rbacv1.RoleBinding, bool, error) {
	// ---
	roleBindCurr, roleBindKey, err := m.getMetricsRoleBinding(ctx)
	if err == nil {
		return roleBindCurr, false, err // OK
	}
	if !errors.IsNotFound(err) {
		m.logger.Error(err, "Failed to get Metrics RoleBinding",
			"key", roleBindKey)
		return nil, false, err
	}
	roleBindWant := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: roleBindKey.Namespace,
			Name:      roleBindKey.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "prometheus-k8s",
				Namespace: "openshift-monitoring",
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "Role",
			Name: role.GetName(),
		},
	}
	err = m.client.Create(ctx, roleBindWant, &rtclient.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			m.logger.Info("Metrics RoleBinding already exists",
				"key", roleBindKey)
		} else {
			m.logger.Error(err, "Failed to create Metrics RoleBinding",
				"key", roleBindKey)
		}
		return nil, false, err
	}
	return roleBindWant, true, nil
}

func (m *OpenShiftManager) getMetricsRoleBinding(
	ctx context.Context) (
	*rbacv1.RoleBinding, types.NamespacedName, error) {
	// ---
	roleBindKey := m.metricsRoleBindingNsname()
	roleBind := &rbacv1.RoleBinding{}
	err := m.client.Get(ctx, roleBindKey, roleBind)
	if err != nil {
		return nil, roleBindKey, err
	}
	return roleBind, roleBindKey, nil
}

func (m *OpenShiftManager) metricsRoleNsname() types.NamespacedName {
	return types.NamespacedName{
		Namespace: m.cfg.PodNamespace,
		Name:      metricsRoleName,
	}
}

func (m *OpenShiftManager) metricsRoleBindingNsname() types.NamespacedName {
	return types.NamespacedName{
		Namespace: m.cfg.PodNamespace,
		Name:      metricsRoleBindingName,
	}
}

// IsOpenShiftCluster checks if operator is running on OpenShift cluster based
// on its self-pod annotations.
func IsOpenShiftCluster(ctx context.Context,
	reader rtclient.Reader,
	cfg *conf.OperatorConfig) bool {
	key := rtclient.ObjectKey{
		Namespace: cfg.PodNamespace,
		Name:      cfg.PodName,
	}
	clusterType, err := resolveClusterTypeByPod(ctx, reader, key)
	return (err == nil) && (clusterType == ClusterTypeOpenshift)
}

// resolveClusterTypeByPod finds the kind of K8s cluster via annotation of one
// of its running pods.
func resolveClusterTypeByPod(ctx context.Context,
	reader rtclient.Reader,
	podKey rtclient.ObjectKey) (ClusterType, error) {
	pod, err := getPod(ctx, reader, podKey)
	if err != nil {
		return ClusterTypeDefault, err
	}
	for key := range pod.Annotations {
		if strings.Contains(key, ClusterTypeOpenshift) {
			return ClusterTypeOpenshift, nil
		}
	}
	return ClusterTypeDefault, nil
}

func getPod(ctx context.Context,
	reader rtclient.Reader,
	podKey rtclient.ObjectKey) (*corev1.Pod, error) {
	pod := corev1.Pod{}
	err := reader.Get(ctx, podKey, &pod)
	return &pod, err
}

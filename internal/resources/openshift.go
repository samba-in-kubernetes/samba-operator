// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	rtclient "sigs.k8s.io/controller-runtime/pkg/client"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
	"github.com/samba-in-kubernetes/samba-operator/internal/conf"
)

const (
	sambaServiceAccountName     = "samba"
	sambaSccName                = "samba"
	sambaSccRoleName            = "samba-scc-role"
	sambaSccRoleBindingName     = "samba-scc-rolebinding"
	sambaMetricsRoleName        = "samba-metrics-role"
	sambaMetricsRoleBindingName = "samba-metrics-rolebinding"
)

var (
	namespaceLabels = map[string]string{
		"openshift.io/cluster-monitoring":                "true",
		"pod-security.kubernetes.io/enforce":             "privileged",
		"pod-security.kubernetes.io/audit":               "privileged",
		"pod-security.kubernetes.io/warn":                "privileged",
		"security.openshift.io/scc.podSecurityLabelSync": "false",
	}
)

// Process is called by the controller on reconciliation of
// SmbCommonConfig
func (m *SmbShareManager) updateForOpenshift(
	ctx context.Context,
	smbshare *sambaoperatorv1alpha1.SmbShare) Result {
	// Do-nothing if not on OpenShift
	m.updateClusterType(ctx)
	if m.cfg.ClusterType != conf.ClusterTypeOpenShift {
		return Done
	}

	requireAnnotationsOf(smbshare)
	m.logger.Info(
		"Updating state for SmbShare on OpenShift",
		"SmbShare.Namespace", smbshare.Namespace,
		"SmbShare.Name", smbshare.Name,
		"SmbShare.Annotations", smbshare.Annotations)

	ns, updated, err := m.getOrUpdateNamespaceOf(ctx, smbshare)
	if err != nil {
		return Result{err: err}
	}
	if updated {
		m.logger.Info("Updated Namespace labels",
			"Namespace.Name", ns.Name,
			"Namespace.Labels", ns.Labels)
		return Requeue
	}

	sa, created, err := m.getOrCreateServiceAccountOf(ctx, smbshare)
	if err != nil {
		return Result{err: err}
	}
	if created {
		m.logger.Info("Created ServiceAccount",
			"ServiceAccount.Namespace", sa.Namespace,
			"ServiceAccount.Name", sa.Name)
		return Requeue
	}

	sccRole, created, err := m.getOrCreateSCCRoleOf(ctx, smbshare)
	if err != nil {
		return Result{err: err}
	}
	if created {
		m.logger.Info("Created SCC Role",
			"SCCRole.Namespace", sccRole.Namespace,
			"SCCRole.Name", sccRole.Name)
		return Requeue
	}

	sccRoleBind, created, err := m.getOrCreateSCCRoleBindingOf(ctx, smbshare, sa, sccRole)
	if err != nil {
		return Result{err: err}
	}
	if created {
		m.logger.Info("Created SCC RoleBinding",
			"SCCRoleBinding.Namespace", sccRoleBind.Namespace,
			"SCCRoleBinding.Name", sccRoleBind.Name)
		return Requeue
	}

	metricsRole, created, err := m.getOrCreateMetricsRoleOf(ctx, smbshare)
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
		m.getOrCreateMetricsRoleBindingOf(ctx, smbshare, metricsRole)
	if err != nil {
		return Result{err: err}
	}
	if created {
		m.logger.Info("Created Metrics RoleBinding",
			"MetricsRoleBinding.Namespace", metricsRoleBind.Namespace,
			"MetricsRoleBinding.Name", metricsRoleBind.Name)
		return Requeue
	}

	m.logger.Info("Done updating SmbShare resources for OpenShift",
		"SmbShare.Namespace", smbshare.Namespace,
		"SmbShare.Name", smbshare.Name,
		"ServiceAccount.Name", sa.Name,
		"SCCRole.Namespace", sccRole.Namespace,
		"SCCRole.Name", sccRole.Name,
		"MetricsRoleBinding.Namespace", metricsRoleBind.Namespace,
		"MetricsRoleBinding.Name", metricsRoleBind.Name)

	return Done
}

func requireAnnotationsOf(smbshare *sambaoperatorv1alpha1.SmbShare) {
	if smbshare.Annotations == nil {
		smbshare.Annotations = map[string]string{}
	}
	smbshare.Annotations["openshift.io/scc"] = sambaSccName
}

// Cache cluster type
func (m *SmbShareManager) updateClusterType(ctx context.Context) {
	if m.cfg.ClusterType == "" {
		if IsOpenShiftCluster(ctx, m.client, m.cfg) {
			m.cfg.ClusterType = conf.ClusterTypeOpenShift
		} else {
			m.cfg.ClusterType = conf.ClusterTypeDefault
		}
	}
}

// Finalize should be called when there's a finalizer on the resource
// and we need to do some cleanup.
func (m *SmbShareManager) finalizeForOpenshift(
	ctx context.Context,
	smbshare *sambaoperatorv1alpha1.SmbShare) Result {
	// ---
	// Do-nothing if not on OpenShift
	m.updateClusterType(ctx)
	if m.cfg.ClusterType != conf.ClusterTypeOpenShift {
		return Done
	}
	m.logger.Info(
		"Finalize state for SmbShare on OpenShift",
		"SmbCommonConfig.Namespace", smbshare.GetNamespace(),
		"SmbCommonConfig.Name", smbshare.GetName())

	metricsRoleBind, metricsRoleBindKey, err := m.getMetricsRoleBindingOf(ctx, smbshare)
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

	metricsRole, metricsRoleKey, err := m.getMetricsRoleOf(ctx, smbshare)
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

	sccRoleBind, sccRoleBindKey, err := m.getSCCRoleBindingOf(ctx, smbshare)
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

	sccRole, sccRoleKey, err := m.getSCCRoleOf(ctx, smbshare)
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

	sa, saKey, err := m.getServiceAccountOf(ctx, smbshare)
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
	return Done
}

func (m *SmbShareManager) getOrUpdateNamespaceOf(
	ctx context.Context,
	smbshare *sambaoperatorv1alpha1.SmbShare) (*corev1.Namespace, bool, error) {
	// ---
	nsCurr, err := m.getNamespace(ctx, smbshare.GetNamespace())
	if err != nil {
		m.logger.Error(err, "Failed to get Namespace", "nsname", smbshare.GetNamespace())
		return nil, false, err
	}
	if hasNamespaceLabels(nsCurr) {
		return nsCurr, false, nil // OK
	}
	nsWant := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   smbshare.GetNamespace(),
			Labels: nsCurr.Labels,
		},
	}
	nsCurr.Spec.DeepCopyInto(&nsWant.Spec)
	addNamespaceLabels(nsWant)
	err = m.client.Update(ctx, nsWant)
	if err != nil {
		m.logger.Error(err, "Failed to update Namespace", "nsname", smbshare.GetNamespace())
		return nil, false, err
	}
	return nsWant, true, nil
}

func hasNamespaceLabels(ns *corev1.Namespace) bool {
	for labelName, labelVal := range namespaceLabels {
		curLabelVal, ok := ns.Labels[labelName]
		if !ok {
			return false
		}
		if curLabelVal != labelVal {
			return false
		}
	}
	return true
}

func addNamespaceLabels(ns *corev1.Namespace) {
	for labelName, labelVal := range namespaceLabels {
		ns.Labels[labelName] = labelVal
	}
}

func (m *SmbShareManager) getNamespace(
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

func (m *SmbShareManager) getOrCreateServiceAccountOf(
	ctx context.Context,
	smbshare *sambaoperatorv1alpha1.SmbShare) (
	*corev1.ServiceAccount, bool, error) {
	// ---
	saCurr, saKey, err := m.getServiceAccountOf(ctx, smbshare)
	if err == nil {
		return saCurr, false, nil // OK
	}
	if !errors.IsNotFound(err) {
		m.logger.Error(err, "Failed to get ServiceAccount", "key", saKey)
		return nil, false, err
	}
	automountServiceAccountToken := true
	saWant := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: saKey.Namespace,
			Name:      saKey.Name,
		},
		AutomountServiceAccountToken: &automountServiceAccountToken,
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

func (m *SmbShareManager) getServiceAccountOf(
	ctx context.Context,
	smbshare *sambaoperatorv1alpha1.SmbShare) (
	*corev1.ServiceAccount, types.NamespacedName, error) {
	// ---
	saKey := serviceAccountNsname(smbshare)
	sa := &corev1.ServiceAccount{}
	err := m.client.Get(ctx, saKey, sa)
	if err != nil {
		return nil, saKey, err
	}
	return sa, saKey, nil
}

func serviceAccountNsname(smbshare *sambaoperatorv1alpha1.SmbShare) types.NamespacedName {
	return types.NamespacedName{
		Namespace: smbshare.GetNamespace(),
		Name:      sambaServiceAccountName,
	}
}

func (m *SmbShareManager) getOrCreateSCCRoleOf(
	ctx context.Context,
	smbshare *sambaoperatorv1alpha1.SmbShare) (
	*rbacv1.Role, bool, error) {
	// ---
	roleCurr, roleKey, err := m.getSCCRoleOf(ctx, smbshare)
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

				APIGroups:     []string{"security.openshift.io"},
				Resources:     []string{"securitycontextconstraints"},
				ResourceNames: []string{sambaSccName},
				Verbs:         []string{"use"},
			},
			{

				APIGroups: []string{""},
				Resources: []string{"namespaces", "services", "pods"},
				Verbs:     []string{"get", "list", "watch"},
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

func (m *SmbShareManager) getSCCRoleOf(
	ctx context.Context,
	smbshare *sambaoperatorv1alpha1.SmbShare) (
	*rbacv1.Role, types.NamespacedName, error) {
	// ---
	roleKey := sccRoleNsnameOf(smbshare)
	role := &rbacv1.Role{}
	err := m.client.Get(ctx, roleKey, role)
	if err != nil {
		return nil, roleKey, err
	}
	return role, roleKey, nil
}

func (m *SmbShareManager) getOrCreateSCCRoleBindingOf(
	ctx context.Context,
	smbshare *sambaoperatorv1alpha1.SmbShare,
	sa *corev1.ServiceAccount,
	role *rbacv1.Role) (*rbacv1.RoleBinding, bool, error) {
	// ---
	roleBindCurr, roleBindKey, err := m.getSCCRoleBindingOf(ctx, smbshare)
	if err == nil {
		return roleBindCurr, false, err // OK
	}
	if !errors.IsNotFound(err) {
		m.logger.Error(err, "Failed to get RoleBinding", "key", roleBindKey)
		return nil, false, err
	}
	roleBindWant := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: smbshare.GetNamespace(),
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

func (m *SmbShareManager) getSCCRoleBindingOf(
	ctx context.Context,
	smbshare *sambaoperatorv1alpha1.SmbShare) (
	*rbacv1.RoleBinding, types.NamespacedName, error) {
	// ---
	roleBindKey := sccRoleBindingNsnameOf(smbshare)
	roleBind := &rbacv1.RoleBinding{}
	err := m.client.Get(ctx, roleBindKey, roleBind)
	if err != nil {
		return nil, roleBindKey, err
	}
	return roleBind, roleBindKey, nil
}

func sccRoleNsnameOf(smbshare *sambaoperatorv1alpha1.SmbShare) types.NamespacedName {
	return types.NamespacedName{
		Namespace: smbshare.GetNamespace(),
		Name:      sambaSccRoleName,
	}
}

func sccRoleBindingNsnameOf(smbshare *sambaoperatorv1alpha1.SmbShare) types.NamespacedName {
	return types.NamespacedName{
		Namespace: smbshare.GetNamespace(),
		Name:      sambaSccRoleBindingName,
	}
}

func (m *SmbShareManager) getOrCreateMetricsRoleOf(
	ctx context.Context,
	smbshare *sambaoperatorv1alpha1.SmbShare) (
	*rbacv1.Role, bool, error) {
	// ---
	roleCurr, roleKey, err := m.getMetricsRoleOf(ctx, smbshare)
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

func (m *SmbShareManager) getMetricsRoleOf(
	ctx context.Context,
	smbshare *sambaoperatorv1alpha1.SmbShare) (
	*rbacv1.Role, types.NamespacedName, error) {
	// ---
	roleKey := metricsRoleNsnameOf(smbshare)
	role := &rbacv1.Role{}
	err := m.client.Get(ctx, roleKey, role)
	if err != nil {
		return nil, roleKey, err
	}
	return role, roleKey, nil
}

func (m *SmbShareManager) getOrCreateMetricsRoleBindingOf(
	ctx context.Context,
	smbshare *sambaoperatorv1alpha1.SmbShare,
	role *rbacv1.Role) (
	*rbacv1.RoleBinding, bool, error) {
	// ---
	roleBindCurr, roleBindKey, err := m.getMetricsRoleBindingOf(ctx, smbshare)
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

func (m *SmbShareManager) getMetricsRoleBindingOf(
	ctx context.Context,
	smbshare *sambaoperatorv1alpha1.SmbShare) (
	*rbacv1.RoleBinding, types.NamespacedName, error) {
	// ---
	roleBindKey := metricsRoleBindingNsnameOf(smbshare)
	roleBind := &rbacv1.RoleBinding{}
	err := m.client.Get(ctx, roleBindKey, roleBind)
	if err != nil {
		return nil, roleBindKey, err
	}
	return roleBind, roleBindKey, nil
}

func metricsRoleNsnameOf(smbshare *sambaoperatorv1alpha1.SmbShare) types.NamespacedName {
	return types.NamespacedName{
		Namespace: smbshare.GetNamespace(),
		Name:      sambaMetricsRoleName,
	}
}

func metricsRoleBindingNsnameOf(smbshare *sambaoperatorv1alpha1.SmbShare) types.NamespacedName {
	return types.NamespacedName{
		Namespace: smbshare.GetNamespace(),
		Name:      sambaMetricsRoleBindingName,
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
	return (err == nil) && (clusterType == conf.ClusterTypeOpenShift)
}

// resolveClusterTypeByPod finds the kind of K8s cluster via annotation of one
// of its running pods.
func resolveClusterTypeByPod(ctx context.Context,
	reader rtclient.Reader,
	podKey rtclient.ObjectKey) (string, error) {
	pod, err := getPod(ctx, reader, podKey)
	if err != nil {
		return conf.ClusterTypeDefault, err
	}
	for key := range pod.Annotations {
		if strings.Contains(key, conf.ClusterTypeOpenShift) {
			return conf.ClusterTypeOpenShift, nil
		}
	}
	return conf.ClusterTypeDefault, nil
}

func getPod(ctx context.Context,
	reader rtclient.Reader,
	podKey rtclient.ObjectKey) (*corev1.Pod, error) {
	pod := corev1.Pod{}
	err := reader.Get(ctx, podKey, &pod)
	return &pod, err
}

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

const sccFinalizer = "samba-operator.samba.org/sccFinalizer"

const (
	// ClusterTypeUnknown defines the default value for unknown cluster type
	ClusterTypeUnknown = "unknown"
	// ClusterTypeOpenshift defines the type-name for OpenShift clusters
	ClusterTypeOpenshift = "openshift"
)

const (
	// DefaultServiceAccountName defines the default name used for ServiceAccount
	DefaultServiceAccountName = "samba"
	// DefaultRoleName defines the default name used for Role
	DefaultRoleName = "samba-anyuid"
	// DefaultRoleBindingName defines the default name used for RoleBinding
	DefaultRoleBindingName = "samba-anyuid-rolebinding"
)

// SccManager is used to manage OpenShift's SCC related resources: create
// ServiceAccount (if not exists) and enable Role and RoleBinding referencing
// to OpenShift's 'anyuid' SCC.
type SccManager struct {
	client rtclient.Client
	logger logr.Logger
	cfg    *conf.OperatorConfig
}

// NewSccManager creates a ServiceAccountManager instance
func NewSccManager(
	clnt rtclient.Client,
	log logr.Logger,
	cfg *conf.OperatorConfig) *SccManager {
	return &SccManager{
		client: clnt,
		logger: log,
		cfg:    cfg,
	}
}

// Process is called by the controller on any type of reconciliation.
func (m *SccManager) Process(
	ctx context.Context,
	nsname types.NamespacedName) Result {
	// Do-nothing if not on OpenShift
	if !m.withSCC(ctx) {
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
			"SmbShare.Namespace", nsname.Namespace,
			"SmbShare.Name", nsname.Name)
		return Result{err: err}
	}

	// now that we have the resource. determine if its live or pending deletion
	if cc.GetDeletionTimestamp() != nil {
		// its being deleted
		if controllerutil.ContainsFinalizer(cc, sccFinalizer) {
			// and our finalizer is present
			return m.Finalize(ctx, cc)
		}
		return Done
	}
	// resource is alive
	return m.Update(ctx, cc)
}

// Update should be called when a ServiceAccount resource changes.
// nolint:funlen
func (m *SccManager) Update(
	ctx context.Context,
	cc *sambaoperatorv1alpha1.SmbCommonConfig) Result {
	// ---
	m.logger.Info(
		"Updating state for SmbCommonConfig SCC",
		"SmbCommonConfig.Namespace", cc.Namespace,
		"SmbCommonConfig.Name", cc.Name,
		"SmbCommonConfig.UID", cc.UID)

	changed, err := m.addFinalizer(ctx, cc)
	if err != nil {
		return Result{err: err}
	}
	if changed {
		m.logger.Info("Added finalizer")
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

	role, created, err := m.getOrCreateRole(ctx)
	if err != nil {
		return Result{err: err}
	}
	if created {
		m.logger.Info("Created Role",
			"Role.Namespace", role.Namespace,
			"Role.Name", role.Name)
		return Requeue
	}

	rolebind, created, err := m.getOrCreateRoleBinding(ctx, sa, role)
	if err != nil {
		return Result{err: err}
	}
	if created {
		m.logger.Info("Created RoleBinding",
			"RoleBinding.Namespace", rolebind.Namespace,
			"RoleBinding.Name", rolebind.Name)
		return Requeue
	}

	m.logger.Info("Done updating SmbCommonConfig SCC resources",
		"ServiceAccount.Name", sa.Name,
		"Role.Namespace", role.Namespace,
		"Role.Name", role.Name,
		"RoleBinding.Namespace", rolebind.Namespace,
		"RoleBinding.Name", rolebind.Name)

	return Done
}

// Finalize should be called when there's a finalizer on the resource
// and we need to do some cleanup.
func (m *SccManager) Finalize(
	ctx context.Context,
	cc *sambaoperatorv1alpha1.SmbCommonConfig) Result {
	// ---
	m.logger.Info(
		"Finalize state for SmbCommonConfig SCC",
		"SmbCommonConfig.Namespace", cc.Namespace,
		"SmbCommonConfig.Name", cc.Name,
		"SmbCommonConfig.UID", cc.UID)

	roleBind, roleBindKey, err := m.getRoleBinding(ctx)
	if err == nil {
		err = m.client.Delete(ctx, roleBind, &rtclient.DeleteOptions{})
		if err != nil {
			m.logger.Error(err,
				"Failed to finalize RoleBinding", "key", roleBindKey)
			return Result{err: err}
		}
		m.logger.Info("Deleted RoleBinding", "key", roleBindKey)
		return Requeue
	} else if !errors.IsNotFound(err) {
		m.logger.Error(err, "Failed to get RoleBinding", "key", roleBindKey)
	}

	role, roleKey, err := m.getRole(ctx)
	if err == nil {
		err = m.client.Delete(ctx, role, &rtclient.DeleteOptions{})
		if err != nil {
			m.logger.Error(err,
				"Failed to finalize Role", "key", roleKey)
			return Result{err: err}
		}
		m.logger.Info("Deleted Role", "key", roleKey)
		return Requeue
	} else if !errors.IsNotFound(err) {
		m.logger.Error(err, "Failed to get Role", "key", roleKey)
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

	m.logger.Info("Removing finalizer")
	controllerutil.RemoveFinalizer(cc, sccFinalizer)
	err = m.client.Update(ctx, cc)
	if err != nil {
		return Result{err: err}
	}
	return Done
}

func (m *SccManager) getOrCreateServiceAccount(
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
		m.logger.Error(err, "Failed to create ServiceAccount", "key", saKey)
		return nil, false, err
	}
	return saWant, true, nil
}

func (m *SccManager) getServiceAccount(
	ctx context.Context) (*corev1.ServiceAccount, types.NamespacedName, error) {
	// ---
	saKey := m.serviceAccountNsname()
	sa := &corev1.ServiceAccount{}
	err := m.client.Get(ctx, saKey, sa)
	if err != nil {
		return nil, saKey, err
	}
	return sa, saKey, err
}

func (m *SccManager) getOrCreateRole(
	ctx context.Context) (*rbacv1.Role, bool, error) {
	// ---
	roleCurr, roleKey, err := m.getRole(ctx)
	if err == nil {
		return roleCurr, false, err // OK
	}
	if !errors.IsNotFound(err) {
		m.logger.Error(err, "Failed to get Role", "key", roleKey)
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
				ResourceNames: []string{"anyuid"},
				Verbs:         []string{"use"},
			},
		},
	}
	err = m.client.Create(ctx, roleWant, &rtclient.CreateOptions{})
	if err != nil {
		m.logger.Error(err, "Failed to create Role", "key", roleKey)
		return nil, false, err
	}
	return roleWant, true, nil
}

func (m *SccManager) getRole(
	ctx context.Context) (*rbacv1.Role, types.NamespacedName, error) {
	// ---
	roleKey := m.roleNsname()
	role := &rbacv1.Role{}
	err := m.client.Get(ctx, roleKey, role)
	if err != nil {
		return nil, roleKey, err
	}
	return role, roleKey, nil
}

func (m *SccManager) getOrCreateRoleBinding(
	ctx context.Context,
	sa *corev1.ServiceAccount,
	role *rbacv1.Role) (*rbacv1.RoleBinding, bool, error) {
	// ---
	roleBindCurr, roleBindKey, err := m.getRoleBinding(ctx)
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
		m.logger.Error(err, "Failed to create RoleBinding",
			"key", roleBindKey)
		return nil, false, err
	}
	return roleBindWant, true, nil
}

func (m *SccManager) getRoleBinding(
	ctx context.Context) (*rbacv1.RoleBinding, types.NamespacedName, error) {
	// ---
	roleBindKey := m.roleBindingNsname()
	roleBind := &rbacv1.RoleBinding{}
	err := m.client.Get(ctx, roleBindKey, roleBind)
	if err != nil {
		return nil, roleBindKey, err
	}
	return roleBind, roleBindKey, nil
}

func (m *SccManager) addFinalizer(
	ctx context.Context,
	cc *sambaoperatorv1alpha1.SmbCommonConfig) (bool, error) {
	// ---
	if controllerutil.ContainsFinalizer(cc, sccFinalizer) {
		return false, nil
	}
	controllerutil.AddFinalizer(cc, sccFinalizer)
	return true, m.client.Update(ctx, cc)
}

func (m *SccManager) withSCC(ctx context.Context) bool {
	clusterType, err := m.resolveClusterType(ctx)
	return (err == nil) && (clusterType == ClusterTypeOpenshift)
}

func (m *SccManager) serviceAccountNsname() types.NamespacedName {
	if len(m.cfg.ServiceAccountName) > 0 {
		return types.NamespacedName{Name: m.cfg.ServiceAccountName}
	}
	return types.NamespacedName{
		Namespace: m.cfg.PodNamespace,
		Name:      DefaultServiceAccountName,
	}
}

func (m *SccManager) roleNsname() types.NamespacedName {
	return types.NamespacedName{
		Namespace: m.cfg.PodNamespace,
		Name:      DefaultRoleName,
	}
}

func (m *SccManager) roleBindingNsname() types.NamespacedName {
	return types.NamespacedName{
		Namespace: m.cfg.PodNamespace,
		Name:      DefaultRoleBindingName,
	}
}

// resolveClusterType finds the kind of k8s cluster on which this pod run
func (m *SccManager) resolveClusterType(
	ctx context.Context) (ClusterType, error) {
	pod, err := m.getSelfPod(ctx)
	if err != nil {
		m.logger.Error(err, "Failed to get self pod")
		return ClusterTypeUnknown, err
	}
	for key := range pod.Annotations {
		if strings.Contains(key, ClusterTypeOpenshift) {
			return ClusterTypeOpenshift, nil
		}
	}
	return ClusterTypeUnknown, nil
}

func (m *SccManager) getSelfPod(ctx context.Context) (*corev1.Pod, error) {
	key := types.NamespacedName{
		Namespace: m.cfg.PodNamespace,
		Name:      m.cfg.PodName,
	}
	pod := corev1.Pod{}
	err := m.client.Get(ctx, key, &pod)
	return &pod, err
}

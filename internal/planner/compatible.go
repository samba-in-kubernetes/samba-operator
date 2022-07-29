// SPDX-License-Identifier: Apache-2.0

package planner

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
)

// IncompatibleInstanceError indicates SmbShare resources are not compatible
// with each other and can not hosted by the same server.
type IncompatibleInstanceError struct {
	current  string
	existing string
	reason   string
}

// Error interface method.
func (iie IncompatibleInstanceError) Error() string {
	return fmt.Sprintf("Share resource %s is incompatible with %s: %s",
		iie.current,
		iie.existing,
		iie.reason)
}

func incompatible(
	current, existing InstanceConfiguration,
	reason string) IncompatibleInstanceError {
	// ---
	return IncompatibleInstanceError{
		current:  current.SmbShare.Name,
		existing: existing.SmbShare.Name,
		reason:   reason,
	}
}

// CheckCompatible returns an error if the instance configurations are
// not compatible with each other. A compatible instance is one that
// can share the same smbd.
func CheckCompatible(current, existing InstanceConfiguration) error {
	// This returns an error rather than a boolean because in the future
	// we can choose to add details about what is incompatible to
	// the specific error type.
	if current.SmbShare == nil || existing.SmbShare == nil {
		name1, name2 := "<invalid>", "<invalid>"
		if current.SmbShare != nil {
			name1 = current.SmbShare.Name
		}
		if existing.SmbShare != nil {
			name2 = existing.SmbShare.Name
		}
		return IncompatibleInstanceError{
			current:  name1,
			existing: name2,
			reason:   "instance configuration missing SmbShare",
		}
	}

	if current.SmbShare.Namespace != existing.SmbShare.Namespace {
		return incompatible(current, existing, "namespaces differ")
	}
	if current.SmbShare.Spec.Storage.Pvc.Name != existing.SmbShare.Spec.Storage.Pvc.Name {
		return incompatible(current, existing,
			"PersistentVolumeClaim name mismatch")
	}
	if current.SmbShare.Spec.SecurityConfig != existing.SmbShare.Spec.SecurityConfig {
		return incompatible(current, existing,
			"security config name mismatch")
	}
	if current.SmbShare.Spec.CommonConfig != existing.SmbShare.Spec.CommonConfig {
		return incompatible(current, existing,
			"common config name mismatch")
	}

	// additional checks
	var uid1, uid2 types.UID
	if current.SecurityConfig != nil {
		uid1 = current.SecurityConfig.UID
	}
	if existing.SecurityConfig != nil {
		uid2 = existing.SecurityConfig.UID
	}
	if uid1 != uid2 {
		return incompatible(current, existing,
			"security configuration resources differ")
	}

	uid1, uid2 = "", ""
	if current.CommonConfig != nil {
		uid1 = current.CommonConfig.UID
	}
	if existing.CommonConfig != nil {
		uid2 = existing.CommonConfig.UID
	}
	if uid1 != uid2 {
		return incompatible(current, existing,
			"common configuration resources differ")
	}

	return nil
}

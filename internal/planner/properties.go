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

package planner

import (
	"strings"
)

// The core properties of an instance are:
// * security mode
// * Domain (AD)
// * WORKGROUP (ADish)
// * isClustered
// * clusterSize
// * pvc (storage)
// * networking

// SecurityMode describes the high level user-authentication
// used by a share or instance.
type SecurityMode string

const (
	// UserMode means users and groups are locally configured
	UserMode = SecurityMode("user")
	// ADMode means users and groups are sourced from an Active Directory
	// domain.
	ADMode = SecurityMode("active-directory")
)

// InstanceName returns the instance's name.
func (sp *Planner) InstanceName() string {
	// for now, its the name of the Server Group
	return sp.SmbShare.Status.ServerGroup
}

// SecurityMode returns the high level security mode to be used.
func (sp *Planner) SecurityMode() SecurityMode {
	if sp.SecurityConfig == nil {
		return UserMode
	}
	m := SecurityMode(sp.SecurityConfig.Spec.Mode)
	if m != UserMode && m != ADMode {
		// this shouldn't normally be possible unless kube validation
		// fails or is out of sync.
		m = UserMode
	}
	return m
}

// Realm returns the name of the realm (domain).
func (sp *Planner) Realm() string {
	return strings.ToUpper(sp.SecurityConfig.Spec.Realm)
}

// Workgroup returns the name of the workgroup. This may be automatically
// derived from the realm.
func (sp *Planner) Workgroup() string {
	// todo: this is a big hack. needs thought and cleanup
	parts := strings.SplitN(sp.Realm(), ".", 2)
	return parts[0]
}

// IsClustered returns true if the instance is configured for clustering.
func (sp *Planner) IsClustered() bool {
	if sp.SmbShare.Spec.Scaling == nil {
		return false
	}
	return sp.SmbShare.Spec.Scaling.AvailabilityMode == "clustered"
}

// ClusterSize returns the (minimum) size of the cluster.
func (sp *Planner) ClusterSize() int32 {
	if sp.SmbShare.Spec.Scaling == nil {
		return 1
	}
	return int32(sp.SmbShare.Spec.Scaling.MinClusterSize)
}

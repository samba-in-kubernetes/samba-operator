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

// GroupMode describes how/if SmbShares are allowed to be grouped
// together in on server instance
type GroupMode string

const (
	// GroupModeNever disallows grouping shares. this is the default
	GroupModeNever = GroupMode("never")
	// GroupModeExplicit enables grouping with specific group naming
	GroupModeExplicit = GroupMode("explicit")
	// GroupModeUnset is equivalent to groupModeNever
	GroupModeUnset = GroupMode("")
)

// InstanceName returns the instance's name.
func (pl *Planner) InstanceName() string {
	// for now, its the name of the Server Group
	return pl.SmbShare.Status.ServerGroup
}

// SecurityMode returns the high level security mode to be used.
func (pl *Planner) SecurityMode() SecurityMode {
	if pl.SecurityConfig == nil {
		return UserMode
	}
	m := SecurityMode(pl.SecurityConfig.Spec.Mode)
	if m != UserMode && m != ADMode {
		// this shouldn't normally be possible unless kube validation
		// fails or is out of sync.
		m = UserMode
	}
	return m
}

// Realm returns the name of the realm (domain).
func (pl *Planner) Realm() string {
	return strings.ToUpper(pl.SecurityConfig.Spec.Realm)
}

// Workgroup returns the name of the workgroup. This may be automatically
// derived from the realm.
func (pl *Planner) Workgroup() string {
	// todo: this is a big hack. needs thought and cleanup
	parts := strings.SplitN(pl.Realm(), ".", 2)
	return parts[0]
}

// IsClustered returns true if the instance is configured for clustering.
func (pl *Planner) IsClustered() bool {
	if pl.SmbShare.Spec.Scaling == nil {
		return false
	}
	return pl.SmbShare.Spec.Scaling.AvailabilityMode == "clustered"
}

// ClusterSize returns the (minimum) size of the cluster.
func (pl *Planner) ClusterSize() int32 {
	if pl.SmbShare.Spec.Scaling == nil {
		return 1
	}
	return int32(pl.SmbShare.Spec.Scaling.MinClusterSize)
}

// Grouping returns the logical grouping mode and group name.
func (pl *Planner) Grouping() (GroupMode, string) {
	mode := GroupModeUnset
	groupName := ""
	if pl.SmbShare.Spec.Scaling != nil {
		mode = GroupMode(pl.SmbShare.Spec.Scaling.GroupMode)
		groupName = pl.SmbShare.Spec.Scaling.Group
	}
	// sanitize the SmbShare values. only a valid group type
	// will return a real name.
	switch mode {
	case GroupModeExplicit:
		return mode, groupName
	// silly linter makes me fill in all the cases
	case GroupModeNever, GroupModeUnset:
		fallthrough
	default:
		return GroupModeNever, ""
	}
}

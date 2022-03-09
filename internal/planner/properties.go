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

type securityMode string

const (
	userMode = securityMode("user")
	adMode   = securityMode("active-directory")
)

func (sp *Planner) instanceName() string {
	// for now, its the name of the Server Group
	return sp.SmbShare.Status.ServerGroup
}

func (sp *Planner) securityMode() securityMode {
	if sp.SecurityConfig == nil {
		return userMode
	}
	m := securityMode(sp.SecurityConfig.Spec.Mode)
	if m != userMode && m != adMode {
		// this shouldn't normally be possible unless kube validation
		// fails or is out of sync.
		m = userMode
	}
	return m
}

func (sp *Planner) realm() string {
	return strings.ToUpper(sp.SecurityConfig.Spec.Realm)
}

func (sp *Planner) workgroup() string {
	// todo: this is a big hack. needs thought and cleanup
	parts := strings.SplitN(sp.realm(), ".", 2)
	return parts[0]
}

func (sp *Planner) isClustered() bool {
	if sp.SmbShare.Spec.Scaling == nil {
		return false
	}
	return sp.SmbShare.Spec.Scaling.AvailabilityMode == "clustered"
}

func (sp *Planner) clusterSize() int32 {
	if sp.SmbShare.Spec.Scaling == nil {
		return 1
	}
	return int32(sp.SmbShare.Spec.Scaling.MinClusterSize)
}

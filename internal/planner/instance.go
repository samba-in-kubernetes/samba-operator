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
	api "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
	"github.com/samba-in-kubernetes/samba-operator/internal/conf"
	"github.com/samba-in-kubernetes/samba-operator/internal/smbcc"
)

// InstanceConfiguration bundles together the various inputs that define
// the configuration of a server group instance.
type InstanceConfiguration struct {
	SmbShare       *api.SmbShare
	SecurityConfig *api.SmbSecurityConfig
	CommonConfig   *api.SmbCommonConfig
	GlobalConfig   *conf.OperatorConfig
}

// Planner arranges the state of the instance to be.
type Planner struct {
	// InstanceConfiguration is used as the configuration "intent".
	// The planner treats it as read only.
	InstanceConfiguration

	// ConfigState covers the shared configuration state that is mapped
	// into all containers/pods and generally interpreted by sambacc
	// for managing the behavior of samba containers. The planner treats
	// it as read/write.
	ConfigState *smbcc.SambaContainerConfig
}

// New instance of a planner based on the configuration CRs as well
// an existing configuration state (if it existed).
func New(
	ic InstanceConfiguration,
	state *smbcc.SambaContainerConfig) *Planner {
	// return a new Planner
	return &Planner{
		InstanceConfiguration: ic,
		ConfigState:           state,
	}
}

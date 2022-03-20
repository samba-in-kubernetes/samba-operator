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

// "cheat codes"
const (
	nodeSpreadKey     = "samba-operator.samba.org/node-spread"
	nodeSpreadDisable = "false"
)

// UserSecuritySource describes the location of user security configuration
// metadata.
type UserSecuritySource struct {
	Configured bool
	Namespace  string
	Secret     string
	Key        string
}

// UserSecuritySource returns the UserSecuritySource type for the
// particular instance.
func (pl *Planner) UserSecuritySource() UserSecuritySource {
	s := UserSecuritySource{}
	if pl.SecurityMode() != UserMode {
		return s
	}
	if pl.SecurityConfig == nil || pl.SecurityConfig.Spec.Users == nil {
		return s
	}
	s.Configured = true
	s.Namespace = pl.SecurityConfig.Namespace
	s.Secret = pl.SecurityConfig.Spec.Users.Secret
	s.Key = pl.SecurityConfig.Spec.Users.Key
	return s
}

// DNSRegister describes how an instance should register itself with
// a DNS system (typically AD).
type DNSRegister string

const (
	// DNSRegisterNever means the system should never register itself.
	DNSRegisterNever = DNSRegister("never")
	// DNSRegisterExternalIP means the system should register its
	// external IP address.
	DNSRegisterExternalIP = DNSRegister("external-ip")
	// DNSRegisterClusterIP means the system should register its
	// in-cluster IP address.
	DNSRegisterClusterIP = DNSRegister("cluster-ip")
)

// DNSRegister returns a DNSRegister type for this instance.
func (pl *Planner) DNSRegister() DNSRegister {
	reg := DNSRegisterNever
	if pl.SecurityMode() == ADMode && pl.SecurityConfig.Spec.DNS != nil {
		reg = DNSRegister(pl.SecurityConfig.Spec.DNS.Register)
	}
	switch reg {
	// allowed values
	case DNSRegisterExternalIP, DNSRegisterClusterIP:
	// anything else is reverted to "never"
	case DNSRegisterNever:
		fallthrough
	default:
		reg = DNSRegisterNever
	}
	return reg
}

// ServiceType returns the value that should be used for a Service fronting
// the SMB port for this instance.
func (pl *Planner) ServiceType() string {
	if pl.CommonConfig != nil && pl.CommonConfig.Spec.Network.Publish == "external" {
		return "LoadBalancer"
	}
	return "ClusterIP"
}

// SambaContainerDebugLevel returns a string that can be passed to Samba
// tools for debugging.
func (pl *Planner) SambaContainerDebugLevel() string {
	return pl.GlobalConfig.SambaDebugLevel
}

// MayCluster returns true if the operator is permitted to create clustered
// instances.
func (pl *Planner) MayCluster() bool {
	return pl.GlobalConfig.ClusterSupport == "ctdb-is-experimental"
}

// NodeSpread returns true if pods are required to be spread over multiple
// nodes.
func (pl *Planner) NodeSpread() bool {
	return pl.SmbShare.Annotations[nodeSpreadKey] != nodeSpreadDisable
}

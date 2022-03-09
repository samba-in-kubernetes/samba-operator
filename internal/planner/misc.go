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

type userSecuritySource struct {
	Configured bool
	Namespace  string
	Secret     string
	Key        string
}

func (sp *Planner) userSecuritySource() userSecuritySource {
	s := userSecuritySource{}
	if sp.securityMode() != userMode {
		return s
	}
	if sp.SecurityConfig == nil || sp.SecurityConfig.Spec.Users == nil {
		return s
	}
	s.Configured = true
	s.Namespace = sp.SecurityConfig.Namespace
	s.Secret = sp.SecurityConfig.Spec.Users.Secret
	s.Key = sp.SecurityConfig.Spec.Users.Key
	return s
}

type dnsRegister string

const (
	dnsRegisterNever      = dnsRegister("never")
	dnsRegisterExternalIP = dnsRegister("external-ip")
	dnsRegisterClusterIP  = dnsRegister("cluster-ip")
)

func (sp *Planner) dnsRegister() dnsRegister {
	reg := dnsRegisterNever
	if sp.securityMode() == adMode && sp.SecurityConfig.Spec.DNS != nil {
		reg = dnsRegister(sp.SecurityConfig.Spec.DNS.Register)
	}
	switch reg {
	// allowed values
	case dnsRegisterExternalIP, dnsRegisterClusterIP:
	// anything else is reverted to "never"
	case dnsRegisterNever:
		fallthrough
	default:
		reg = dnsRegisterNever
	}
	return reg
}

func (sp *Planner) serviceType() string {
	if sp.CommonConfig != nil && sp.CommonConfig.Spec.Network.Publish == "external" {
		return "LoadBalancer"
	}
	return "ClusterIP"
}

func (sp *Planner) sambaContainerDebugLevel() string {
	return sp.GlobalConfig.SambaDebugLevel
}

func (sp *Planner) mayCluster() bool {
	return sp.GlobalConfig.ClusterSupport == "ctdb-is-experimental"
}

// nodeSpread returns true if pods are required to be spread over multiple
// nodes.
func (sp *Planner) nodeSpread() bool {
	return sp.SmbShare.Annotations[nodeSpreadKey] != nodeSpreadDisable
}

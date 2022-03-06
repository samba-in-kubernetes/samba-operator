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
	"fmt"
	"strings"

	api "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
	"github.com/samba-in-kubernetes/samba-operator/internal/conf"
	"github.com/samba-in-kubernetes/samba-operator/internal/smbcc"
)

type securityMode string

const (
	userMode = securityMode("user")
	adMode   = securityMode("active-directory")
)

type dnsRegister string

const (
	dnsRegisterNever      = dnsRegister("never")
	dnsRegisterExternalIP = dnsRegister("external-ip")
	dnsRegisterClusterIP  = dnsRegister("cluster-ip")
)

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

func (sp *Planner) instanceName() string {
	// for now, its the name of the Server Group
	return sp.SmbShare.Status.ServerGroup
}

func (sp *Planner) instanceID() smbcc.Key {
	return smbcc.Key(sp.instanceName())
}

func (sp *Planner) shareName() string {
	// todo: make sure this is smb-conf clean, otherwise we need to
	// fix up the name value(s).
	if sp.SmbShare.Spec.ShareName != "" {
		return sp.SmbShare.Spec.ShareName
	}
	// It was not named explicitly. Name it after the CR.
	// todo: may need massaging too.
	return sp.SmbShare.Name
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

func (*Planner) joinJSONSuffix(index int) string {
	return fmt.Sprintf("-%d", index)
}

func (*Planner) joinEnvPaths(p []string) string {
	return strings.Join(p, ":")
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

func (sp *Planner) idmapOptions() smbcc.SmbOptions {
	if sp.SecurityConfig == nil || len(sp.SecurityConfig.Spec.Domains) == 0 {
		// default idmap config
		return smbcc.SmbOptions{
			"idmap config * : backend": "autorid",
			"idmap config * : range":   "2000-9999999",
		}
	}
	// this is hacky and needs both config to support user supplied ID
	// ranges as well as a decent algo to deal with ID map ranges.
	// for now we're just punting though (call it prototyping) :-)
	o := smbcc.SmbOptions{}
	doms := []api.SmbSecurityDomainSpec{}
	userDefault := false
	for _, d := range sp.SecurityConfig.Spec.Domains {
		doms = append(doms, d)
		if d.Name == "*" {
			userDefault = true
		}
	}
	if !userDefault {
		doms = append(doms, api.SmbSecurityDomainSpec{
			Name:    "*",
			Backend: "autorid",
		})
	}
	step := 10000
	for i, d := range doms {
		pfx := fmt.Sprintf("idmap config %s : ", d.Name)
		if d.Backend == "autorid" {
			o[pfx+"backend"] = "autorid"
		} else {
			o[pfx+"backend"] = "ad"
			o[pfx+"schema_mode"] = "rfc2307"
		}
		rs := (i * step) + 2000
		o[pfx+"range"] = fmt.Sprintf("%d-%d", rs, rs+step-1)
	}
	return o
}

func (sp *Planner) update() (changed bool, err error) {
	globals, found := sp.ConfigState.Globals[smbcc.Globals]
	if !found {
		globalOptions := smbcc.NewGlobalOptions()
		globalOptions.SmbPort = sp.GlobalConfig.SmbdPort
		globals = smbcc.NewGlobals(globalOptions)
		sp.ConfigState.Globals[smbcc.Globals] = globals
		changed = true
	}
	shareKey := smbcc.Key(sp.shareName())
	share, found := sp.ConfigState.Shares[shareKey]
	if !found {
		share = smbcc.NewSimpleShare(sp.sharePath())
		if !sp.SmbShare.Spec.Browseable {
			share.Options[smbcc.BrowseableParam] = smbcc.No
		}
		if sp.SmbShare.Spec.ReadOnly {
			share.Options[smbcc.ReadOnlyParam] = smbcc.Yes
		}
		sp.ConfigState.Shares[shareKey] = share
		changed = true
	}
	cfgKey := sp.instanceID()
	cfg, found := sp.ConfigState.Configs[cfgKey]
	if !found || cfg.Shares[0] != shareKey {
		cfg = smbcc.ConfigSection{
			Shares:       []smbcc.Key{shareKey},
			Globals:      []smbcc.Key{smbcc.Globals},
			InstanceName: sp.instanceName(),
		}
		if sp.securityMode() == adMode {
			realmKey := smbcc.Key(sp.realm())
			cfg.Globals = append(cfg.Globals, realmKey)
		}
		if sp.isClustered() {
			cfg.InstanceFeatures = []smbcc.FeatureFlag{smbcc.CTDB}
		}
		sp.ConfigState.Configs[cfgKey] = cfg
		changed = true
	}
	if len(sp.ConfigState.Users) == 0 {
		sp.ConfigState.Users = smbcc.NewDefaultUsers()
		changed = true
	}
	if sp.securityMode() == adMode {
		realmKey := smbcc.Key(sp.realm())
		_, found := sp.ConfigState.Globals[realmKey]
		if !found {
			opts := sp.idmapOptions()
			// security mode
			opts["security"] = "ads"
			// workgroup and realm
			opts["workgroup"] = sp.workgroup()
			opts["realm"] = sp.realm()
			sp.ConfigState.Globals[realmKey] = smbcc.GlobalConfig{
				Options: opts,
			}
			changed = true
		}
	}
	return
}

// nolint:unused
func (sp *Planner) prune() (changed bool, err error) {
	cfgKey := sp.instanceID()
	if _, found := sp.ConfigState.Configs[cfgKey]; found {
		delete(sp.ConfigState.Configs, cfgKey)
		changed = true
	}
	shareKey := smbcc.Key(sp.shareName())
	if _, found := sp.ConfigState.Shares[shareKey]; found {
		delete(sp.ConfigState.Shares, shareKey)
		changed = true
	}
	return
}

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

// nodeSpread returns true if pods are required to be spread over multiple
// nodes.
func (sp *Planner) nodeSpread() bool {
	return sp.SmbShare.Annotations[nodeSpreadKey] != nodeSpreadDisable
}

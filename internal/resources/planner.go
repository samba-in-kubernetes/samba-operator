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

package resources

import (
	"fmt"
	"path"
	"strings"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
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

type userSecuritySource struct {
	Configured bool
	Namespace  string
	Secret     string
	Key        string
}

// InstanceConfiguration bundles together the various inputs that define
// the configuration of a server group instance.
type InstanceConfiguration struct {
	SmbShare       *sambaoperatorv1alpha1.SmbShare
	SecurityConfig *sambaoperatorv1alpha1.SmbSecurityConfig
	CommonConfig   *sambaoperatorv1alpha1.SmbCommonConfig
	GlobalConfig   *conf.OperatorConfig
}

type sharePlanner struct {
	// InstanceConfiguration is used as the configuration "intent".
	// The planner treats it as read only.
	InstanceConfiguration

	// ConfigState covers the shared configuration state that is mapped
	// into all containers/pods and generally interpreted by sambacc
	// for managing the behavior of samba containers. The planner treats
	// it as read/write.
	ConfigState *smbcc.SambaContainerConfig
}

func newSharePlanner(
	ic InstanceConfiguration,
	state *smbcc.SambaContainerConfig) *sharePlanner {
	// return a new sharePlanner
	return &sharePlanner{
		InstanceConfiguration: ic,
		ConfigState:           state,
	}
}

func (sp *sharePlanner) instanceName() string {
	// for now, its the name of the Server Group
	return sp.SmbShare.Status.ServerGroup
}

func (sp *sharePlanner) instanceID() smbcc.Key {
	return smbcc.Key(sp.instanceName())
}

func (sp *sharePlanner) shareName() string {
	// todo: make sure this is smb-conf clean, otherwise we need to
	// fix up the name value(s).
	if sp.SmbShare.Spec.ShareName != "" {
		return sp.SmbShare.Spec.ShareName
	}
	// It was not named explicitly. Name it after the CR.
	// todo: may need massaging too.
	return sp.SmbShare.Name
}

func (sp *sharePlanner) sharePath() string {
	return path.Join("/mnt", string(sp.SmbShare.UID))
}

func (sp *sharePlanner) containerConfigPath() string {
	cpath := path.Join(sp.containerConfigDir(), "config.json")
	if sp.userSecuritySource().Configured {
		upath := path.Join(sp.usersConfigDir(), sp.usersConfigFileName())
		cpath += ":" + upath
	}
	return cpath
}

func (*sharePlanner) containerConfigDir() string {
	return "/etc/container-config"
}

func (*sharePlanner) usersConfigFileName() string {
	return "users.json"
}

func (*sharePlanner) usersConfigDir() string {
	return "/etc/container-users"
}

func (*sharePlanner) winbindSocketsDir() string {
	return "/run/samba/winbindd"
}

func (*sharePlanner) sambaStateDir() string {
	return "/var/lib/samba"
}

func (*sharePlanner) osRunDir() string {
	return "/run"
}

func (sp *sharePlanner) securityMode() securityMode {
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

func (sp *sharePlanner) realm() string {
	return strings.ToUpper(sp.SecurityConfig.Spec.Realm)
}

func (sp *sharePlanner) workgroup() string {
	// todo: this is a big hack. needs thought and cleanup
	parts := strings.SplitN(sp.realm(), ".", 2)
	return parts[0]
}

func (*sharePlanner) joinJSONSuffix(index int) string {
	return fmt.Sprintf("-%d", index)
}

func (*sharePlanner) joinJSONSourceDir(index int) string {
	return fmt.Sprintf("/var/tmp/join/%d", index)
}

func (*sharePlanner) joinJSONFileName() string {
	return "join.json"
}

func (sp *sharePlanner) joinJSONSourcePath(index int) string {
	return path.Join(sp.joinJSONSourceDir(index), sp.joinJSONFileName())
}

func (*sharePlanner) joinEnvPaths(p []string) string {
	return strings.Join(p, ":")
}

func (sp *sharePlanner) userSecuritySource() userSecuritySource {
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

func (sp *sharePlanner) idmapOptions() smbcc.SmbOptions {
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
	doms := []sambaoperatorv1alpha1.SmbSecurityDomainSpec{}
	userDefault := false
	for _, d := range sp.SecurityConfig.Spec.Domains {
		doms = append(doms, d)
		if d.Name == "*" {
			userDefault = true
		}
	}
	if !userDefault {
		doms = append(doms, sambaoperatorv1alpha1.SmbSecurityDomainSpec{
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

func (sp *sharePlanner) update() (changed bool, err error) {
	noprinting, found := sp.ConfigState.Globals[smbcc.NoPrintingKey]
	if !found {
		noprinting = smbcc.NewNoPrintingGlobals()
		sp.ConfigState.Globals[smbcc.NoPrintingKey] = noprinting
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
			Globals:      []smbcc.Key{smbcc.NoPrintingKey},
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

func (sp *sharePlanner) prune() (changed bool, err error) {
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

func (sp *sharePlanner) dnsRegister() dnsRegister {
	reg := dnsRegisterNever
	if sp.securityMode() == adMode && sp.SecurityConfig.Spec.DNS != nil {
		reg = dnsRegister(sp.SecurityConfig.Spec.DNS.Register)
	}
	switch reg {
	// allowed values
	case dnsRegisterExternalIP, dnsRegisterClusterIP:
	// anything else is reverted to "never"
	default:
		reg = dnsRegisterNever
	}
	return reg
}

func (*sharePlanner) serviceWatchStateDir() string {
	return "/var/lib/svcwatch"
}

func (sp *sharePlanner) serviceWatchJSONPath() string {
	return path.Join(sp.serviceWatchStateDir(), "status.json")
}

func (sp *sharePlanner) initializerArgs(cmd string) []string {
	args := []string{}
	if sp.isClustered() {
		// if this is a ctdb enabled setup, this "initializer"
		// container-command will be skipped if certain things have already
		// been "initialized"
		args = append(args, "--skip-if-file=/var/lib/ctdb/shared/nodes")
	}
	args = append(args, cmd)
	return args
}

func (sp *sharePlanner) dnsRegisterArgs() []string {
	args := []string{
		"dns-register",
		"--watch",
	}
	if sp.dnsRegister() == dnsRegisterClusterIP {
		args = append(args, "--target=internal")
	}
	args = append(args, sp.serviceWatchJSONPath())
	return args
}

func (sp *sharePlanner) runDaemonArgs(name string) []string {
	args := []string{"run", name}
	if sp.isClustered() {
		if sp.securityMode() == adMode {
			args = append(args, "--setup=nsswitch", "--setup=smb_ctdb")
		} else if name == "smbd" {
			args = append(args, "--setup=users", "--setup=smb_ctdb")
		}
	}
	return args
}

func (*sharePlanner) ctdbDaemonArgs() []string {
	return []string{
		"run",
		"ctdbd",
		"--setup=smb_ctdb",
		"--setup=ctdb_config",
		"--setup=ctdb_etc",
		"--setup=ctdb_nodes",
	}
}

func (*sharePlanner) ctdbManageNodesArgs() []string {
	return []string{
		"ctdb-manage-nodes",
		"--hostname=$(HOSTNAME)",
		"--take-node-number-from-hostname=after-last-dash",
	}
}

func (*sharePlanner) ctdbMigrateArgs() []string {
	return []string{
		"ctdb-migrate",
		"--dest-dir=/var/lib/ctdb/persistent",
	}
}

func (*sharePlanner) ctdbSetNodeArgs() []string {
	return []string{
		"ctdb-set-node",
		"--hostname=$(HOSTNAME)",
		"--take-node-number-from-hostname=after-last-dash",
	}
}

func (*sharePlanner) ctdbMustHaveNodeArgs() []string {
	return []string{
		"ctdb-must-have-node",
		"--hostname=$(HOSTNAME)",
		"--take-node-number-from-hostname=after-last-dash",
	}
}

func (sp *sharePlanner) serviceType() string {
	if sp.CommonConfig != nil && sp.CommonConfig.Spec.Network.Publish == "external" {
		return "LoadBalancer"
	}
	return "ClusterIP"
}

func (sp *sharePlanner) sambaContainerDebugLevel() string {
	return sp.GlobalConfig.SambaDebugLevel
}

func (sp *sharePlanner) isClustered() bool {
	if sp.SmbShare.Spec.Scaling == nil {
		return false
	}
	return sp.SmbShare.Spec.Scaling.AvailbilityMode == "clustered"
}

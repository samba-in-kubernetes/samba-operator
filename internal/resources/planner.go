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
	"github.com/samba-in-kubernetes/samba-operator/internal/smbcc"
)

type securityMode string

const (
	userMode = securityMode("user")
	adMode   = securityMode("active-directory")
)

type userSecuritySource struct {
	Configured bool
	Namespace  string
	Secret     string
	Key        string
}

type sharePlanner struct {
	SmbShare       *sambaoperatorv1alpha1.SmbShare
	SecurityConfig *sambaoperatorv1alpha1.SmbSecurityConfig
	Config         *smbcc.SambaContainerConfig
}

func newSharePlanner(
	share *sambaoperatorv1alpha1.SmbShare,
	security *sambaoperatorv1alpha1.SmbSecurityConfig,
	config *smbcc.SambaContainerConfig) *sharePlanner {
	// return a new sharePlanner
	return &sharePlanner{
		SmbShare:       share,
		SecurityConfig: security,
		Config:         config,
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

func (*sharePlanner) joinJsonSuffix(index int) string {
	return fmt.Sprintf("-%d", index)
}

func (*sharePlanner) joinJsonSourceDir(index int) string {
	return fmt.Sprintf("/var/tmp/join/%d", index)
}

func (*sharePlanner) joinJsonFileName() string {
	return "join.json"
}

func (sp *sharePlanner) joinJsonSourcePath(index int) string {
	return path.Join(sp.joinJsonSourceDir(index), sp.joinJsonFileName())
}

func (*sharePlanner) joinEnvPaths(p []string) string {
	return strings.Join(p, ":")
}

func (sp *sharePlanner) userSecuritySource() userSecuritySource {
	s := userSecuritySource{}
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
	noprinting, found := sp.Config.Globals[smbcc.NoPrintingKey]
	if !found {
		noprinting = smbcc.NewNoPrintingGlobals()
		sp.Config.Globals[smbcc.NoPrintingKey] = noprinting
		changed = true
	}
	shareKey := smbcc.Key(sp.shareName())
	share, found := sp.Config.Shares[shareKey]
	if !found {
		share = smbcc.NewSimpleShare(sp.sharePath())
		if !sp.SmbShare.Spec.Browseable {
			share.Options[smbcc.BrowseableParam] = smbcc.No
		}
		if sp.SmbShare.Spec.ReadOnly {
			share.Options[smbcc.ReadOnlyParam] = smbcc.Yes
		}
		sp.Config.Shares[shareKey] = share
		changed = true
	}
	cfgKey := sp.instanceID()
	cfg, found := sp.Config.Configs[cfgKey]
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
		sp.Config.Configs[cfgKey] = cfg
		changed = true
	}
	if len(sp.Config.Users) == 0 {
		sp.Config.Users = smbcc.NewDefaultUsers()
		changed = true
	}
	if sp.securityMode() == adMode {
		realmKey := smbcc.Key(sp.realm())
		_, found := sp.Config.Globals[realmKey]
		if !found {
			opts := sp.idmapOptions()
			// security mode
			opts["security"] = "ads"
			// workgroup and realm
			opts["workgroup"] = sp.workgroup()
			opts["realm"] = sp.realm()
			sp.Config.Globals[realmKey] = smbcc.GlobalConfig{
				Options: opts,
			}
			changed = true
		}
	}
	return
}

func (sp *sharePlanner) prune() (changed bool, err error) {
	cfgKey := sp.instanceID()
	if _, found := sp.Config.Configs[cfgKey]; found {
		delete(sp.Config.Configs, cfgKey)
		changed = true
	}
	shareKey := smbcc.Key(sp.shareName())
	if _, found := sp.Config.Shares[shareKey]; found {
		delete(sp.Config.Shares, shareKey)
		changed = true
	}
	return
}

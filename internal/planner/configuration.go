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

	api "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
	"github.com/samba-in-kubernetes/samba-operator/internal/smbcc"
)

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

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

func (pl *Planner) instanceID() smbcc.Key {
	return smbcc.Key(pl.InstanceName())
}

func (pl *Planner) shareName() string {
	// todo: make sure this is smb-conf clean, otherwise we need to
	// fix up the name value(s).
	if pl.SmbShare.Spec.ShareName != "" {
		return pl.SmbShare.Spec.ShareName
	}
	// It was not named explicitly. Name it after the CR.
	// todo: may need massaging too.
	return pl.SmbShare.Name
}

func (pl *Planner) idmapOptions() smbcc.SmbOptions {
	if pl.SecurityConfig == nil || len(pl.SecurityConfig.Spec.Domains) == 0 {
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
	for _, d := range pl.SecurityConfig.Spec.Domains {
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

// Update the held configuration based on the state of the instance
// configuration.
func (pl *Planner) Update() (changed bool, err error) {
	_, found := pl.ConfigState.Globals[smbcc.Globals]
	if !found {
		globalOptions := smbcc.NewGlobalOptions()
		globalOptions.SmbPort = pl.GlobalConfig.SmbdPort
		globals := smbcc.NewGlobals(globalOptions)
		pl.ConfigState.Globals[smbcc.Globals] = globals
		changed = true
	}
	shareKey := smbcc.Key(pl.shareName())
	share, found := pl.ConfigState.Shares[shareKey]
	if !found {
		share = smbcc.NewSimpleShare(pl.Paths().Share())
		pl.ConfigState.Shares[shareKey] = share
		changed = true
	}
	if pl.CommonConfig != nil {
		if c := applyCustomGlobal(pl.ConfigState.Globals[smbcc.Globals], pl.CommonConfig.Spec); c {
			changed = true
		}
	}
	if c := applyCustomShare(share, pl.SmbShare.Spec); c {
		changed = true
	}
	if c := applyShareValues(share, pl.SmbShare.Spec); c {
		changed = true
	}
	cfgKey := pl.instanceID()
	cfg, found := pl.ConfigState.Configs[cfgKey]
	if !found {
		cfg = smbcc.ConfigSection{
			Shares:       []smbcc.Key{shareKey},
			Globals:      []smbcc.Key{smbcc.Globals},
			InstanceName: pl.InstanceName(),
			Permissions:  smbcc.NewPermissionsConfig(),
		}
		if pl.SecurityMode() == ADMode {
			realmKey := smbcc.Key(pl.Realm())
			cfg.Globals = append(cfg.Globals, realmKey)
		}
		if pl.IsClustered() {
			cfg.InstanceFeatures = []smbcc.FeatureFlag{smbcc.CTDB}
		}
		changed = true
	}
	if !hasShare(&cfg, shareKey) {
		cfg.Shares = append(cfg.Shares, shareKey)
		changed = true
	}
	if changed {
		pl.ConfigState.Configs[cfgKey] = cfg
	}
	if len(pl.ConfigState.Users) == 0 {
		pl.ConfigState.Users = smbcc.NewDefaultUsers()
		changed = true
	}
	if pl.SecurityMode() == ADMode {
		realmKey := smbcc.Key(pl.Realm())
		_, found := pl.ConfigState.Globals[realmKey]
		if !found {
			opts := pl.idmapOptions()
			// security mode
			opts["security"] = "ads"
			// workgroup and realm
			opts["workgroup"] = pl.Workgroup()
			opts["realm"] = pl.Realm()
			pl.ConfigState.Globals[realmKey] = smbcc.GlobalConfig{
				Options: opts,
			}
			changed = true
		}
	}
	return
}

// Prune the target share from the configuration.
func (pl *Planner) Prune() (changed bool, err error) {
	cfgKey := pl.instanceID()
	shareKey := smbcc.Key(pl.shareName())

	if cfg, found := pl.ConfigState.Configs[cfgKey]; found {
		if removeShare(&cfg, shareKey) {
			pl.ConfigState.Configs[cfgKey] = cfg
			changed = true
		}
	}
	if _, found := pl.ConfigState.Shares[shareKey]; found {
		delete(pl.ConfigState.Shares, shareKey)
		changed = true
	}
	return
}

func applyCustomGlobal(globals smbcc.GlobalConfig, spec api.SmbCommonConfigSpec) bool {
	changed := false
	if spec.CustomGlobalConfig != nil {
		for k, v := range spec.CustomGlobalConfig.Configs {
			oriValue, ok := globals.Options[k]
			if !ok || (ok && oriValue != v) {
				globals.Options[k] = v
				changed = true
			}
		}
	}
	return changed
}

func applyCustomShare(share smbcc.ShareConfig, spec api.SmbShareSpec) bool {
	changed := false
	if spec.CustomShareConfig != nil {
		for k, v := range spec.CustomShareConfig.Configs {
			oriValue, ok := share.Options[k]
			if !ok || (ok && oriValue != v) {
				share.Options[k] = v
				changed = true
			}
		}
	}
	return changed
}

func applyShareValues(share smbcc.ShareConfig, spec api.SmbShareSpec) bool {
	changed := false

	setBrowsable := smbcc.Yes
	if !spec.Browseable {
		setBrowsable = smbcc.No
	}

	setReadOnly := smbcc.No
	if spec.ReadOnly {
		setReadOnly = smbcc.Yes
	}

	if share.Options[smbcc.BrowseableParam] != setBrowsable {
		share.Options[smbcc.BrowseableParam] = setBrowsable
		changed = true
	}

	if share.Options[smbcc.ReadOnlyParam] != setReadOnly {
		share.Options[smbcc.ReadOnlyParam] = setReadOnly
		changed = true
	}

	return changed
}

func hasShare(cfg *smbcc.ConfigSection, k smbcc.Key) bool {
	for i := range cfg.Shares {
		if cfg.Shares[i] == k {
			return true
		}
	}
	return false
}

func removeShare(cfg *smbcc.ConfigSection, k smbcc.Key) bool {
	for i := range cfg.Shares {
		if cfg.Shares[i] == k {
			cfg.Shares = append(cfg.Shares[:i], cfg.Shares[i+1:]...)
			return true
		}
	}
	return false
}

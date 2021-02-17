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
	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
	"github.com/samba-in-kubernetes/samba-operator/internal/smbcc"
)

type sharePlanner struct {
	SmbShare *sambaoperatorv1alpha1.SmbShare
	Config   *smbcc.SambaContainerConfig
}

func newSharePlanner(
	share *sambaoperatorv1alpha1.SmbShare,
	config *smbcc.SambaContainerConfig) *sharePlanner {
	// return a new sharePlanner
	return &sharePlanner{
		SmbShare: share,
		Config:   config,
	}
}

func (sp *sharePlanner) instanceName() string {
	// for now, its just the name of the k8s resource
	return sp.SmbShare.Name
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
	return "share"
}

func (*sharePlanner) sharePath() string {
	// for now, everything mounts at /share
	return "/share"
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
		sp.Config.Configs[cfgKey] = cfg
		changed = true
	}
	if len(sp.Config.Users) == 0 {
		sp.Config.Users = smbcc.NewDefaultUsers()
		changed = true
	}
	return
}

// SPDX-License-Identifier: Apache-2.0

package planner

import (
	"encoding/json"
)

// builtinDefaultNodeSelector defines our defaults if no administrative
// overrides are supplied. Samba containers run only on linux. Currently
// they're only built for x86_64 (amd64).
var builtinDefaultNodeSelector = map[string]string{
	"kubernetes.io/os":   "linux",
	"kubernetes.io/arch": "amd64",
}

// NodeSelector returns a mapping of k8s label names to values that are
// used to select what nodes resources are scheduled to run on.
func (pl *Planner) NodeSelector() (map[string]string, error) {
	// do we have node selector values specified in the smb common config?
	if pl.CommonConfig != nil && pl.CommonConfig.Spec.PodSettings != nil {
		ps := pl.CommonConfig.Spec.PodSettings
		if len(ps.NodeSelector) > 0 {
			return ps.NodeSelector, nil
		}
	}
	// get node selector from operator configuration
	if pl.GlobalConfig.DefaultNodeSelector == "" {
		return builtinDefaultNodeSelector, nil
	}
	var nsel map[string]string
	err := json.Unmarshal([]byte(pl.GlobalConfig.DefaultNodeSelector), &nsel)
	if err != nil {
		return builtinDefaultNodeSelector, err
	}
	return nsel, nil
}

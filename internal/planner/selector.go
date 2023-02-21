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

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

package smbcc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

const json1 = `
{
  "samba-container-config": "v0",
  "configs": {
    "wbtest": {
      "shares": [
        "share"
      ],
      "globals": [
        "noprinting",
        "wbtest"
      ],
      "instance_name": "WB1"
    }
  },
  "shares": {
    "share": {
      "options": {
        "path": "/share",
        "read only": "no"
      }
    }
  },
  "_NOTE": "Change the security and workgroup keys to match your domain.",
  "globals": {
    "noprinting": {
      "options": {
        "load printers": "no",
        "printing": "bsd",
        "printcap name": "/dev/null",
        "disable spoolss": "yes"
      }
    },
    "wbtest": {
      "options": {
        "log level": "10",
        "security": "ads",
        "workgroup": "FOOBAR",
        "realm": "FOOBAR.EXAMPLE.ORG",
        "server min protocol": "SMB2",
        "idmap config * : backend": "autorid",
        "idmap config * : range": "2000-9999999"
      }
    }
  }
}
`

func TestUnmarshal(t *testing.T) {
	scc := New()
	err := json.Unmarshal([]byte(json1), scc)
	require.NoError(t, err)
	require.Equal(t, version0, scc.SCCVersion)
	require.Len(t, scc.Configs, 1)
	require.Contains(t, scc.Configs, Key("wbtest"))
}

func TestMarshal(t *testing.T) {
	scc := New()
	globalsKey := Key("globals")
	shareKey := Key("share")
	wbtestKey := Key("wbtest")
	scc.Globals[globalsKey] = NewGlobals()
	scc.Globals[wbtestKey] = GlobalConfig{
		Options: SmbOptions{
			"log level":                "10",
			"security":                 "ads",
			"workgroup":                "FOOBAR",
			"realm":                    "FOOBAR.EXAMPLE.ORG",
			"server min protocol":      "SMB2",
			"idmap config * : backend": "autorid",
			"idmap config * : range":   "2000-9999999",
		},
	}
	scc.Shares[shareKey] = NewSimpleShare("/path")
	cfg := NewConfigSection("WB1")
	cfg.Shares = []Key{shareKey}
	cfg.Globals = []Key{globalsKey, wbtestKey}
	scc.Configs[wbtestKey] = cfg

	b, err := json.Marshal(scc)
	require.NoError(t, err)
	scc2 := New()
	err = json.Unmarshal(b, scc2)
	require.NoError(t, err)
	require.Equal(t, scc, scc2)
}

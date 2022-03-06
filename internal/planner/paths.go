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
	"path"
)

func (sp *Planner) sharePath() string {
	return path.Join("/mnt", string(sp.SmbShare.UID))
}

func (sp *Planner) containerConfigPath() string {
	cpath := path.Join(sp.containerConfigDir(), "config.json")
	if sp.userSecuritySource().Configured {
		upath := path.Join(sp.usersConfigDir(), sp.usersConfigFileName())
		cpath += ":" + upath
	}
	return cpath
}

func (*Planner) containerConfigDir() string {
	return "/etc/container-config"
}

func (*Planner) usersConfigFileName() string {
	return "users.json"
}

func (*Planner) usersConfigDir() string {
	return "/etc/container-users"
}

func (*Planner) winbindSocketsDir() string {
	return "/run/samba/winbindd"
}

func (*Planner) sambaStateDir() string {
	return "/var/lib/samba"
}

func (*Planner) osRunDir() string {
	return "/run"
}

func (*Planner) joinJSONSourceDir(index int) string {
	return fmt.Sprintf("/var/tmp/join/%d", index)
}

func (*Planner) joinJSONFileName() string {
	return "join.json"
}

func (sp *Planner) joinJSONSourcePath(index int) string {
	return path.Join(sp.joinJSONSourceDir(index), sp.joinJSONFileName())
}

func (*Planner) serviceWatchStateDir() string {
	return "/var/lib/svcwatch"
}

func (sp *Planner) serviceWatchJSONPath() string {
	return path.Join(sp.serviceWatchStateDir(), "status.json")
}

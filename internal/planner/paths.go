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

// Paths for relevant files and dirs within the containers.
type Paths struct {
	planner *Planner
}

// Paths returns an instance of the Paths type for our intended configuration.
func (planner *Planner) Paths() *Paths {
	return &Paths{planner}
}

// ShareMountPath returns the mount path.
func (p *Paths) ShareMountPath() string {
	// retain the previous approach to using UID for compatibility with
	// older versions ONLY if grouping is disabled.
	gmode, gname := p.planner.Grouping()
	if gmode == GroupModeNever {
		return path.Join("/mnt", string(p.planner.SmbShare.UID))
	}
	return path.Join("/mnt", gname)
}

// Share path.
func (p *Paths) Share() string {
	sharepath := p.planner.SmbShare.Spec.Storage.Pvc.Path
	if sharepath != "" {
		return path.Join(p.ShareMountPath(), "/", sharepath)
	}
	return p.ShareMountPath()
}

// ContainerConfigs returns a slice containing all configuration
// files that the samba-container should process.
func (p *Paths) ContainerConfigs() []string {
	paths := []string{p.containerConfig()}
	if uc := p.usersConfig(); uc != "" {
		paths = append(paths, uc)
	}
	return paths
}

func (p *Paths) containerConfig() string {
	return path.Join(p.ContainerConfigDir(), "config.json")
}

func (p *Paths) usersConfig() string {
	if !p.planner.UserSecuritySource().Configured {
		return ""
	}
	return path.Join(p.UsersConfigDir(), p.UsersConfigBaseName())
}

// ContainerConfigDir absolute path.
func (*Paths) ContainerConfigDir() string {
	return "/etc/container-config"
}

// UsersConfigBaseName file name component.
func (*Paths) UsersConfigBaseName() string {
	return "users.json"
}

// UsersConfigDir absolute path.
func (*Paths) UsersConfigDir() string {
	return "/etc/container-users"
}

// WinbindSocketsDir absolute path.
func (*Paths) WinbindSocketsDir() string {
	return "/run/samba/winbindd"
}

// SambaStateDir absolute path.
func (*Paths) SambaStateDir() string {
	return "/var/lib/samba"
}

// OSRunDir absolute path.
func (*Paths) OSRunDir() string {
	return "/run"
}

// JoinJSONSourceDir absolute path based on the given index value.
func (*Paths) JoinJSONSourceDir(index int) string {
	return fmt.Sprintf("/var/tmp/join/%d", index)
}

// JoinJSONBaseName file name component.
func (*Paths) JoinJSONBaseName() string {
	return "join.json"
}

// JoinJSONSource absolute path based on the given index value.
func (p *Paths) JoinJSONSource(index int) string {
	return path.Join(p.JoinJSONSourceDir(index), p.JoinJSONBaseName())
}

// ServiceWatchStateDir absolute path.
func (*Paths) ServiceWatchStateDir() string {
	return "/var/lib/svcwatch"
}

// ServiceWatchJSON file absolute path.
func (p *Paths) ServiceWatchJSON() string {
	return path.Join(p.ServiceWatchStateDir(), "status.json")
}

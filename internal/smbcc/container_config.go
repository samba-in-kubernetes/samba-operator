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

import "strconv"

// Key values are used to select subsections in the container config.
type Key string

// FeatureFlag values are used to select top level features that
// sambacc will apply when setting up a container.
type FeatureFlag string

const (
	// CTDB feature flag indicates the system should be configured with CTDB.
	CTDB FeatureFlag = "ctdb"

	// DefaultSmbPort is the default port which containerized smbd binds to.
	DefaultSmbPort int = 445
)

// GlobalOptions is used to pass options to modify the samba configuration
type GlobalOptions struct {
	// AddVFSFileid is used to check if we add vfs_fileid to the smb config
	AddVFSFileid bool
	// SmbPort is used as value to 'smb ports' config
	SmbPort int
}

// SambaContainerConfig holds one or more configuration for samba
// containers.
type SambaContainerConfig struct {
	SCCVersion string                `json:"samba-container-config"`
	Configs    map[Key]ConfigSection `json:"configs,omitempty"`
	Shares     map[Key]ShareConfig   `json:"shares,omitempty"`
	Globals    map[Key]GlobalConfig  `json:"globals,omitempty"`
	Users      map[Key]UserEntries   `json:"users,omitempty"`
	Groups     map[Key]GroupEntries  `json:"groups,omitempty"`
}

// ConfigSection identifies the shares, globals, and instance name of
// a single configuration.
type ConfigSection struct {
	Shares           []Key             `json:"shares,omitempty"`
	Globals          []Key             `json:"globals,omitempty"`
	InstanceName     string            `json:"instance_name,omitempty"`
	InstanceFeatures []FeatureFlag     `json:"instance_features,omitempty"`
	Permissions      PermissionsConfig `json:"permissions,omitempty"`
}

// ShareConfig holds configuration values for one share.
type ShareConfig struct {
	Options SmbOptions `json:"options,omitempty"`
}

// GlobalConfig holds configuration values for samba server globals.
type GlobalConfig struct {
	Options SmbOptions `json:"options,omitempty"`
}

// UserEntry represents a single "local" user for share access.
type UserEntry struct {
	Name     string `json:"name"`
	Uid      uint   `json:"uid,omitempty"`
	Gid      uint   `json:"gid,omitempty"`
	NTHash   string `json:"nt_hash,omitempty"`
	Password string `json:"password,omitempty"`
}

// UserEntries is a slice of UserEntry values.
type UserEntries []UserEntry

// GroupEntry represents a single "local" group for share access.
type GroupEntry struct {
	Name string `json:"name"`
	Gid  uint   `json:"gid,omitempty"`
}

// GroupEntries is a slice of GroupEntry values.
type GroupEntries []GroupEntry

// SmbOptions is a common type for storing smb.conf parameters.
type SmbOptions map[string]string

// PermissionsConfig indicates the permissions to be set on the share mountpoint
type PermissionsConfig struct {
	Method      string `json:"method,omitempty"`
	StatusXAttr string `json:"status_xattr,omitempty"`
	Mode        string `json:"mode,omitempty"`
}

const version0 = "v0"

const (
	// Globals is used for the default globals subsection.
	Globals = Key("globals")
	// AllEntriesKey is used for the standard "all_entries" default key for
	// users and groups.
	AllEntriesKey = Key("all_entries")

	// BrowseableParam controls if a share is browseable.
	BrowseableParam = "browseable"
	// ReadOnlyParam controls if a share is read only.
	ReadOnlyParam = "read only"

	// Yes means yes.
	Yes = "yes"
	// No means no.
	No = "no"
)

// NewGlobalOptions is the constructor for struct SambaConfigOptions
func NewGlobalOptions() GlobalOptions {
	return GlobalOptions{
		AddVFSFileid: true,
		SmbPort:      DefaultSmbPort,
	}
}

// New returns a new samba container config.
func New() *SambaContainerConfig {
	return &SambaContainerConfig{
		SCCVersion: version0,
		Configs:    map[Key]ConfigSection{},
		Shares:     map[Key]ShareConfig{},
		Globals:    map[Key]GlobalConfig{},
	}
}

// NewGlobals returns a default GlobalConfig.
func NewGlobals(opts GlobalOptions) GlobalConfig {
	cfg := GlobalConfig{
		Options: SmbOptions{
			"load printers":   No,
			"printing":        "bsd",
			"printcap name":   "/dev/null",
			"disable spoolss": Yes,
			"smb ports":       strconv.Itoa(opts.SmbPort),
		},
	}

	if opts.AddVFSFileid {
		_, found := cfg.Options["vfs objects"]
		if found {
			cfg.Options["vfs objects"] += " fileid"
		} else {
			cfg.Options["vfs objects"] = "fileid"
		}
		cfg.Options["fileid:algorithm"] = "fsid"
	}

	return cfg
}

// NewSimpleShare returns a ShareConfig with a simple configuration.
func NewSimpleShare(path string) ShareConfig {
	return ShareConfig{
		Options: SmbOptions{
			"path":      path,
			"read only": No,
		},
	}
}

// NewConfigSection returns a new ConfigSection.
func NewConfigSection(name string) ConfigSection {
	return ConfigSection{
		Shares:       []Key{},
		Globals:      []Key{},
		InstanceName: name,
	}
}

// NewPermissionsConfig returns a new PermissionsConfig.
func NewPermissionsConfig() PermissionsConfig {
	return PermissionsConfig{
		Method:      "initialize-share-perms",
		StatusXAttr: "user.share-perms-status",
		Mode:        "0777",
	}
}

// NewDefaultUsers returns a full subsection for a default (good for testing)
// set of users.
func NewDefaultUsers() map[Key]UserEntries {
	return map[Key]UserEntries{
		AllEntriesKey: {{
			Name:     "sambauser",
			Password: "samba",
		}},
	}
}

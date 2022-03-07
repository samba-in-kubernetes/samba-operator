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

// SambaContainerArgs generates sets for arguments for samba-container
// instances.
type SambaContainerArgs struct {
	planner *Planner
}

// Args returns a SambaContainerArgs type for starting containers in
// this instance.
func (p *Planner) Args() *SambaContainerArgs {
	return &SambaContainerArgs{p}
}

// Initializer container arguments generator.
func (s *SambaContainerArgs) Initializer(cmd string) []string {
	args := []string{}
	if s.planner.IsClustered() {
		// if this is a ctdb enabled setup, this "initializer"
		// container-command will be skipped if certain things have already
		// been "initialized"
		args = append(args, "--skip-if-file=/var/lib/ctdb/shared/nodes")
	}
	args = append(args, cmd)
	return args
}

// DNSRegister container arguments generator.
func (s *SambaContainerArgs) DNSRegister() []string {
	args := []string{
		"dns-register",
		"--watch",
	}
	if s.planner.DNSRegister() == DNSRegisterClusterIP {
		args = append(args, "--target=internal")
	}
	args = append(args, s.planner.Paths().ServiceWatchJSON())
	return args
}

// Run container arguments generator.
func (s *SambaContainerArgs) Run(name string) []string {
	args := []string{"run", name}
	if s.planner.IsClustered() {
		if s.planner.SecurityMode() == ADMode {
			args = append(args, "--setup=nsswitch", "--setup=smb_ctdb")
		} else if name == "smbd" {
			args = append(args, "--setup=users", "--setup=smb_ctdb")
		}
	}
	return args
}

// CTDBDaemon container arguments generator.
func (*SambaContainerArgs) CTDBDaemon() []string {
	return []string{
		"run",
		"ctdbd",
		"--setup=smb_ctdb",
		"--setup=ctdb_config",
		"--setup=ctdb_etc",
		"--setup=ctdb_nodes",
	}
}

// CTDBManageNodes container arguments generator.
func (*SambaContainerArgs) CTDBManageNodes() []string {
	return []string{
		"ctdb-manage-nodes",
		"--hostname=$(HOSTNAME)",
		"--take-node-number-from-hostname=after-last-dash",
	}
}

// CTDBMigrate container arguments generator.
func (*SambaContainerArgs) CTDBMigrate() []string {
	return []string{
		"ctdb-migrate",
		"--dest-dir=/var/lib/ctdb/persistent",
	}
}

// CTDBSetNode container arguments generator.
func (*SambaContainerArgs) CTDBSetNode() []string {
	return []string{
		"ctdb-set-node",
		"--hostname=$(HOSTNAME)",
		"--take-node-number-from-hostname=after-last-dash",
	}
}

// CTDBMustHaveNode container arguments generator.
func (*SambaContainerArgs) CTDBMustHaveNode() []string {
	return []string{
		"ctdb-must-have-node",
		"--hostname=$(HOSTNAME)",
		"--take-node-number-from-hostname=after-last-dash",
	}
}

// CTDBNodeStatus container arguments generator.
func (*SambaContainerArgs) CTDBNodeStatus() []string {
	return []string{
		"samba-container",
		"check",
		"ctdb-nodestatus",
	}
}

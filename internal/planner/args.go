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

func (sp *Planner) initializerArgs(cmd string) []string {
	args := []string{}
	if sp.IsClustered() {
		// if this is a ctdb enabled setup, this "initializer"
		// container-command will be skipped if certain things have already
		// been "initialized"
		args = append(args, "--skip-if-file=/var/lib/ctdb/shared/nodes")
	}
	args = append(args, cmd)
	return args
}

func (sp *Planner) dnsRegisterArgs() []string {
	args := []string{
		"dns-register",
		"--watch",
	}
	if sp.dnsRegister() == dnsRegisterClusterIP {
		args = append(args, "--target=internal")
	}
	args = append(args, sp.serviceWatchJSONPath())
	return args
}

func (sp *Planner) runDaemonArgs(name string) []string {
	args := []string{"run", name}
	if sp.IsClustered() {
		if sp.SecurityMode() == ADMode {
			args = append(args, "--setup=nsswitch", "--setup=smb_ctdb")
		} else if name == "smbd" {
			args = append(args, "--setup=users", "--setup=smb_ctdb")
		}
	}
	return args
}

func (*Planner) ctdbDaemonArgs() []string {
	return []string{
		"run",
		"ctdbd",
		"--setup=smb_ctdb",
		"--setup=ctdb_config",
		"--setup=ctdb_etc",
		"--setup=ctdb_nodes",
	}
}

func (*Planner) ctdbManageNodesArgs() []string {
	return []string{
		"ctdb-manage-nodes",
		"--hostname=$(HOSTNAME)",
		"--take-node-number-from-hostname=after-last-dash",
	}
}

func (*Planner) ctdbMigrateArgs() []string {
	return []string{
		"ctdb-migrate",
		"--dest-dir=/var/lib/ctdb/persistent",
	}
}

func (*Planner) ctdbSetNodeArgs() []string {
	return []string{
		"ctdb-set-node",
		"--hostname=$(HOSTNAME)",
		"--take-node-number-from-hostname=after-last-dash",
	}
}

func (*Planner) ctdbMustHaveNodeArgs() []string {
	return []string{
		"ctdb-must-have-node",
		"--hostname=$(HOSTNAME)",
		"--take-node-number-from-hostname=after-last-dash",
	}
}

func (*Planner) ctdbReadinessProbeArgs() []string {
	return []string{
		"samba-container",
		"check",
		"ctdb-nodestatus",
	}
}

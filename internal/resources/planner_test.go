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
	"testing"

	"github.com/stretchr/testify/assert"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
	"github.com/samba-in-kubernetes/samba-operator/internal/smbcc"
)

func TestPlannerDNSRegister(t *testing.T) {
	var (
		d       dnsRegister
		planner *sharePlanner
	)

	// no dns register specified
	planner = newSharePlanner(
		&sambaoperatorv1alpha1.SmbShare{},
		&sambaoperatorv1alpha1.SmbSecurityConfig{
			Spec: sambaoperatorv1alpha1.SmbSecurityConfigSpec{
				Mode: "active-directory",
			}},
		&sambaoperatorv1alpha1.SmbCommonConfig{},
		&smbcc.SambaContainerConfig{})
	d = planner.dnsRegister()
	assert.Equal(t, dnsRegisterNever, d)

	// external-ip
	planner = newSharePlanner(
		&sambaoperatorv1alpha1.SmbShare{},
		&sambaoperatorv1alpha1.SmbSecurityConfig{
			Spec: sambaoperatorv1alpha1.SmbSecurityConfigSpec{
				Mode: "active-directory",
				DNS: &sambaoperatorv1alpha1.SmbSecurityDNSSpec{
					Register: "external-ip",
				},
			}},
		&sambaoperatorv1alpha1.SmbCommonConfig{},
		&smbcc.SambaContainerConfig{})
	d = planner.dnsRegister()
	assert.Equal(t, dnsRegisterExternalIP, d)

	// cluster-ip
	planner = newSharePlanner(
		&sambaoperatorv1alpha1.SmbShare{},
		&sambaoperatorv1alpha1.SmbSecurityConfig{
			Spec: sambaoperatorv1alpha1.SmbSecurityConfigSpec{
				Mode: "active-directory",
				DNS: &sambaoperatorv1alpha1.SmbSecurityDNSSpec{
					Register: "cluster-ip",
				},
			}},
		&sambaoperatorv1alpha1.SmbCommonConfig{},
		&smbcc.SambaContainerConfig{})
	d = planner.dnsRegister()
	assert.Equal(t, dnsRegisterClusterIP, d)

	// invalid string for register
	planner = newSharePlanner(
		&sambaoperatorv1alpha1.SmbShare{},
		&sambaoperatorv1alpha1.SmbSecurityConfig{
			Spec: sambaoperatorv1alpha1.SmbSecurityConfigSpec{
				Mode: "active-directory",
				DNS: &sambaoperatorv1alpha1.SmbSecurityDNSSpec{
					Register: "junk",
				},
			}},
		&sambaoperatorv1alpha1.SmbCommonConfig{},
		&smbcc.SambaContainerConfig{})
	d = planner.dnsRegister()
	assert.Equal(t, dnsRegisterNever, d)

	// not AD. ignore specified value
	planner = newSharePlanner(
		&sambaoperatorv1alpha1.SmbShare{},
		&sambaoperatorv1alpha1.SmbSecurityConfig{
			Spec: sambaoperatorv1alpha1.SmbSecurityConfigSpec{
				Mode: "user",
				DNS: &sambaoperatorv1alpha1.SmbSecurityDNSSpec{
					Register: "cluster-ip",
				},
			}},
		&sambaoperatorv1alpha1.SmbCommonConfig{},
		&smbcc.SambaContainerConfig{})
	d = planner.dnsRegister()
	assert.Equal(t, dnsRegisterNever, d)
}

func TestPlannerDNSRegisterArgs(t *testing.T) {
	var (
		v       []string
		planner *sharePlanner
	)

	// external-ip
	planner = newSharePlanner(
		&sambaoperatorv1alpha1.SmbShare{},
		&sambaoperatorv1alpha1.SmbSecurityConfig{
			Spec: sambaoperatorv1alpha1.SmbSecurityConfigSpec{
				Mode: "active-directory",
				DNS: &sambaoperatorv1alpha1.SmbSecurityDNSSpec{
					Register: "external-ip",
				},
			}},
		&sambaoperatorv1alpha1.SmbCommonConfig{},
		&smbcc.SambaContainerConfig{})
	v = planner.dnsRegisterArgs()
	assert.Equal(t,
		[]string{"dns-register", "--watch", "/var/lib/svcwatch/status.json"},
		v)

	// cluster-ip
	planner = newSharePlanner(
		&sambaoperatorv1alpha1.SmbShare{},
		&sambaoperatorv1alpha1.SmbSecurityConfig{
			Spec: sambaoperatorv1alpha1.SmbSecurityConfigSpec{
				Mode: "active-directory",
				DNS: &sambaoperatorv1alpha1.SmbSecurityDNSSpec{
					Register: "cluster-ip",
				},
			}},
		&sambaoperatorv1alpha1.SmbCommonConfig{},
		&smbcc.SambaContainerConfig{})
	v = planner.dnsRegisterArgs()
	assert.Equal(t,
		[]string{"dns-register", "--watch", "--target=internal", "/var/lib/svcwatch/status.json"},
		v)
}

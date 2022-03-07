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
	"testing"

	"github.com/stretchr/testify/assert"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
	"github.com/samba-in-kubernetes/samba-operator/internal/smbcc"
)

func TestPlannerDNSRegister(t *testing.T) {
	var (
		d       DNSRegister
		planner *Planner
	)

	// no dns register specified
	planner = New(
		InstanceConfiguration{
			SecurityConfig: &sambaoperatorv1alpha1.SmbSecurityConfig{
				Spec: sambaoperatorv1alpha1.SmbSecurityConfigSpec{
					Mode: "active-directory",
				},
			},
		},
		&smbcc.SambaContainerConfig{})
	d = planner.DNSRegister()
	assert.Equal(t, DNSRegisterNever, d)

	// external-ip
	planner = New(
		InstanceConfiguration{
			SecurityConfig: &sambaoperatorv1alpha1.SmbSecurityConfig{
				Spec: sambaoperatorv1alpha1.SmbSecurityConfigSpec{
					Mode: "active-directory",
					DNS: &sambaoperatorv1alpha1.SmbSecurityDNSSpec{
						Register: "external-ip",
					},
				},
			},
		},
		&smbcc.SambaContainerConfig{})
	d = planner.DNSRegister()
	assert.Equal(t, DNSRegisterExternalIP, d)

	// cluster-ip
	planner = New(
		InstanceConfiguration{
			SecurityConfig: &sambaoperatorv1alpha1.SmbSecurityConfig{
				Spec: sambaoperatorv1alpha1.SmbSecurityConfigSpec{
					Mode: "active-directory",
					DNS: &sambaoperatorv1alpha1.SmbSecurityDNSSpec{
						Register: "cluster-ip",
					},
				},
			},
		},
		&smbcc.SambaContainerConfig{})
	d = planner.DNSRegister()
	assert.Equal(t, DNSRegisterClusterIP, d)

	// invalid string for register
	planner = New(
		InstanceConfiguration{
			SecurityConfig: &sambaoperatorv1alpha1.SmbSecurityConfig{
				Spec: sambaoperatorv1alpha1.SmbSecurityConfigSpec{
					Mode: "active-directory",
					DNS: &sambaoperatorv1alpha1.SmbSecurityDNSSpec{
						Register: "junk",
					},
				},
			},
		},
		&smbcc.SambaContainerConfig{})
	d = planner.DNSRegister()
	assert.Equal(t, DNSRegisterNever, d)

	// not AD. ignore specified value
	planner = New(
		InstanceConfiguration{
			SecurityConfig: &sambaoperatorv1alpha1.SmbSecurityConfig{
				Spec: sambaoperatorv1alpha1.SmbSecurityConfigSpec{
					Mode: "user",
					DNS: &sambaoperatorv1alpha1.SmbSecurityDNSSpec{
						Register: "cluster-ip",
					},
				},
			},
		},
		&smbcc.SambaContainerConfig{})
	d = planner.DNSRegister()
	assert.Equal(t, DNSRegisterNever, d)
}

func TestPlannerDNSRegisterArgs(t *testing.T) {
	var (
		v       []string
		planner *Planner
	)

	// external-ip
	planner = New(
		InstanceConfiguration{
			SecurityConfig: &sambaoperatorv1alpha1.SmbSecurityConfig{
				Spec: sambaoperatorv1alpha1.SmbSecurityConfigSpec{
					Mode: "active-directory",
					DNS: &sambaoperatorv1alpha1.SmbSecurityDNSSpec{
						Register: "external-ip",
					},
				},
			},
		},
		&smbcc.SambaContainerConfig{})
	v = planner.Args().DNSRegister()
	assert.Equal(t,
		[]string{"dns-register", "--watch", "/var/lib/svcwatch/status.json"},
		v)

	// cluster-ip
	planner = New(
		InstanceConfiguration{
			SecurityConfig: &sambaoperatorv1alpha1.SmbSecurityConfig{
				Spec: sambaoperatorv1alpha1.SmbSecurityConfigSpec{
					Mode: "active-directory",
					DNS: &sambaoperatorv1alpha1.SmbSecurityDNSSpec{
						Register: "cluster-ip",
					},
				},
			},
		},
		&smbcc.SambaContainerConfig{})
	v = planner.Args().DNSRegister()
	assert.Equal(t,
		[]string{
			"dns-register",
			"--watch",
			"--target=internal",
			"/var/lib/svcwatch/status.json",
		},
		v)
}

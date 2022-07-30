// SPDX-License-Identifier: Apache-2.0

package planner

import (
	"testing"

	"github.com/stretchr/testify/assert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
)

func phonyInstanceConfiguration() InstanceConfiguration {
	ic1 := InstanceConfiguration{
		SmbShare: &sambaoperatorv1alpha1.SmbShare{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "share1",
				Namespace: "smbshares",
				UID:       "phonyuid1",
			},
			Spec: sambaoperatorv1alpha1.SmbShareSpec{
				ShareName:      "share1",
				ReadOnly:       false,
				Browseable:     true,
				SecurityConfig: "myusers1",
				CommonConfig:   "mycommon1",
				Storage: sambaoperatorv1alpha1.SmbShareStorageSpec{
					Pvc: &sambaoperatorv1alpha1.SmbSharePvcSpec{
						Name: "mydata",
						Path: "share1",
					},
				},
			},
		},
	}
	return ic1
}

func phonyInstanceConfiguration2(ns, security, common, pvcname string) InstanceConfiguration {
	ic2 := InstanceConfiguration{
		SmbShare: &sambaoperatorv1alpha1.SmbShare{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "share2",
				Namespace: ns,
				UID:       "phonyuid2",
			},
			Spec: sambaoperatorv1alpha1.SmbShareSpec{
				ShareName:      "share2",
				ReadOnly:       false,
				Browseable:     false,
				SecurityConfig: security,
				CommonConfig:   common,
				Storage: sambaoperatorv1alpha1.SmbShareStorageSpec{
					Pvc: &sambaoperatorv1alpha1.SmbSharePvcSpec{
						Name: pvcname,
						Path: "share2",
					},
				},
			},
		},
	}
	return ic2
}

func TestCheckCompatible(t *testing.T) {
	ic1 := phonyInstanceConfiguration()

	t.Run("compatible", func(t *testing.T) {
		ic2 := phonyInstanceConfiguration2("smbshares", "myusers1", "mycommon1", "mydata")
		assert.NoError(t, CheckCompatible(ic1, ic2))
	})

	t.Run("invalidShares", func(t *testing.T) {
		icx := InstanceConfiguration{}
		err := CheckCompatible(icx, icx)
		if assert.Error(t, err) {
			iie := err.(IncompatibleInstanceError)
			assert.Equal(t, iie.current, "<invalid>")
			assert.Equal(t, iie.existing, "<invalid>")
		}
	})
	t.Run("invalidExisting", func(t *testing.T) {
		icx := InstanceConfiguration{}
		err := CheckCompatible(icx, ic1)
		if assert.Error(t, err) {
			iie := err.(IncompatibleInstanceError)
			assert.Equal(t, iie.current, "<invalid>")
			assert.Equal(t, iie.existing, "share1")
		}
	})
	t.Run("invalidCurrent", func(t *testing.T) {
		icx := InstanceConfiguration{}
		err := CheckCompatible(ic1, icx)
		if assert.Error(t, err) {
			iie := err.(IncompatibleInstanceError)
			assert.Equal(t, iie.current, "share1")
			assert.Equal(t, iie.existing, "<invalid>")
		}
	})

	t.Run("differentNamespace", func(t *testing.T) {
		ic2 := phonyInstanceConfiguration2("whoopsie", "myusers1", "mycommon1", "mydata")
		err := CheckCompatible(ic1, ic2)
		if assert.Error(t, err) {
			assert.ErrorContains(t, err, "namespaces")
		}
	})

	t.Run("differentPVC", func(t *testing.T) {
		ic2 := phonyInstanceConfiguration2("smbshares", "myusers1", "mycommon1", "foobar")
		err := CheckCompatible(ic1, ic2)
		if assert.Error(t, err) {
			assert.ErrorContains(t, err, "PersistentVolumeClaim name")
		}
	})

	t.Run("differentSecurityConfig", func(t *testing.T) {
		ic2 := phonyInstanceConfiguration2("smbshares", "xxxxx", "mycommon1", "mydata")
		err := CheckCompatible(ic1, ic2)
		if assert.Error(t, err) {
			assert.ErrorContains(t, err, "security config name")
		}
	})

	t.Run("differentCommonConfig", func(t *testing.T) {
		ic2 := phonyInstanceConfiguration2("smbshares", "myusers1", "zzzzz", "mydata")
		err := CheckCompatible(ic1, ic2)
		if assert.Error(t, err) {
			assert.ErrorContains(t, err, "common config name")
		}
	})
}

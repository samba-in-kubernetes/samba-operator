// SPDX-License-Identifier: Apache-2.0

package planner

import (
	"testing"

	"github.com/stretchr/testify/assert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
	"github.com/samba-in-kubernetes/samba-operator/internal/conf"
	"github.com/samba-in-kubernetes/samba-operator/internal/smbcc"
)

func TestUpdate(t *testing.T) {
	t.Run("simpleUpdate", func(t *testing.T) {
		testSimpleUpdate(t, smbcc.New())
	})
	t.Run("secondShare", func(t *testing.T) {
		testSecondShare(t, smbcc.New())
	})
	t.Run("adShare", func(t *testing.T) {
		testADShare(t, smbcc.New())
	})
}

func TestPrune(t *testing.T) {
	t.Run("addTwoPruneOne", func(t *testing.T) {
		testAddTwoPruneOne(t, smbcc.New())
	})
	t.Run("addTwoPruneTwo", func(t *testing.T) {
		testAddTwoPruneTwo(t, smbcc.New())
	})
	t.Run("addTwoPruneSame", func(t *testing.T) {
		testAddTwoPruneSame(t, smbcc.New())
	})
}

func sampleSmbShare1() *sambaoperatorv1alpha1.SmbShare {
	return &sambaoperatorv1alpha1.SmbShare{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "share1",
			Namespace: "smbshares",
			UID:       "phonyuid1",
		},
		Spec: sambaoperatorv1alpha1.SmbShareSpec{
			ShareName:      "share1",
			ReadOnly:       false,
			Browseable:     true,
			SecurityConfig: "",
			CommonConfig:   "",
			Storage: sambaoperatorv1alpha1.SmbShareStorageSpec{
				Pvc: &sambaoperatorv1alpha1.SmbSharePvcSpec{
					Name: "mydata",
					Path: "share1",
				},
			},
		},
	}
}

func sampleSmbShare2() *sambaoperatorv1alpha1.SmbShare {
	return &sambaoperatorv1alpha1.SmbShare{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "share2",
			Namespace: "smbshares",
			UID:       "phonyuid2",
		},
		Spec: sambaoperatorv1alpha1.SmbShareSpec{
			ShareName:      "share2",
			ReadOnly:       true,
			Browseable:     false,
			SecurityConfig: "",
			CommonConfig:   "",
			Storage: sambaoperatorv1alpha1.SmbShareStorageSpec{
				Pvc: &sambaoperatorv1alpha1.SmbSharePvcSpec{
					Name: "mydata",
					Path: "share2",
				},
			},
		},
	}
}

func sampleSmbShare3() *sambaoperatorv1alpha1.SmbShare {
	return &sambaoperatorv1alpha1.SmbShare{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "share3",
			Namespace: "smbshares",
			UID:       "phonyuid3",
		},
		Spec: sambaoperatorv1alpha1.SmbShareSpec{
			ShareName:      "",
			ReadOnly:       false,
			Browseable:     true,
			SecurityConfig: "sc1",
			CommonConfig:   "",
			Storage: sambaoperatorv1alpha1.SmbShareStorageSpec{
				Pvc: &sambaoperatorv1alpha1.SmbSharePvcSpec{
					Name: "mydata",
					Path: "share3",
				},
			},
		},
	}
}

func sampleADSecConfig1() *sambaoperatorv1alpha1.SmbSecurityConfig {
	return &sambaoperatorv1alpha1.SmbSecurityConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sc1",
			Namespace: "smbshares",
			UID:       "phonyadscuid1",
		},
		Spec: sambaoperatorv1alpha1.SmbSecurityConfigSpec{
			Mode:  "active-directory",
			Realm: "FOO.TEST",
		},
	}
}

func testSimpleUpdate(t *testing.T, state *smbcc.SambaContainerConfig) {
	assert.Len(t, state.Shares, 0)
	assert.Len(t, state.Configs, 0)
	assert.Len(t, state.Globals, 0)

	p := New(InstanceConfiguration{
		SmbShare:     sampleSmbShare1(),
		GlobalConfig: &conf.OperatorConfig{},
	}, state)

	// apply changes
	changed, err := p.Update()
	assert.NoError(t, err)
	assert.True(t, changed)

	// changes already applied
	changed, err = p.Update()
	assert.NoError(t, err)
	assert.False(t, changed)

	assert.Len(t, state.Shares, 1)
	assert.Len(t, state.Configs, 1)
	assert.Len(t, state.Globals, 1)
	assert.Contains(t, state.Shares, smbcc.Key("share1"))
}

func testSecondShare(t *testing.T, state *smbcc.SambaContainerConfig) {
	assert.Len(t, state.Shares, 0)
	assert.Len(t, state.Configs, 0)
	assert.Len(t, state.Globals, 0)

	p := New(InstanceConfiguration{
		SmbShare:     sampleSmbShare1(),
		GlobalConfig: &conf.OperatorConfig{},
	}, state)

	// apply changes
	changed, err := p.Update()
	assert.NoError(t, err)
	assert.True(t, changed)

	p2 := New(InstanceConfiguration{
		SmbShare:     sampleSmbShare2(),
		GlobalConfig: &conf.OperatorConfig{},
	}, state)
	changed, err = p2.Update()
	assert.NoError(t, err)
	assert.True(t, changed)

	assert.Len(t, state.Shares, 2)
	assert.Len(t, state.Configs, 1)
	assert.Len(t, state.Globals, 1)
	assert.Contains(t, state.Shares, smbcc.Key("share1"))
	assert.Contains(t, state.Shares, smbcc.Key("share2"))
	assert.Contains(t, state.Configs[p.instanceID()].Shares, smbcc.Key("share1"))
	assert.Contains(t, state.Configs[p.instanceID()].Shares, smbcc.Key("share2"))
}

func testADShare(t *testing.T, state *smbcc.SambaContainerConfig) {
	assert.Len(t, state.Shares, 0)
	assert.Len(t, state.Configs, 0)
	assert.Len(t, state.Globals, 0)

	p := New(InstanceConfiguration{
		SmbShare:       sampleSmbShare3(),
		SecurityConfig: sampleADSecConfig1(),
		GlobalConfig:   &conf.OperatorConfig{},
	}, state)

	// apply changes
	changed, err := p.Update()
	assert.NoError(t, err)
	assert.True(t, changed)

	assert.Len(t, state.Shares, 1)
	assert.Len(t, state.Configs, 1)
	assert.Len(t, state.Globals, 2)
	assert.Contains(t, state.Shares, smbcc.Key("share3"))
	assert.Contains(t, state.Configs[p.instanceID()].Shares, smbcc.Key("share3"))
	assert.Contains(t, state.Globals, smbcc.Key("FOO.TEST"))
}

func testAddTwoPruneOne(t *testing.T, state *smbcc.SambaContainerConfig) {
	testSecondShare(t, state)

	p := New(InstanceConfiguration{
		SmbShare:     sampleSmbShare1(),
		GlobalConfig: &conf.OperatorConfig{},
	}, state)
	changed, err := p.Prune()
	assert.NoError(t, err)
	assert.True(t, changed)

	assert.Len(t, state.Shares, 1)
	assert.Len(t, state.Configs, 1)
	assert.Len(t, state.Globals, 1)
	assert.Contains(t, state.Shares, smbcc.Key("share2"))
	assert.Contains(t, state.Configs[p.instanceID()].Shares, smbcc.Key("share2"))
	assert.NotContains(t, state.Configs[p.instanceID()].Shares, smbcc.Key("share1"))
}

func testAddTwoPruneTwo(t *testing.T, state *smbcc.SambaContainerConfig) {
	testAddTwoPruneOne(t, state)

	p := New(InstanceConfiguration{
		SmbShare:     sampleSmbShare2(),
		GlobalConfig: &conf.OperatorConfig{},
	}, state)
	changed, err := p.Prune()
	assert.NoError(t, err)
	assert.True(t, changed)

	assert.Len(t, state.Shares, 0)
	assert.Len(t, state.Configs, 1)
	assert.Len(t, state.Globals, 1)
	assert.NotContains(t, state.Configs[p.instanceID()].Shares, smbcc.Key("share1"))
	assert.NotContains(t, state.Configs[p.instanceID()].Shares, smbcc.Key("share2"))
}

func testAddTwoPruneSame(t *testing.T, state *smbcc.SambaContainerConfig) {
	testAddTwoPruneOne(t, state)

	p := New(InstanceConfiguration{
		SmbShare:     sampleSmbShare1(),
		GlobalConfig: &conf.OperatorConfig{},
	}, state)
	changed, err := p.Prune()
	assert.NoError(t, err)
	assert.False(t, changed)

	assert.Len(t, state.Shares, 1)
	assert.Len(t, state.Configs, 1)
	assert.Len(t, state.Globals, 1)
	assert.NotContains(t, state.Configs[p.instanceID()].Shares, smbcc.Key("share1"))
	assert.Contains(t, state.Configs[p.instanceID()].Shares, smbcc.Key("share2"))
}

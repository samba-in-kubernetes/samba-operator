// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
)

func testingManager() *SmbShareManager {
	scheme, err := sambaoperatorv1alpha1.SchemeBuilder.Build()
	if err != nil {
		panic(err)
	}
	m := &SmbShareManager{
		scheme: scheme,
		logger: &fakeLogger{},
		client: &fakeClient{scheme: scheme},
	}
	return m
}

func TestClaimOwnership(t *testing.T) {
	ctx := context.Background()
	m := testingManager()
	assert.NotNil(t, m)
	s := &sambaoperatorv1alpha1.SmbShare{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "professor",
			Namespace: "frink",
			UID:       "abc123",
		},
	}
	cm, err := newDefaultConfigMap("cm", "frink")
	assert.NoError(t, err)

	changed, err := m.claimOwnership(ctx, s, cm)
	assert.NoError(t, err)
	assert.True(t, changed)

	changed, err = m.claimOwnership(ctx, s, cm)
	assert.NoError(t, err)
	assert.False(t, changed)

	// intentionally break stuff
	oref := metav1.OwnerReference{
		APIVersion: "////",
		Kind:       "fish",
		UID:        "fry",
		Name:       "flounder",
	}
	cm.SetOwnerReferences(append(cm.GetOwnerReferences(), oref))

	// change the name and uid so we are sure there will be no
	// match before it hits the invalid owner reference
	s.Name = "professorx"
	s.UID = "321bca"
	_, err = m.claimOwnership(ctx, s, cm)
	assert.Error(t, err)
}

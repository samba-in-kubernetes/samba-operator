// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	rtclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

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

func TestTransferOwnership(t *testing.T) {
	ctx := context.Background()
	m := testingManager()
	assert.NotNil(t, m)
	s1 := &sambaoperatorv1alpha1.SmbShare{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gumby",
			Namespace: "clayland",
			UID:       "111111111",
		},
	}
	s2 := &sambaoperatorv1alpha1.SmbShare{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pokey",
			Namespace: "clayland",
			UID:       "222222222",
		},
	}

	t.Run("standardTransfer", func(t *testing.T) {
		// successfully transfer controller-owner status from share s1
		// to share s2
		cm, err := newDefaultConfigMap("cm", "clayland")
		assert.NoError(t, err)
		err = controllerutil.SetControllerReference(
			s1, cm, m.scheme)
		assert.NoError(t, err)

		changed, err := m.claimOwnership(ctx, s2, cm)
		assert.NoError(t, err)
		assert.True(t, changed)

		res := m.transferOwnership(ctx, cm, s1)
		assert.NoError(t, res.err)
		assert.True(t, res.requeue)
	})

	t.Run("noOtherOwners", func(t *testing.T) {
		// nothing to do if nothing else owns the resource
		cm, err := newDefaultConfigMap("cm", "clayland")
		assert.NoError(t, err)
		err = controllerutil.SetControllerReference(
			s1, cm, m.scheme)
		assert.NoError(t, err)

		res := m.transferOwnership(ctx, cm, s1)
		assert.NoError(t, res.err)
		assert.False(t, res.requeue)
	})

	t.Run("notNeeded", func(t *testing.T) {
		// owner transfer is not needed if the object we are transferring
		// away from is not the controller-owner
		cm, err := newDefaultConfigMap("cm", "clayland")
		assert.NoError(t, err)
		err = controllerutil.SetControllerReference(
			s1, cm, m.scheme)
		assert.NoError(t, err)

		changed, err := m.claimOwnership(ctx, s2, cm)
		assert.NoError(t, err)
		assert.True(t, changed)

		res := m.transferOwnership(ctx, cm, s2)
		assert.NoError(t, res.err)
		assert.False(t, res.requeue)
	})

	t.Run("nothingValid", func(t *testing.T) {
		// if all possible other owners are being deleted there
		// is nothing valid to transfer to (so we do nothing)
		// we need to mark the other possible owner as being deleted
		// and return it from the fake client when it's getting
		// fetched by name
		cm, err := newDefaultConfigMap("cm", "clayland")
		assert.NoError(t, err)
		err = controllerutil.SetControllerReference(
			s1, cm, m.scheme)
		assert.NoError(t, err)

		s3 := &sambaoperatorv1alpha1.SmbShare{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "goo",
				Namespace: "clayland",
				UID:       "33333",
			},
		}
		changed, err := m.claimOwnership(ctx, s3, cm)
		assert.NoError(t, err)
		assert.True(t, changed)

		ts := metav1.Now()
		s3.SetDeletionTimestamp(&ts)
		m.client.(*fakeClient).clientGet = func(
			_ context.Context,
			nn types.NamespacedName,
			obj rtclient.Object) error {
			// ---
			if nn.Namespace == "clayland" && nn.Name == "goo" {
				out := obj.(*sambaoperatorv1alpha1.SmbShare)
				s3.DeepCopyInto(out)
				return nil
			}
			if nn.Namespace == "clayland" && nn.Name == "gumby" {
				out := obj.(*sambaoperatorv1alpha1.SmbShare)
				s1.DeepCopyInto(out)
				return nil
			}
			return fmt.Errorf("unexpected name: %s/%s", nn.Namespace, nn.Name)
		}

		res := m.transferOwnership(ctx, cm, s1)
		assert.NoError(t, res.err)
		assert.False(t, res.requeue)
	})

	t.Run("failLookup", func(t *testing.T) {
		// fail to look up the smbshare owner that isn't the conteroller-owner
		// to do this we need to return an error from the fake client
		cm, err := newDefaultConfigMap("cm", "clayland")
		assert.NoError(t, err)
		err = controllerutil.SetControllerReference(
			s1, cm, m.scheme)
		assert.NoError(t, err)

		changed, err := m.claimOwnership(ctx, s2, cm)
		assert.NoError(t, err)
		assert.True(t, changed)

		m.client.(*fakeClient).clientGet = func(
			_ context.Context,
			_ types.NamespacedName,
			_ rtclient.Object) error {
			// ---
			return fmt.Errorf("wild failure")
		}

		res := m.transferOwnership(ctx, cm, s1)
		assert.Error(t, res.err)
		assert.False(t, res.requeue)
	})
}

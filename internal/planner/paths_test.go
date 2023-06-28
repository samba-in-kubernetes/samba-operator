// SPDX-License-Identifier: Apache-2.0

package planner

import (
	"testing"

	"github.com/stretchr/testify/assert"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
)

func TestShareMountPath(t *testing.T) {
	ic := phonyInstanceConfiguration()
	t.Run("neverGrouped", func(t *testing.T) {
		ic.SmbShare.Spec.Scaling = &sambaoperatorv1alpha1.SmbShareScalingSpec{
			GroupMode: string(GroupModeNever),
		}
		planner := New(ic, nil)
		mp := planner.Paths().ShareMountPath()
		assert.Equal(t, "/mnt/phonyuid1", mp)
	})
	t.Run("explicitGroup", func(t *testing.T) {
		ic.SmbShare.Spec.Scaling = &sambaoperatorv1alpha1.SmbShareScalingSpec{
			GroupMode: string(GroupModeExplicit),
			Group:     "goodgroup",
		}
		planner := New(ic, nil)
		mp := planner.Paths().ShareMountPath()
		assert.Equal(t, "/mnt/goodgroup", mp)
	})
}

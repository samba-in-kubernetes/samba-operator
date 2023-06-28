// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func genVolMount(name, path string, tag volMountTag) volMount {
	var vmnt volMount
	vmnt.volume = corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
		},
	}
	// mount
	vmnt.mount = corev1.VolumeMount{
		MountPath: path,
		Name:      name,
	}
	vmnt.tag = tag
	return vmnt
}

func TestVolKeeper(t *testing.T) {
	vm1 := genVolMount("foo", "/mnt/foo", tagData)
	vm2 := genVolMount("bar", "/var/bar", tagMeta)

	vk1 := newVolKeeper()
	assert.Len(t, vk1.all(), 0)

	vk1.add(vm1)
	vk1.add(vm2)
	t.Run("lengthAfterAdd", func(t *testing.T) {
		assert.Len(t, vk1.all(), 2)
	})

	vk2 := newVolKeeper()
	assert.Len(t, vk2.all(), 0)

	vk2.extend([]volMount{vm1, vm2})
	t.Run("lengthAfterExtend", func(t *testing.T) {
		assert.Len(t, vk2.all(), 2)
	})

	t.Run("clone", func(t *testing.T) {
		vk3 := vk1.clone()
		vall := vk3.all()
		if assert.Len(t, vall, 2) {
			assert.Equal(t, "foo", vall[0].volume.Name)
			assert.Equal(t, "bar", vall[1].volume.Name)
		}
	})

	t.Run("validateMismatch", func(t *testing.T) {
		vk3 := vk1.clone()
		assert.NoError(t, vk3.validate())
		vm3 := genVolMount("baz", "/", tagData)
		vm3.volume.Name = "bib"
		vk3.add(vm3)
		assert.Error(t, vk3.validate())
	})
	t.Run("validateDuplicate", func(t *testing.T) {
		vk3 := vk1.clone()
		assert.NoError(t, vk3.validate())
		vm3 := genVolMount("foo", "/", tagData)
		vk3.add(vm3)
		assert.Error(t, vk3.validate())
	})
	t.Run("mustValidatePanics", func(t *testing.T) {
		vk3 := vk1.clone()
		vm3 := genVolMount("foo", "/", tagData)
		vk3.add(vm3)
		assert.Panics(t, func() {
			vk3.mustValidate()
		})
	})

	t.Run("exclude", func(t *testing.T) {
		assert.Len(t, vk1.exclude(tagData).all(), 1)
		assert.Len(t, vk1.exclude(tagMeta).all(), 1)
		assert.Len(t, vk1.exclude(tagCTDBMeta).all(), 2)
	})
}

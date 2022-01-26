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

	"github.com/samba-in-kubernetes/samba-operator/internal/smbcc"
)

func TestNewDefaultConfigMap(t *testing.T) {
	cm, err := newDefaultConfigMap("a", "b")
	assert.NoError(t, err)
	assert.NotNil(t, cm)
	assert.Equal(t, cm.Name, "a")
	assert.Equal(t, cm.Namespace, "b")
}

func TestGetContainerConfig(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		cm, err := newDefaultConfigMap("a", "b")
		assert.NoError(t, err)
		assert.NotNil(t, cm)

		cc, err := getContainerConfig(cm)
		assert.NoError(t, err)
		assert.NotNil(t, cc)
		assert.Equal(t, cc.SCCVersion, "v0")
		assert.Len(t, cc.Configs, 0)
	})
	t.Run("emptyMap", func(t *testing.T) {
		cm, err := newDefaultConfigMap("a", "b")
		assert.NoError(t, err)
		assert.NotNil(t, cm)
		delete(cm.Data, ConfigJSONKey)

		cc, err := getContainerConfig(cm)
		assert.NoError(t, err)
		assert.NotNil(t, cc)
	})
	t.Run("invalidContent", func(t *testing.T) {
		cm, err := newDefaultConfigMap("a", "b")
		assert.NoError(t, err)
		assert.NotNil(t, cm)
		cm.Data[ConfigJSONKey] = "I'm invalid"

		cc, err := getContainerConfig(cm)
		assert.Error(t, err)
		assert.Nil(t, cc)
	})
}

func TestSetContainerConfig(t *testing.T) {
	cm, err := newDefaultConfigMap("a", "b")
	assert.NoError(t, err)
	assert.NotNil(t, cm)

	cc := smbcc.New()
	k := smbcc.Key("example")
	cc.Configs[k] = smbcc.NewConfigSection("example")
	err = setContainerConfig(cm, cc)
	assert.NoError(t, err)

	cc2, err := getContainerConfig(cm)
	assert.NoError(t, err)
	assert.Equal(t, cc.Configs[k].InstanceName, cc2.Configs[k].InstanceName)
}

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
	"context"
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	rtclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/samba-in-kubernetes/samba-operator/internal/smbcc"
)

const (
	// ConfigMapName is the name of the configmap we store in.
	ConfigMapName = "samba-container-config"
	// ConfigJSONKey is the name of the key our json is under.
	ConfigJSONKey = "config.json"
)

func getConfigMap(
	ctx context.Context, client rtclient.Client, ns string) (
	*corev1.ConfigMap, error) {
	// fetch the existing config, if available
	cm := &corev1.ConfigMap{}
	err := client.Get(
		ctx,
		types.NamespacedName{
			Name:      ConfigMapName,
			Namespace: ns,
		},
		cm)
	return cm, err
}

func getOrCreateConfigMap(
	ctx context.Context, client rtclient.Client, ns string) (
	*corev1.ConfigMap, bool, error) {
	// fetch the existing config, if available
	cm, err := getConfigMap(ctx, client, ns)
	if err == nil {
		return cm, false, nil
	}

	if errors.IsNotFound(err) {
		cm, err = newDefaultConfigMap(ConfigMapName, ns)
		if err != nil {
			return cm, false, err
		}
		err = client.Create(ctx, cm)
		if err != nil {
			return cm, false, err
		}
		// Deployment created successfully
		return cm, true, nil
	}
	return nil, false, err
}

func newDefaultConfigMap(name, ns string) (*corev1.ConfigMap, error) {
	// we use marshal indent so that the json is semi-human-readable
	// so that debugging is not so tedious.
	jb, err := json.MarshalIndent(smbcc.New(), "", "  ")
	if err != nil {
		return nil, err
	}
	data := map[string]string{ConfigJSONKey: string(jb)}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Data: data,
	}
	return cm, nil
}

func getContainerConfig(cm *corev1.ConfigMap) (*smbcc.SambaContainerConfig, error) {
	cc := smbcc.New()
	jstr, found := cm.Data[ConfigJSONKey]
	if !found {
		return cc, nil
	}
	if err := json.Unmarshal([]byte(jstr), cc); err != nil {
		return nil, err
	}
	return cc, nil
}

func setContainerConfig(cm *corev1.ConfigMap, cc *smbcc.SambaContainerConfig) error {
	jb, err := json.MarshalIndent(cc, "", "  ")
	if err != nil {
		return err
	}
	cm.Data[ConfigJSONKey] = string(jb)
	return nil
}

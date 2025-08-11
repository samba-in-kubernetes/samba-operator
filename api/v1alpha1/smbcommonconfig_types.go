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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags
// for the fields to be serialized.

// SmbCommonConfigSpec values act as a template for properties of the services
// that will host shares.
type SmbCommonConfigSpec struct {
	// Network specifies what kind of networking shares associated with
	// this config will use.
	// +kubebuilder:validation:Required
	Network SmbCommonNetworkSpec `json:"network,omitempty"`

	// PodSettings are configuration values that are applied to pods that
	// the operator may create in order to host shares. The values specified
	// under PodSettings allow admins and users to customize how pods
	// are scheduled in a kubernetes cluster.
	PodSettings *SmbCommonConfigPodSettings `json:"podSettings,omitempty"`

	// StateSCName specifies which StorageClass is to be used for this share.
	// If left empty, the operator's default will be used.
	// +optional
	StatePVSCName string `json:"statePVSCName,omitempty"`

  // GlobalConfig are configuration values that are applied to [global]
	// section in smb.conf for the smb server. This allows users to add or
	// override default configurations.
	// +opional
	CustomGlobalConfig *SmbCommonConfigGlobalConfig `json:"customGlobalConfig,omitempty"`
}

// SmbCommonNetworkSpec values define networking properties for the services
// that will host shares.
type SmbCommonNetworkSpec struct {
	// Publish broadly specifies what kind of networking shares associated with
	// this config are expected to use.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum:=cluster;external
	Publish string `json:"publish,omitempty"`
}

// SmbCommonConfigPodSettings contains values pertaining to the customization
// of pods created by the samba operator.
type SmbCommonConfigPodSettings struct {
	// NodeSelector values will be assigned to a PodSpec's NodeSelector.
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Affinity values will be used as defaults for pods created by the
	// samba operator.
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
}

// SmbCommonConfigGlobalConfig contains values for customizing configs in
// [global] section in smb.conf
type SmbCommonConfigGlobalConfig struct {
	// Check if the user wants to use custom configs
	UseUnsafeCustomConfig bool `json:"useUnsafeCustomConfig,omitempty"`
	// Configs specify keys and values to smb.conf
	Configs map[string]string `json:"configs,omitempty"`
}

// SmbCommonConfigStatus defines the observed state of SmbCommonConfig
type SmbCommonConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// SmbCommonConfig is the Schema for the smbcommonconfigs API
type SmbCommonConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SmbCommonConfigSpec   `json:"spec,omitempty"`
	Status SmbCommonConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SmbCommonConfigList contains a list of SmbCommonConfig
type SmbCommonConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SmbCommonConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SmbCommonConfig{}, &SmbCommonConfigList{})
}

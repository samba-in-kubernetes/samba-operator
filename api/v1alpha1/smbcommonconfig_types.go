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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SmbCommonConfigSpec values act as a template for properties of the services
// that will host shares.
type SmbCommonConfigSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Network specifies what kind of networking shares associated with
	// this config will use.
	// +kubebuilder:validation:Required
	Network SmbCommonNetworkSpec `json:"network,omitempty"`
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

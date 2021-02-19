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

// SmbSecurityConfigSpec defines the desired state of SmbSecurityConfig
type SmbSecurityConfigSpec struct {
	// Mode specifies what approach to security is being used.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum:=user;active-directory
	Mode string `json:"mode,omitempty"`

	// Users is used to configure "local" user and group based security.
	Users *SmbSecurityUsersSpec `json:"users,omitempty"`
}

// SmbSecurityUsersSpec configures user level security.
type SmbSecurityUsersSpec struct {
	// Secret identifies the name of the secret storing user and group
	// configuration json.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength:=1
	Secret string `json:"secret,omitempty"`

	// Key identifies the key within the secret that stores the user and
	// group configuration json.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength:=1
	Key string `json:"key,omitempty"`
}

// SmbSecurityConfigStatus defines the observed state of SmbSecurityConfig
type SmbSecurityConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// SmbSecurityConfig is the Schema for the smbsecurityconfigs API
type SmbSecurityConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SmbSecurityConfigSpec   `json:"spec,omitempty"`
	Status SmbSecurityConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SmbSecurityConfigList contains a list of SmbSecurityConfig
type SmbSecurityConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SmbSecurityConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SmbSecurityConfig{}, &SmbSecurityConfigList{})
}

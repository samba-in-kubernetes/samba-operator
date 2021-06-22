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

	// Realm specifies the active directory domain to use.
	Realm string `json:"realm,omitempty"`

	// JoinSources holds a list of sources for domain join data for
	// this configuration.
	JoinSources []SmbSecurityJoinSpec `json:"joinSources,omitempty"`

	// Domains holds a list of primary & trusted domain configurations.
	// If left empty a simple default that automatically works with
	// trusted domains will be used.
	// +optional
	Domains []SmbSecurityDomainSpec `json:"domains,omitempty"`

	// DNS is used to configure properties related to the DNS services
	// of the domain.
	// +optional
	DNS *SmbSecurityDNSSpec `json:"dns,omitempty"`
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

// SmbSecurityJoinSpec configures how samba instances are allowed to
// join to active directory if needed.
type SmbSecurityJoinSpec struct {
	UserJoin *SmbSecurityUserJoinSpec `json:"userJoin,omitempty"`
}

// SmbSecurityUserJoinSpec configures samba container instances to
// use a secret containing a username and password.
type SmbSecurityUserJoinSpec struct {
	// Secret that contains the username and password.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength:=1
	Secret string `json:"secret,omitempty"`
	// Key within the secret containing the username and password.
	// +kubebuilder:default:=join.json
	// +optional
	Key string `json:"key,omitempty"`
}

// SmbSecurityDomainSpec configures samba's domain management and ID mapping
// behavior for the specified domain.
type SmbSecurityDomainSpec struct {
	// Name of the domain.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength:=1
	Name string `json:"name,omitempty"`

	// Mode specifies what approach to security is being used.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum:=autorid;ad-rfc2307
	Backend string `json:"backend,omitempty"`

	// TODO: add support for id mapping ranges, etc.
}

// SmbSecurityDNSSpec configures the relationship between systems managed
// vi this SmbSecurityConfig and the domain. Ignored by user mode.
type SmbSecurityDNSSpec struct {
	// Register a specified member server's address with the domain's DNS or
	// disabled when set to "never".
	// NOTE: cluster-ip is not generally supported, it is only for testing.
	// +kubebuilder:validation:Enum:=never;external-ip;cluster-ip
	Register string `json:"register,omitempty"`
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

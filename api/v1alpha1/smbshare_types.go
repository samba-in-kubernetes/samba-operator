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
// Important: Run "make" to regenerate code after modifying this file
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SmbShareSpec defines the desired state of SmbShare
type SmbShareSpec struct {
	// TODO validation of share name

	// ShareName is an optional string that lets you define an SMB compliant
	// name for the share. If unset, the name will be derived automatically.
	// +optional
	ShareName string `json:"shareName,omitempty"`

	// Storage defines the type and location of the storage that backs this
	// share.
	Storage SmbShareStorageSpec `json:"storage"`

	// ReadOnly controls if this share is to be read-only or not.
	// +kubebuilder:default:=false
	// +optional
	ReadOnly bool `json:"readOnly"`

	// Browseable controls if the share will be browseable. A browseable share
	// is visible in listings.
	// +kubebuilder:default:=true
	// +optional
	Browseable bool `json:"browseable"`

	// SecurityConfig specifies which SmbSecurityConfig CR is to be used
	// for this share. If left blank, the operator's default will be
	// used.
	// +kubebuilder:validation:MinLength:=1
	// +optional
	SecurityConfig string `json:"securityConfig,omitempty"`
}

// SmbShareStorageSpec defines how storage is associated with a share.
type SmbShareStorageSpec struct {
	// Pvc defines PVC backed storage for this share.
	// +optional
	Pvc *SmbSharePvcSpec `json:"pvc,omitempty"`
}

// SmbSharePvcSpec defines how a PVC may be associated with a share.
type SmbSharePvcSpec struct {
	// Name of the PVC to use for the share.
	// +optional
	Name string `json:"name,omitempty"`

	// Spec defines a new, temporary, PVC to use for the share.
	// Behaves similar to the embedded PVC spec for pods.
	// +optional
	Spec *corev1.PersistentVolumeClaimSpec `json:"spec,omitempty"`
}

// SmbShareStatus defines the observed state of SmbShare
type SmbShareStatus struct {
	// ServerGroup is a string indicating a name for the smb server or group of
	// servers hosting this share. The name is assigned by the operator but is
	// frequently the same as the SmbShare resource's name.
	ServerGroup string `json:"serverGroup,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// SmbShare is the Schema for the smbshares API
type SmbShare struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SmbShareSpec   `json:"spec,omitempty"`
	Status SmbShareStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SmbShareList contains a list of SmbShare
type SmbShareList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SmbShare `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SmbShare{}, &SmbShareList{})
}

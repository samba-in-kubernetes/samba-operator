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
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SmbPvcSpec defines the desired state of SmbPvc
type SmbPvcSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// TODO: Unsure yet whether we need the PVC params embedded or under
	// their own layer...
	Pvc *corev1.PersistentVolumeClaimSpec `json:"pvc"`
	//*corev1.PersistentVolumeClaimSpec `json:",inline"`
}

// SmbPvcStatus defines the observed state of SmbPvc
type SmbPvcStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// SmbPvc is the Schema for the smbpvcs API
type SmbPvc struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SmbPvcSpec   `json:"spec,omitempty"`
	Status SmbPvcStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SmbPvcList contains a list of SmbPvc
type SmbPvcList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SmbPvc `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SmbPvc{}, &SmbPvcList{})
}

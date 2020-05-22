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
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// TODO: Unsure yet whether we need the PVC params embedded or under
	// their own layer...
	//Pvc *corev1.PersistentVolumeClaim `json:"pvc"`
	*corev1.PersistentVolumeClaimSpec `json:",inline"`
}

// SmbPvcStatus defines the observed state of SmbPvc
type SmbPvcStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SmbPvc is the Schema for the smbpvcs API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=smbpvcs,scope=Namespaced
type SmbPvc struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SmbPvcSpec   `json:"spec,omitempty"`
	Status SmbPvcStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SmbPvcList contains a list of SmbPvc
type SmbPvcList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SmbPvc `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SmbPvc{}, &SmbPvcList{})
}

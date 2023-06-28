// SPDX-License-Identifier: Apache-2.0

package resources

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
)

func smbShareOwnerRefs(obj metav1.Object) ([]metav1.OwnerReference, error) {
	found := []metav1.OwnerReference{}
	smbgv := sambaoperatorv1alpha1.GroupVersion
	refs := obj.GetOwnerReferences()
	for _, ref := range refs {
		refgv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil, err
		}
		// we intentionally don't check the version as it can change
		// but the resource would still be "our" SmbShare
		if refgv.Group == smbgv.Group && ref.Kind == "SmbShare" {
			found = append(found, ref)
		}
	}
	return found, nil
}

func ownerRefsToNames(
	refs []metav1.OwnerReference, ns string) []types.NamespacedName {
	// ---
	owners := []types.NamespacedName{}
	for _, ref := range refs {
		owners = append(owners, types.NamespacedName{
			Namespace: ns,
			Name:      ref.Name,
		})
	}
	return owners
}

func excludeOwnerRefs(
	refs []metav1.OwnerReference,
	name string,
	uid types.UID) []metav1.OwnerReference {
	// ---
	out := []metav1.OwnerReference{}
	for _, ref := range refs {
		if ref.Name != name || ref.UID != uid {
			out = append(out, ref)
		}
	}
	return out
}

func ownerSharesExcluding(
	obj metav1.Object,
	s *sambaoperatorv1alpha1.SmbShare) ([]types.NamespacedName, error) {
	// ---
	refs, err := smbShareOwnerRefs(obj)
	if err != nil {
		return nil, err
	}
	otherRefs := excludeOwnerRefs(refs, s.GetName(), s.GetUID())
	return ownerRefsToNames(otherRefs, s.GetNamespace()), nil
}

func changeControllerOwnerTo(
	obj metav1.Object,
	target *metav1.OwnerReference) {
	// ---
	refs := obj.GetOwnerReferences()
	for i := range refs {
		if refs[i].Name == target.Name && refs[i].UID == target.UID {
			v := true
			refs[i].Controller = &v
			refs[i].BlockOwnerDeletion = &v
		} else {
			refs[i].Controller = nil
			refs[i].BlockOwnerDeletion = nil
		}
	}
	obj.SetOwnerReferences(refs)
}

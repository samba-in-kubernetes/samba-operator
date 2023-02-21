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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pln "github.com/samba-in-kubernetes/samba-operator/internal/planner"
)

func buildStatefulSet(
	planner *pln.Planner,
	dataPVCName, statePVCName, ns string) *appsv1.StatefulSet {
	// ---
	labels := labelsForSmbServer(planner)
	size := planner.ClusterSize()
	podSpec := buildClusteredPodSpec(planner, dataPVCName, statePVCName)
	if planner.NodeSpread() {
		podSpec.Affinity = buildOneSmbdPerNodeAffinity(planner, labels, serviceLabel)
	} else {
		podSpec.Affinity = affinityForSmbPod(planner)
	}
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      planner.InstanceName(),
			Namespace: ns,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &size,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotationsForSmbPod(planner.GlobalConfig),
				},
				Spec: podSpec,
			},
		},
	}
	return statefulSet
}

func buildOneSmbdPerNodeAffinity(
	planner *pln.Planner,
	labels map[string]string,
	key string) *corev1.Affinity {
	// ---
	topoKey := "kubernetes.io/hostname"
	value := labels[key]

	onePodPerNode := corev1.PodAffinityTerm{
		TopologyKey: topoKey,
		LabelSelector: &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{{
				Key:      key,
				Operator: metav1.LabelSelectorOpIn,
				Values: []string{
					value,
				},
			}},
		},
	}

	affinity := affinityForSmbPod(planner)
	if affinity == nil {
		affinity = &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{},
		}
	}
	affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(
		affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
		onePodPerNode)
	// ^ what a mouthful!
	return affinity
}

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
)

func buildStatefulSet(
	planner *sharePlanner,
	dataPVCName, statePVCName, ns string) *appsv1.StatefulSet {
	// ---
	labels := labelsForSmbServer(planner.instanceName())
	size := planner.clusterSize()
	podSpec := buildClusteredPodSPec(planner, dataPVCName, statePVCName)
	if planner.nodeSpread() {
		podSpec.Affinity = buildOneSmbdPerNodeAffinity(labels, serviceLabel)
	}
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      planner.instanceName(),
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
					Annotations: annotationsForSmbPod(planner.GlobalConfig.SmbdContainerName),
				},
				Spec: podSpec,
			},
		},
	}
	return statefulSet
}

func buildOneSmbdPerNodeAffinity(labels map[string]string, key string) *corev1.Affinity {
	topoKey := "kubernetes.io/hostname"
	value := labels[key]

	aaTerms := []corev1.PodAffinityTerm{{
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
	}}
	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: aaTerms,
			// ^ what a mouthful!
		},
	}
}

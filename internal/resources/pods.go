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
	corev1 "k8s.io/api/core/v1"

	"github.com/samba-in-kubernetes/samba-operator/internal/conf"
)

func buildPodSpec(cfg *conf.OperatorConfig, pvcName string) corev1.PodSpec {
	pvcVolName := pvcName + "-smb"
	podSpec := corev1.PodSpec{
		Volumes: []corev1.Volume{{
			Name: pvcVolName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		}},
		Containers: []corev1.Container{{
			Image: cfg.SmbdContainerImage,
			Name:  cfg.SmbdContainerName,
			//NEEDED? - Command: []string{"cmd", "arg", "arg", "..."},
			Ports: []corev1.ContainerPort{{
				ContainerPort: 445,
				Name:          "smb",
			}},
			VolumeMounts: []corev1.VolumeMount{{
				MountPath: "/share",
				Name:      pvcVolName,
			}},
		}},
	}
	return podSpec
}

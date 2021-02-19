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

const (
	userSecretVolName = "users-config"
)

func buildPodSpec(planner *sharePlanner, cfg *conf.OperatorConfig, pvcName string) corev1.PodSpec {
	pvcVolName := pvcName + "-smb"
	cmSrc := &corev1.ConfigMapVolumeSource{}
	cmSrc.Name = ConfigMapName
	volumes := []corev1.Volume{
		{
			Name: pvcVolName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		},
		{
			Name: ConfigMapName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: cmSrc,
			},
		},
	}
	mounts := []corev1.VolumeMount{
		{
			MountPath: planner.sharePath(),
			Name:      pvcVolName,
		},
		{
			MountPath: planner.containerConfigDir(),
			Name:      ConfigMapName,
		},
	}
	if planner.securityMode() == userMode {
		uss := planner.userSecuritySource()
		if uss.Configured {
			volumes = append(volumes, corev1.Volume{
				Name: userSecretVolName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: uss.Secret,
						Items: []corev1.KeyToPath{{
							Key:  uss.Key,
							Path: planner.usersConfigFileName(),
						}},
					},
				},
			})
			mounts = append(mounts, corev1.VolumeMount{
				MountPath: planner.usersConfigDir(),
				Name:      userSecretVolName,
			})
		}
	}
	podSpec := corev1.PodSpec{
		Volumes: volumes,
		Containers: []corev1.Container{{
			Image: cfg.SmbdContainerImage,
			Name:  cfg.SmbdContainerName,
			Args:  []string{"run", "smbd"},
			Env: []corev1.EnvVar{
				{
					Name:  "SAMBA_CONTAINER_ID",
					Value: string(planner.instanceID()),
				},
				{
					Name:  "SAMBACC_CONFIG",
					Value: planner.containerConfigPath(),
				},
			},
			Ports: []corev1.ContainerPort{{
				ContainerPort: 445,
				Name:          "smb",
			}},
			VolumeMounts: mounts,
		}},
	}
	return podSpec
}

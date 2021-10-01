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
)

const (
	userSecretVolName = "users-config"
	wbSocketsVolName  = "samba-wb-sockets-dir"
	stateVolName      = "samba-state-dir"
	osRunVolName      = "run"
	joinJSONVolName   = "join-data"
)

func shareVolumeAndMount(planner *sharePlanner, pvcName string) (
	corev1.Volume, corev1.VolumeMount) {
	// volume
	pvcVolName := pvcName + "-smb"
	volume := corev1.Volume{
		Name: pvcVolName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
			},
		},
	}
	// mount
	mount := corev1.VolumeMount{
		MountPath: planner.sharePath(),
		Name:      pvcVolName,
	}
	return volume, mount
}

func configVolumeAndMount(planner *sharePlanner) (
	corev1.Volume, corev1.VolumeMount) {
	// volume
	cmSrc := &corev1.ConfigMapVolumeSource{}
	cmSrc.Name = planner.instanceName()
	volume := corev1.Volume{
		Name: configMapName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: cmSrc,
		},
	}
	// mount
	mount := corev1.VolumeMount{
		MountPath: planner.containerConfigDir(),
		Name:      configMapName,
	}
	return volume, mount
}

func userConfigVolumeAndMount(planner *sharePlanner) (
	corev1.Volume, corev1.VolumeMount) {
	// volume
	uss := planner.userSecuritySource()
	volume := corev1.Volume{
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
	}
	// mount
	mount := corev1.VolumeMount{
		MountPath: planner.usersConfigDir(),
		Name:      userSecretVolName,
	}
	return volume, mount
}

func sambaStateVolumeAndMount(planner *sharePlanner) (
	corev1.Volume, corev1.VolumeMount) {
	// todo: should this use a persistent volume?
	// volume
	volume := corev1.Volume{
		Name: stateVolName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumDefault,
			},
		},
	}
	// mount
	mount := corev1.VolumeMount{
		MountPath: planner.sambaStateDir(),
		Name:      stateVolName,
	}
	return volume, mount
}

func osRunVolumeAndMount(planner *sharePlanner) (
	corev1.Volume, corev1.VolumeMount) {
	// volume
	volume := corev1.Volume{
		Name: osRunVolName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
		},
	}
	// mount
	mount := corev1.VolumeMount{
		MountPath: planner.osRunDir(),
		Name:      osRunVolName,
	}
	return volume, mount
}

func wbSocketsVolumeAndMount(planner *sharePlanner) (
	corev1.Volume, corev1.VolumeMount) {
	// volume
	volume := corev1.Volume{
		Name: wbSocketsVolName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
		},
	}
	// mount
	mount := corev1.VolumeMount{
		MountPath: planner.winbindSocketsDir(),
		Name:      wbSocketsVolName,
	}
	return volume, mount
}

func joinJSONFileVolumeAndMount(planner *sharePlanner, index int) (
	corev1.Volume, corev1.VolumeMount) {
	// volume
	vname := joinJSONVolName + planner.joinJSONSuffix(index)
	j := planner.SecurityConfig.Spec.JoinSources[index]
	volume := corev1.Volume{
		Name: vname,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: j.UserJoin.Secret,
				Items: []corev1.KeyToPath{{
					Key:  j.UserJoin.Key,
					Path: planner.joinJSONFileName(),
				}},
			},
		},
	}
	// mount
	mount := corev1.VolumeMount{
		MountPath: planner.joinJSONSourceDir(index),
		Name:      vname,
	}
	return volume, mount
}

func svcWatchVolumeAndMount(dir string) (
	corev1.Volume, corev1.VolumeMount) {
	// volume
	name := "svcwatch"
	volume := corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
		},
	}
	// mount
	mount := corev1.VolumeMount{
		MountPath: dir,
		Name:      name,
	}
	return volume, mount
}

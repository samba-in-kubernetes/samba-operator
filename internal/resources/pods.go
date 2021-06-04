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
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/samba-in-kubernetes/samba-operator/internal/conf"
)

const (
	userSecretVolName = "users-config"
	wbSocketsVolName  = "samba-wb-sockets-dir"
	stateVolName      = "samba-state-dir"
	osRunVolName      = "run"
	joinJsonVolName   = "join-data"
)

func buildPodSpec(planner *sharePlanner, cfg *conf.OperatorConfig, pvcName string) corev1.PodSpec {
	if planner.securityMode() == adMode {
		return buildADPodSpec(planner, cfg, pvcName)
	}
	return buildUserPodSpec(planner, cfg, pvcName)
}

func buildADPodSpec(planner *sharePlanner, cfg *conf.OperatorConfig, pvcName string) corev1.PodSpec {
	volumes := []corev1.Volume{}
	mounts := []corev1.VolumeMount{}

	configVol, configMount := configVolumeAndMount(planner)
	volumes = append(volumes, configVol)
	mounts = append(mounts, configMount)

	stateVol, stateMount := sambaStateVolumeAndMount(planner)
	volumes = append(volumes, stateVol)
	mounts = append(mounts, stateMount)

	// for smbd only
	shareVol, shareMount := shareVolumeAndMount(planner, pvcName)
	volumes = append(volumes, shareVol)

	// for smbd and winbind only (not init containers)
	wbSockVol, wbSockMount := wbSocketsVolumeAndMount(planner)
	volumes = append(volumes, wbSockVol)

	jsrc := getJoinSources(planner)
	joinEnv := []corev1.EnvVar{{
		Name:  "SAMBACC_JOIN_FILES",
		Value: planner.joinEnvPaths(jsrc.paths),
	}}
	volumes = append(volumes, jsrc.volumes...)

	podEnv := defaultPodEnv(planner)
	spn := true
	podSpec := corev1.PodSpec{
		Volumes: volumes,
		// we need to set ShareProcessNamespace to true.
		ShareProcessNamespace: &spn,
		InitContainers: []corev1.Container{
			{
				Image:        cfg.SmbdContainerImage,
				Name:         "init",
				Args:         []string{"init"},
				Env:          podEnv,
				VolumeMounts: mounts,
			},
			{
				Image:        cfg.SmbdContainerImage,
				Name:         "must-join",
				Args:         []string{"must-join"},
				Env:          append(podEnv, joinEnv...),
				VolumeMounts: append(mounts, jsrc.mounts...),
			},
		},
		Containers: []corev1.Container{
			{
				Image: cfg.SmbdContainerImage,
				Name:  cfg.SmbdContainerName,
				Args:  []string{"run", "smbd"},
				Env:   podEnv,
				Ports: []corev1.ContainerPort{{
					ContainerPort: 445,
					Name:          "smb",
				}},
				VolumeMounts: append(mounts, wbSockMount, shareMount),
				LivenessProbe: &corev1.Probe{
					Handler: corev1.Handler{
						TCPSocket: &corev1.TCPSocketAction{
							Port: intstr.FromInt(445),
						},
					},
				},
			},
			{
				Image:        cfg.SmbdContainerImage,
				Name:         "wb", //cfg.WinbindContainerName,
				Args:         []string{"run", "winbindd"},
				Env:          podEnv,
				VolumeMounts: append(mounts, wbSockMount),
				LivenessProbe: &corev1.Probe{
					Handler: corev1.Handler{
						Exec: &corev1.ExecAction{
							Command: []string{
								"samba-container",
								"check",
								"winbind",
							},
						},
					},
				},
			},
		},
	}
	return podSpec
}

func buildUserPodSpec(planner *sharePlanner, cfg *conf.OperatorConfig, pvcName string) corev1.PodSpec {
	volumes := []corev1.Volume{}
	mounts := []corev1.VolumeMount{}

	shareVol, shareMount := shareVolumeAndMount(planner, pvcName)
	volumes = append(volumes, shareVol)
	mounts = append(mounts, shareMount)

	configVol, configMount := configVolumeAndMount(planner)
	volumes = append(volumes, configVol)
	mounts = append(mounts, configMount)

	osRunVol, osRunMount := osRunVolumeAndMount(planner)
	volumes = append(volumes, osRunVol)
	mounts = append(mounts, osRunMount)

	if planner.securityMode() == userMode && planner.userSecuritySource().Configured {
		v, m := userConfigVolumeAndMount(planner)
		volumes = append(volumes, v)
		mounts = append(mounts, m)
	}
	podEnv := defaultPodEnv(planner)
	podSpec := corev1.PodSpec{
		Volumes: volumes,
		Containers: []corev1.Container{{
			Image: cfg.SmbdContainerImage,
			Name:  cfg.SmbdContainerName,
			Args:  []string{"run", "smbd"},
			Env:   podEnv,
			Ports: []corev1.ContainerPort{{
				ContainerPort: 445,
				Name:          "smb",
			}},
			VolumeMounts: mounts,
			LivenessProbe: &corev1.Probe{
				Handler: corev1.Handler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(445),
					},
				},
			},
		}},
	}
	return podSpec
}

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
	cmSrc.Name = ConfigMapName
	volume := corev1.Volume{
		Name: ConfigMapName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: cmSrc,
		},
	}
	// mount
	mount := corev1.VolumeMount{
		MountPath: planner.containerConfigDir(),
		Name:      ConfigMapName,
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

func joinJsonFileVolumeAndMount(planner *sharePlanner, index int) (
	corev1.Volume, corev1.VolumeMount) {
	// volume
	vname := joinJsonVolName + planner.joinJsonSuffix(index)
	j := planner.SecurityConfig.Spec.JoinSources[index]
	volume := corev1.Volume{
		Name: vname,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: j.UserJoin.Secret,
				Items: []corev1.KeyToPath{{
					Key:  j.UserJoin.Key,
					Path: planner.joinJsonFileName(),
				}},
			},
		},
	}
	// mount
	mount := corev1.VolumeMount{
		MountPath: planner.joinJsonSourceDir(index),
		Name:      vname,
	}
	return volume, mount
}

func defaultPodEnv(planner *sharePlanner) []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "SAMBA_CONTAINER_ID",
			Value: string(planner.instanceID()),
		},
		{
			Name:  "SAMBACC_CONFIG",
			Value: planner.containerConfigPath(),
		},
	}
}

type joinSources struct {
	volumes []corev1.Volume
	mounts  []corev1.VolumeMount
	paths   []string
}

func getJoinSources(planner *sharePlanner) joinSources {
	src := joinSources{
		volumes: []corev1.Volume{},
		mounts:  []corev1.VolumeMount{},
		paths:   []string{},
	}
	for i, js := range planner.SecurityConfig.Spec.JoinSources {
		if js.UserJoin != nil {
			v, m := joinJsonFileVolumeAndMount(planner, i)
			src.volumes = append(src.volumes, v)
			src.mounts = append(src.mounts, m)
			src.paths = append(src.paths, planner.joinJsonSourcePath(i))
		}
	}
	return src
}

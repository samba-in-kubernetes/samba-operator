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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/samba-in-kubernetes/samba-operator/internal/conf"
)

func buildPodSpec(
	planner *sharePlanner,
	cfg *conf.OperatorConfig,
	pvcName string) corev1.PodSpec {
	// ---
	if planner.securityMode() == adMode {
		return buildADPodSpec(planner, cfg, pvcName)
	}
	return buildUserPodSpec(planner, cfg, pvcName)
}

func buildADPodSpec(
	planner *sharePlanner,
	cfg *conf.OperatorConfig,
	pvcName string) corev1.PodSpec {
	// ---
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
	if planner.dnsRegister() != dnsRegisterNever {
		watchVol, watchMount := svcWatchVolumeAndMount(
			planner.serviceWatchStateDir(),
		)
		podSpec.Volumes = append(podSpec.Volumes, watchVol)
		podSpec.Containers = append(podSpec.Containers, corev1.Container{
			Image:        cfg.SmbdContainerImage,
			Name:         "dns-register",
			Args:         planner.dnsRegisterArgs(),
			Env:          podEnv,
			VolumeMounts: append(mounts, wbSockMount, watchMount),
		})
		serviceLabelSel := fmt.Sprintf("metadata.labels['%s']", svcSelectorKey)
		podSpec.Containers = append(podSpec.Containers, corev1.Container{
			Image: cfg.SvcWatchContainerImage,
			Name:  "svc-watch",
			Env: []corev1.EnvVar{
				{
					Name:  "DESTINATION_PATH",
					Value: planner.serviceWatchJSONPath(),
				},
				{
					Name:  "SERVICE_LABEL_KEY",
					Value: svcSelectorKey,
				},
				{
					Name: "SERVICE_LABEL_VALUE",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: serviceLabelSel,
						},
					},
				},
				{
					Name: "SERVICE_NAMESPACE",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.namespace",
						},
					},
				},
			},
			VolumeMounts: []corev1.VolumeMount{watchMount},
		})
	}
	return podSpec
}

func buildUserPodSpec(
	planner *sharePlanner,
	cfg *conf.OperatorConfig,
	pvcName string) corev1.PodSpec {
	// ---
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

func defaultPodEnv(planner *sharePlanner) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{
			Name:  "SAMBA_CONTAINER_ID",
			Value: string(planner.instanceID()),
		},
		{
			Name:  "SAMBACC_CONFIG",
			Value: planner.containerConfigPath(),
		},
	}
	// In the future we may want per-container debug levels. The
	// infrastructure could support that. For the moment we simply have one
	// debug level for all samba containers in the pod.
	if lvl := planner.sambaContainerDebugLevel(); lvl != "" {
		env = append(env, corev1.EnvVar{
			Name:  "SAMBA_DEBUG_LEVEL",
			Value: lvl,
		})
	}
	return env
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
			v, m := joinJSONFileVolumeAndMount(planner, i)
			src.volumes = append(src.volumes, v)
			src.mounts = append(src.mounts, m)
			src.paths = append(src.paths, planner.joinJSONSourcePath(i))
		}
	}
	return src
}

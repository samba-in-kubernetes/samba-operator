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

func buildClusteredPodSpec(
	planner *sharePlanner,
	dataPVCName, statePVCName string) corev1.PodSpec {
	// ---
	if planner.securityMode() == adMode {
		return buildClusteredADPodSpec(planner, dataPVCName, statePVCName)
	}
	return buildClusteredUserPodSpec(planner, dataPVCName, statePVCName)
}

func buildADPodSpec(
	planner *sharePlanner,
	_ *conf.OperatorConfig,
	pvcName string) corev1.PodSpec {
	// ---
	volumes := []volMount{}
	smbAllVols := []volMount{}

	configVol := configVolumeAndMount(planner)
	volumes = append(volumes, configVol)
	smbAllVols = append(smbAllVols, configVol)

	stateVol := sambaStateVolumeAndMount(planner)
	volumes = append(volumes, stateVol)
	smbAllVols = append(smbAllVols, stateVol)

	// for smb server containers (not init containers)
	wbSockVol := wbSocketsVolumeAndMount(planner)
	volumes = append(volumes, wbSockVol)
	smbServerVols := append(smbAllVols, wbSockVol)

	// for smbd only
	shareVol := shareVolumeAndMount(planner, pvcName)
	volumes = append(volumes, shareVol)
	smbdVols := append(smbServerVols, shareVol)

	jsrc := getJoinSources(planner)
	joinEnv := []corev1.EnvVar{{
		Name:  "SAMBACC_JOIN_FILES",
		Value: planner.joinEnvPaths(jsrc.paths),
	}}
	volumes = append(volumes, jsrc.volumes...)
	joinVols := append(smbAllVols, jsrc.volumes...)

	podEnv := defaultPodEnv(planner)
	joinEnv = append(
		podEnv,
		corev1.EnvVar{
			Name:  "SAMBACC_JOIN_FILES",
			Value: planner.joinEnvPaths(jsrc.paths),
		},
	)

	containers := []corev1.Container{
		buildSmbdCtr(planner, podEnv, smbdVols),
		buildWinbinddCtr(planner, podEnv, smbServerVols),
	}

	if planner.dnsRegister() != dnsRegisterNever {
		watchVol := svcWatchVolumeAndMount(
			planner.serviceWatchStateDir(),
		)
		volumes = append(volumes, watchVol)
		svcWatchVols := []volMount{watchVol}
		dnsRegVols := append(smbServerVols, watchVol)
		containers = append(
			containers,
			buildSvcWatchCtr(planner, svcWatchEnv(planner), svcWatchVols),
			buildDNSRegCtr(planner, podEnv, dnsRegVols),
		)
	}

	shareProcessNamespace := true
	podSpec := corev1.PodSpec{
		Volumes: getVolumes(volumes),
		// we need to set ShareProcessNamespace to true.
		ShareProcessNamespace: &shareProcessNamespace,
		InitContainers: []corev1.Container{
			buildInitCtr(planner, podEnv, smbAllVols),
			buildMustJoinCtr(planner, joinEnv, joinVols),
		},
		Containers: containers,
	}
	return podSpec
}

func buildUserPodSpec(
	planner *sharePlanner,
	_ *conf.OperatorConfig,
	pvcName string) corev1.PodSpec {
	// ---
	vols := []volMount{}

	shareVol := shareVolumeAndMount(planner, pvcName)
	vols = append(vols, shareVol)

	configVol := configVolumeAndMount(planner)
	vols = append(vols, configVol)

	osRunVol := osRunVolumeAndMount(planner)
	vols = append(vols, osRunVol)

	if planner.userSecuritySource().Configured {
		v := userConfigVolumeAndMount(planner)
		vols = append(vols, v)
	}
	podEnv := defaultPodEnv(planner)
	podSpec := corev1.PodSpec{
		Volumes: getVolumes(vols),
		Containers: []corev1.Container{
			buildSmbdCtr(planner, podEnv, vols),
		},
	}
	return podSpec
}

func buildClusteredUserPodSpec(
	planner *sharePlanner,
	dataPVCName, statePVCName string) corev1.PodSpec {
	// ---
	var (
		volumes        []volMount
		podCfgVols     []volMount
		initContainers []corev1.Container
		containers     []corev1.Container
	)

	shareVol := shareVolumeAndMount(planner, dataPVCName)
	volumes = append(volumes, shareVol)

	configVol := configVolumeAndMount(planner)
	volumes = append(volumes, configVol)
	podCfgVols = append(podCfgVols, configVol)

	stateVol := sambaStateVolumeAndMount(planner)
	volumes = append(volumes, stateVol)

	ctdbConfigVol := ctdbConfigVolumeAndMount(planner)
	volumes = append(volumes, ctdbConfigVol)

	ctdbPeristentVol := ctdbPersistentVolumeAndMount(planner)
	volumes = append(volumes, ctdbPeristentVol)

	ctdbVolatileVol := ctdbVolatileVolumeAndMount(planner)
	volumes = append(volumes, ctdbVolatileVol)

	ctdbSocketsVol := ctdbSocketsVolumeAndMount(planner)
	volumes = append(volumes, ctdbSocketsVol)

	ctdbSharedVol := ctdbSharedStateVolumeAndMount(planner, statePVCName)
	volumes = append(volumes, ctdbSharedVol)

	if planner.userSecuritySource().Configured {
		v := userConfigVolumeAndMount(planner)
		volumes = append(volumes, v)
		podCfgVols = append(podCfgVols, v)
	}

	podEnv := defaultPodEnv(planner)
	ctdbEnv := append(podEnv, ctdbHostnameEnv(planner)...)

	initContainers = append(
		initContainers,
		buildInitCtr(planner, podEnv, append(
			podCfgVols,
			stateVol,
			ctdbSharedVol, // needed to decide if real init or not
		)))

	initContainers = append(
		initContainers,
		buildCTDBMigrateCtr(planner, ctdbEnv, append(
			podCfgVols,
			stateVol,
			ctdbSharedVol,
			ctdbConfigVol,
			ctdbPeristentVol,
		)))

	ctdbInitVols := append(
		podCfgVols,
		stateVol,
		ctdbSharedVol,
		ctdbConfigVol,
	)
	initContainers = append(
		initContainers,
		buildCTDBSetNodeCtr(planner, ctdbEnv, ctdbInitVols),
		buildCTDBMustHaveNodeCtr(planner, ctdbEnv, ctdbInitVols),
	)

	ctdbdVols := append(
		podCfgVols,
		ctdbConfigVol,
		ctdbPeristentVol,
		ctdbVolatileVol,
		ctdbSocketsVol,
		ctdbSharedVol,
	)
	containers = append(
		containers,
		buildCTDBDaemonCtr(planner, ctdbEnv, ctdbdVols))

	ctdbManageNodesVols := append(
		podCfgVols,
		ctdbConfigVol,
		ctdbSocketsVol,
		ctdbSharedVol,
	)
	containers = append(
		containers,
		buildCTDBManageNodesCtr(planner, ctdbEnv, ctdbManageNodesVols))

	// smbd
	containers = append(
		containers,
		buildSmbdCtr(planner, podEnv, volumes))

	shareProcessNamespace := true
	podSpec := corev1.PodSpec{
		Volumes: getVolumes(volumes),
		// we need to set ShareProcessNamespace to true.
		ShareProcessNamespace: &shareProcessNamespace,
		InitContainers:        initContainers,
		Containers:            containers,
	}
	return podSpec
}

func buildClusteredADPodSpec(
	planner *sharePlanner,
	dataPVCName, statePVCName string) corev1.PodSpec {
	// ---
	var (
		volumes        []volMount
		podCfgVols     []volMount
		initContainers []corev1.Container
		containers     []corev1.Container
	)

	shareVol := shareVolumeAndMount(planner, dataPVCName)
	volumes = append(volumes, shareVol)

	configVol := configVolumeAndMount(planner)
	volumes = append(volumes, configVol)
	podCfgVols = append(podCfgVols, configVol)

	stateVol := sambaStateVolumeAndMount(planner)
	volumes = append(volumes, stateVol)

	ctdbConfigVol := ctdbConfigVolumeAndMount(planner)
	volumes = append(volumes, ctdbConfigVol)

	ctdbPeristentVol := ctdbPersistentVolumeAndMount(planner)
	volumes = append(volumes, ctdbPeristentVol)

	ctdbVolatileVol := ctdbVolatileVolumeAndMount(planner)
	volumes = append(volumes, ctdbVolatileVol)

	ctdbSocketsVol := ctdbSocketsVolumeAndMount(planner)
	volumes = append(volumes, ctdbSocketsVol)

	ctdbSharedVol := ctdbSharedStateVolumeAndMount(planner, statePVCName)
	volumes = append(volumes, ctdbSharedVol)

	// the winbind sockets volume is needed for winbind and clients (smbd)
	wbSockVol := wbSocketsVolumeAndMount(planner)
	volumes = append(volumes, wbSockVol)

	jsrc := getJoinSources(planner)
	joinEnv := []corev1.EnvVar{{
		Name:  "SAMBACC_JOIN_FILES",
		Value: planner.joinEnvPaths(jsrc.paths),
	}}
	volumes = append(volumes, jsrc.volumes...)

	podEnv := defaultPodEnv(planner)
	ctdbEnv := append(podEnv, ctdbHostnameEnv(planner)...)

	initContainers = append(
		initContainers,
		buildInitCtr(planner, podEnv, append(
			podCfgVols,
			stateVol,
			ctdbSharedVol, // needed to decide if real init or not
		)))

	joinVols := append(
		append(podCfgVols, stateVol, ctdbSharedVol),
		jsrc.volumes...)
	initContainers = append(
		initContainers,
		buildMustJoinCtr(planner, joinEnv, joinVols),
	)

	initContainers = append(
		initContainers,
		buildCTDBMigrateCtr(planner, ctdbEnv, append(
			podCfgVols,
			stateVol,
			ctdbSharedVol,
			ctdbConfigVol,
			ctdbPeristentVol,
		)))

	ctdbInitVols := append(
		podCfgVols,
		stateVol,
		ctdbSharedVol,
		ctdbConfigVol,
	)
	initContainers = append(
		initContainers,
		buildCTDBSetNodeCtr(planner, ctdbEnv, ctdbInitVols),
		buildCTDBMustHaveNodeCtr(planner, ctdbEnv, ctdbInitVols),
	)

	ctdbdVols := append(
		podCfgVols,
		ctdbConfigVol,
		ctdbPeristentVol,
		ctdbVolatileVol,
		ctdbSocketsVol,
		ctdbSharedVol,
	)
	containers = append(
		containers,
		buildCTDBDaemonCtr(planner, ctdbEnv, ctdbdVols))

	ctdbManageNodesVols := append(
		podCfgVols,
		ctdbConfigVol,
		ctdbSocketsVol,
		ctdbSharedVol,
	)
	containers = append(
		containers,
		buildCTDBManageNodesCtr(planner, ctdbEnv, ctdbManageNodesVols))

	// winbindd
	wbVols := append(
		podCfgVols,
		stateVol,
		wbSockVol,
		ctdbConfigVol,
		ctdbPeristentVol,
		ctdbVolatileVol,
		ctdbSocketsVol,
		ctdbSharedVol,
	)
	containers = append(
		containers,
		buildWinbinddCtr(planner, podEnv, wbVols))

	// smbd
	containers = append(
		containers,
		buildSmbdCtr(planner, podEnv, volumes))

	// dns-register containers
	if planner.dnsRegister() != dnsRegisterNever {
		watchVol := svcWatchVolumeAndMount(
			planner.serviceWatchStateDir(),
		)
		volumes = append(volumes, watchVol)
		svcWatchVols := []volMount{watchVol}
		dnsRegVols := append(wbVols, watchVol)
		containers = append(
			containers,
			buildSvcWatchCtr(planner, svcWatchEnv(planner), svcWatchVols),
			buildDNSRegCtr(planner, podEnv, dnsRegVols),
		)
	}

	shareProcessNamespace := true
	podSpec := corev1.PodSpec{
		Volumes: getVolumes(volumes),
		// we need to set ShareProcessNamespace to true.
		ShareProcessNamespace: &shareProcessNamespace,
		InitContainers:        initContainers,
		Containers:            containers,
	}
	return podSpec
}

func buildSmbdCtr(
	planner *sharePlanner,
	env []corev1.EnvVar,
	vols []volMount) corev1.Container {
	// ---
	return corev1.Container{
		Image: planner.GlobalConfig.SmbdContainerImage,
		Name:  planner.GlobalConfig.SmbdContainerName,
		Args:  planner.runDaemonArgs("smbd"),
		Env:   env,
		Ports: []corev1.ContainerPort{{
			ContainerPort: 445,
			Name:          "smb",
		}},
		VolumeMounts: getMounts(vols),
		ReadinessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(445),
				},
			},
		},
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(445),
				},
			},
		},
	}
}

func buildWinbinddCtr(
	planner *sharePlanner,
	env []corev1.EnvVar,
	vols []volMount) corev1.Container {
	// ---
	return corev1.Container{
		Image:        planner.GlobalConfig.SmbdContainerImage,
		Name:         planner.GlobalConfig.WinbindContainerName,
		Args:         planner.runDaemonArgs("winbindd"),
		Env:          env,
		VolumeMounts: getMounts(vols),
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
	}
}

func buildCTDBDaemonCtr(
	planner *sharePlanner,
	env []corev1.EnvVar,
	vols []volMount) corev1.Container {
	// ---
	return corev1.Container{
		Image:        planner.GlobalConfig.SmbdContainerImage,
		Name:         "ctdb",
		Args:         planner.ctdbDaemonArgs(),
		Env:          env,
		VolumeMounts: getMounts(vols),
	}
}

func buildCTDBManageNodesCtr(
	planner *sharePlanner,
	env []corev1.EnvVar,
	vols []volMount) corev1.Container {
	// ---
	return corev1.Container{
		Image:        planner.GlobalConfig.SmbdContainerImage,
		Name:         "ctdb-manage-nodes",
		Args:         planner.ctdbManageNodesArgs(),
		Env:          env,
		VolumeMounts: getMounts(vols),
	}
}

func buildDNSRegCtr(
	planner *sharePlanner,
	env []corev1.EnvVar,
	vols []volMount) corev1.Container {
	// ---
	return corev1.Container{
		Image:        planner.GlobalConfig.SmbdContainerImage,
		Name:         "dns-register",
		Args:         planner.dnsRegisterArgs(),
		Env:          env,
		VolumeMounts: getMounts(vols),
	}
}

func buildSvcWatchCtr(
	planner *sharePlanner,
	env []corev1.EnvVar,
	vols []volMount) corev1.Container {
	// ---
	return corev1.Container{
		Image:        planner.GlobalConfig.SvcWatchContainerImage,
		Name:         "svc-watch",
		Env:          env,
		VolumeMounts: getMounts(vols),
	}
}

func buildInitCtr(
	planner *sharePlanner,
	env []corev1.EnvVar,
	vols []volMount) corev1.Container {
	// ---
	return corev1.Container{
		Image:        planner.GlobalConfig.SmbdContainerImage,
		Name:         "init",
		Args:         planner.initializerArgs("init"),
		Env:          env,
		VolumeMounts: getMounts(vols),
	}
}

func buildMustJoinCtr(
	planner *sharePlanner,
	env []corev1.EnvVar,
	vols []volMount) corev1.Container {
	// ---
	return corev1.Container{
		Image:        planner.GlobalConfig.SmbdContainerImage,
		Name:         "must-join",
		Args:         planner.initializerArgs("must-join"),
		Env:          env,
		VolumeMounts: getMounts(vols),
	}
}

func buildCTDBMigrateCtr(
	planner *sharePlanner,
	env []corev1.EnvVar,
	vols []volMount) corev1.Container {
	// ---
	return corev1.Container{
		Image:        planner.GlobalConfig.SmbdContainerImage,
		Name:         "ctdb-migrate",
		Args:         planner.ctdbMigrateArgs(),
		Env:          env,
		VolumeMounts: getMounts(vols),
	}
}

func buildCTDBSetNodeCtr(
	planner *sharePlanner,
	env []corev1.EnvVar,
	vols []volMount) corev1.Container {
	// ---
	return corev1.Container{
		Image:        planner.GlobalConfig.SmbdContainerImage,
		Name:         "ctdb-set-node",
		Args:         planner.ctdbSetNodeArgs(),
		Env:          env,
		VolumeMounts: getMounts(vols),
	}
}

func buildCTDBMustHaveNodeCtr(
	planner *sharePlanner,
	env []corev1.EnvVar,
	vols []volMount) corev1.Container {
	// ---
	return corev1.Container{
		Image:        planner.GlobalConfig.SmbdContainerImage,
		Name:         "ctdb-must-have-node",
		Args:         planner.ctdbMustHaveNodeArgs(),
		Env:          env,
		VolumeMounts: getMounts(vols),
	}
}

func svcWatchEnv(planner *sharePlanner) []corev1.EnvVar {
	serviceLabelSel := fmt.Sprintf("metadata.labels['%s']", svcSelectorKey)
	return []corev1.EnvVar{
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
	}
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

func ctdbHostnameEnv(_ *sharePlanner) []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name: "HOSTNAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name:  "SAMBACC_CTDB",
			Value: "ctdb-is-experimental",
		},
	}
}

type joinSources struct {
	volumes []volMount
	paths   []string
}

func getJoinSources(planner *sharePlanner) joinSources {
	src := joinSources{
		volumes: []volMount{},
		paths:   []string{},
	}
	for i, js := range planner.SecurityConfig.Spec.JoinSources {
		if js.UserJoin != nil {
			vm := joinJSONFileVolumeAndMount(planner, i)
			src.volumes = append(src.volumes, vm)
			src.paths = append(src.paths, planner.joinJSONSourcePath(i))
		}
	}
	return src
}

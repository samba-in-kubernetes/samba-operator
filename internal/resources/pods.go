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
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/samba-in-kubernetes/samba-operator/internal/conf"
	pln "github.com/samba-in-kubernetes/samba-operator/internal/planner"
)

func buildPodSpec(
	planner *pln.Planner,
	cfg *conf.OperatorConfig,
	pvcName string) corev1.PodSpec {
	// ---
	if planner.SecurityMode() == pln.ADMode {
		return buildADPodSpec(planner, cfg, pvcName)
	}
	return buildUserPodSpec(planner, cfg, pvcName)
}

func buildClusteredPodSpec(
	planner *pln.Planner,
	dataPVCName, statePVCName string) corev1.PodSpec {
	// ---
	if planner.SecurityMode() == pln.ADMode {
		return buildClusteredADPodSpec(planner, dataPVCName, statePVCName)
	}
	return buildClusteredUserPodSpec(planner, dataPVCName, statePVCName)
}

func buildADPodSpec(
	planner *pln.Planner,
	_ *conf.OperatorConfig,
	pvcName string) corev1.PodSpec {
	// ---
	volumes := newVolKeeper()
	smbAllVols := newVolKeeper()

	configVol := configVolumeAndMount(planner)
	volumes.add(configVol)
	smbAllVols.add(configVol)

	stateVol := sambaStateVolumeAndMount(planner)
	volumes.add(stateVol)
	smbAllVols.add(stateVol)

	// for smb server containers (not init containers)
	wbSockVol := wbSocketsVolumeAndMount(planner)
	volumes.add(wbSockVol)
	smbServerVols := smbAllVols.clone().add(wbSockVol)

	// for smbd only
	shareVol := shareVolumeAndMount(planner, pvcName)
	volumes.add(shareVol)
	smbdVols := smbServerVols.clone().add(shareVol)

	jsrc := getJoinSources(planner)
	volumes.extend(jsrc.volumes)
	joinVols := smbAllVols.clone().extend(jsrc.volumes)

	podEnv := defaultPodEnv(planner)
	// nolint:gocritic
	joinEnv := append(
		podEnv,
		corev1.EnvVar{
			Name:  "SAMBACC_JOIN_FILES",
			Value: joinEnvPaths(jsrc.paths),
		},
	)

	containers := buildSmbdCtrs(planner, podEnv, smbdVols.all())
	containers = append(containers,
		buildWinbinddCtr(planner, podEnv, smbServerVols.all()))

	if planner.DNSRegister() != pln.DNSRegisterNever {
		watchVol := svcWatchVolumeAndMount(
			planner.Paths().ServiceWatchStateDir(),
		)
		volumes.add(watchVol)
		svcWatchVols := newVolKeeper().add(watchVol)
		dnsRegVols := smbServerVols.clone().add(watchVol)
		containers = append(
			containers,
			buildSvcWatchCtr(planner, svcWatchEnv(planner), svcWatchVols.all()),
			buildDNSRegCtr(planner, podEnv, dnsRegVols.all()),
		)
	}

	podSpec := defaultPodSpec(planner)
	podSpec.Volumes = getVolumes(volumes.all())
	podSpec.InitContainers = []corev1.Container{
		buildInitCtr(planner, podEnv, smbAllVols.all()),
		buildEnsureShareCtr(planner, podEnv, smbdVols.all()),
		buildMustJoinCtr(planner, joinEnv, joinVols.all()),
	}
	podSpec.Containers = containers
	return podSpec
}

func buildUserPodSpec(
	planner *pln.Planner,
	_ *conf.OperatorConfig,
	pvcName string) corev1.PodSpec {
	// ---
	vols := newVolKeeper()
	initContainers := []corev1.Container{}

	shareVol := shareVolumeAndMount(planner, pvcName)
	vols.add(shareVol)

	stateVol := sambaStateVolumeAndMount(planner)
	vols.add(stateVol)

	configVol := configVolumeAndMount(planner)
	vols.add(configVol)

	podEnv := defaultPodEnv(planner)
	initContainers = append(initContainers,
		buildEnsureShareCtr(planner, podEnv, vols.all()))

	osRunVol := osRunVolumeAndMount(planner)
	vols.add(osRunVol)

	if planner.UserSecuritySource().Configured {
		v := userConfigVolumeAndMount(planner)
		vols.add(v)
	}
	podSpec := defaultPodSpec(planner)
	podSpec.Volumes = getVolumes(vols.all())
	podSpec.Containers = buildSmbdCtrs(planner, podEnv, vols.all())
	podSpec.InitContainers = initContainers
	return podSpec
}

func buildClusteredUserPodSpec(
	planner *pln.Planner,
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

	if planner.UserSecuritySource().Configured {
		v := userConfigVolumeAndMount(planner)
		volumes = append(volumes, v)
		podCfgVols = append(podCfgVols, v)
	}

	podEnv := defaultPodEnv(planner)
	// nolint:gocritic
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
		buildEnsureShareCtr(planner, podEnv, append(
			podCfgVols,
			stateVol,
			shareVol,
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

	ctdbInitVols := dupVolMounts(podCfgVols)
	ctdbInitVols = append(
		ctdbInitVols,
		stateVol,
		ctdbSharedVol,
		ctdbConfigVol,
	)
	initContainers = append(
		initContainers,
		buildCTDBSetNodeCtr(planner, ctdbEnv, ctdbInitVols),
		buildCTDBMustHaveNodeCtr(planner, ctdbEnv, ctdbInitVols),
	)

	ctdbdVols := dupVolMounts(podCfgVols)
	ctdbdVols = append(
		ctdbdVols,
		ctdbConfigVol,
		ctdbPeristentVol,
		ctdbVolatileVol,
		ctdbSocketsVol,
		ctdbSharedVol,
	)
	containers = append(
		containers,
		buildCTDBDaemonCtr(planner, ctdbEnv, ctdbdVols))

	ctdbManageNodesVols := dupVolMounts(podCfgVols)
	ctdbManageNodesVols = append(
		ctdbManageNodesVols,
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
		buildSmbdCtrs(planner, podEnv, volumes)...)

	podSpec := defaultPodSpec(planner)
	podSpec.Volumes = getVolumes(volumes)
	podSpec.InitContainers = initContainers
	podSpec.Containers = containers
	return podSpec
}

func buildClusteredADPodSpec(
	planner *pln.Planner,
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
		Value: joinEnvPaths(jsrc.paths),
	}}
	volumes = append(volumes, jsrc.volumes...)

	podEnv := defaultPodEnv(planner)
	// nolint:gocritic
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
		buildEnsureShareCtr(planner, podEnv, append(
			podCfgVols,
			stateVol,
			shareVol,
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

	ctdbInitVols := dupVolMounts(podCfgVols)
	ctdbInitVols = append(
		ctdbInitVols,
		stateVol,
		ctdbSharedVol,
		ctdbConfigVol,
	)
	initContainers = append(
		initContainers,
		buildCTDBSetNodeCtr(planner, ctdbEnv, ctdbInitVols),
		buildCTDBMustHaveNodeCtr(planner, ctdbEnv, ctdbInitVols),
	)

	ctdbdVols := dupVolMounts(podCfgVols)
	ctdbdVols = append(
		ctdbdVols,
		ctdbConfigVol,
		ctdbPeristentVol,
		ctdbVolatileVol,
		ctdbSocketsVol,
		ctdbSharedVol,
	)
	containers = append(
		containers,
		buildCTDBDaemonCtr(planner, ctdbEnv, ctdbdVols))

	ctdbManageNodesVols := dupVolMounts(podCfgVols)
	ctdbManageNodesVols = append(
		ctdbManageNodesVols,
		ctdbConfigVol,
		ctdbSocketsVol,
		ctdbSharedVol,
	)
	containers = append(
		containers,
		buildCTDBManageNodesCtr(planner, ctdbEnv, ctdbManageNodesVols))

	// winbindd
	wbVols := dupVolMounts(podCfgVols)
	wbVols = append(
		wbVols,
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
		buildSmbdCtrs(planner, podEnv, volumes)...)

	// dns-register containers
	if planner.DNSRegister() != pln.DNSRegisterNever {
		watchVol := svcWatchVolumeAndMount(
			planner.Paths().ServiceWatchStateDir(),
		)
		volumes = append(volumes, watchVol)
		svcWatchVols := []volMount{watchVol}
		dnsRegVols := dupVolMounts(wbVols)
		dnsRegVols = append(dnsRegVols, watchVol)
		containers = append(
			containers,
			buildSvcWatchCtr(planner, svcWatchEnv(planner), svcWatchVols),
			buildDNSRegCtr(planner, podEnv, dnsRegVols),
		)
	}

	podSpec := defaultPodSpec(planner)
	podSpec.Volumes = getVolumes(volumes)
	podSpec.InitContainers = initContainers
	podSpec.Containers = containers
	return podSpec
}

func buildSmbdCtrs(
	planner *pln.Planner,
	env []corev1.EnvVar,
	vols []volMount) []corev1.Container {
	// ---
	ctrs := []corev1.Container{}
	ctrs = append(ctrs, buildSmbdCtr(planner, env, vols))
	if withMetricsExporter(planner.GlobalConfig) {
		ctrs = append(ctrs, buildSmbdMetricsCtr(planner, metaPodEnv(), vols))
	}
	return ctrs
}

func buildSmbdCtr(
	planner *pln.Planner,
	env []corev1.EnvVar,
	vols []volMount) corev1.Container {
	// ---
	portnum := planner.GlobalConfig.SmbdPort
	return corev1.Container{
		Image:   planner.GlobalConfig.SmbdContainerImage,
		Name:    planner.GlobalConfig.SmbdContainerName,
		Command: []string{"samba-container"},
		Args:    planner.Args().Run("smbd"),
		Env:     env,
		Ports: []corev1.ContainerPort{{
			ContainerPort: int32(portnum),
			Name:          "smb",
		}},
		VolumeMounts: getMounts(vols),
		ReadinessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(portnum),
				},
			},
		},
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(portnum),
				},
			},
		},
	}
}

func buildSmbdMetricsCtr(
	planner *pln.Planner,
	env []corev1.EnvVar,
	vols []volMount) corev1.Container {
	// ---
	return buildSmbMetricsContainer(
		planner.GlobalConfig.SmbdMetricsContainerImage, env, getMounts(vols))
}

func buildWinbinddCtr(
	planner *pln.Planner,
	env []corev1.EnvVar,
	vols []volMount) corev1.Container {
	// ---
	return corev1.Container{
		Image:        planner.GlobalConfig.SmbdContainerImage,
		Name:         planner.GlobalConfig.WinbindContainerName,
		Args:         planner.Args().Run("winbindd"),
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
	planner *pln.Planner,
	env []corev1.EnvVar,
	vols []volMount) corev1.Container {
	// ---
	return corev1.Container{
		Image:        planner.GlobalConfig.SmbdContainerImage,
		Name:         "ctdb",
		Args:         planner.Args().CTDBDaemon(),
		Env:          env,
		VolumeMounts: getMounts(vols),
		ReadinessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				Exec: &corev1.ExecAction{
					Command: planner.Args().CTDBNodeStatus(),
				},
			},
		},
	}
}

func buildCTDBManageNodesCtr(
	planner *pln.Planner,
	env []corev1.EnvVar,
	vols []volMount) corev1.Container {
	// ---
	return corev1.Container{
		Image:        planner.GlobalConfig.SmbdContainerImage,
		Name:         "ctdb-manage-nodes",
		Args:         planner.Args().CTDBManageNodes(),
		Env:          env,
		VolumeMounts: getMounts(vols),
	}
}

func buildDNSRegCtr(
	planner *pln.Planner,
	env []corev1.EnvVar,
	vols []volMount) corev1.Container {
	// ---
	return corev1.Container{
		Image:        planner.GlobalConfig.SmbdContainerImage,
		Name:         "dns-register",
		Args:         planner.Args().DNSRegister(),
		Env:          env,
		VolumeMounts: getMounts(vols),
	}
}

func buildSvcWatchCtr(
	planner *pln.Planner,
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
	planner *pln.Planner,
	env []corev1.EnvVar,
	vols []volMount) corev1.Container {
	// ---
	return corev1.Container{
		Image:        planner.GlobalConfig.SmbdContainerImage,
		Name:         "init",
		Args:         planner.Args().Initializer("init"),
		Env:          env,
		VolumeMounts: getMounts(vols),
	}
}

func buildEnsureShareCtr(
	planner *pln.Planner,
	env []corev1.EnvVar,
	vols []volMount) corev1.Container {
	// ---
	return corev1.Container{
		Image:        planner.GlobalConfig.SmbdContainerImage,
		Name:         "ensure-share-paths",
		Args:         planner.Args().EnsureSharePaths(),
		Env:          env,
		VolumeMounts: getMounts(vols),
	}
}

func buildMustJoinCtr(
	planner *pln.Planner,
	env []corev1.EnvVar,
	vols []volMount) corev1.Container {
	// ---
	return corev1.Container{
		Image:        planner.GlobalConfig.SmbdContainerImage,
		Name:         "must-join",
		Args:         planner.Args().Initializer("must-join"),
		Env:          env,
		VolumeMounts: getMounts(vols),
	}
}

func buildCTDBMigrateCtr(
	planner *pln.Planner,
	env []corev1.EnvVar,
	vols []volMount) corev1.Container {
	// ---
	return corev1.Container{
		Image:        planner.GlobalConfig.SmbdContainerImage,
		Name:         "ctdb-migrate",
		Args:         planner.Args().CTDBMigrate(),
		Env:          env,
		VolumeMounts: getMounts(vols),
	}
}

func buildCTDBSetNodeCtr(
	planner *pln.Planner,
	env []corev1.EnvVar,
	vols []volMount) corev1.Container {
	// ---
	return corev1.Container{
		Image:        planner.GlobalConfig.SmbdContainerImage,
		Name:         "ctdb-set-node",
		Args:         planner.Args().CTDBSetNode(),
		Env:          env,
		VolumeMounts: getMounts(vols),
	}
}

func buildCTDBMustHaveNodeCtr(
	planner *pln.Planner,
	env []corev1.EnvVar,
	vols []volMount) corev1.Container {
	// ---
	return corev1.Container{
		Image:        planner.GlobalConfig.SmbdContainerImage,
		Name:         "ctdb-must-have-node",
		Args:         planner.Args().CTDBMustHaveNode(),
		Env:          env,
		VolumeMounts: getMounts(vols),
	}
}

func svcWatchEnv(planner *pln.Planner) []corev1.EnvVar {
	serviceLabelSel := fmt.Sprintf("metadata.labels['%s']", svcSelectorKey)
	return []corev1.EnvVar{
		{
			Name:  "DESTINATION_PATH",
			Value: planner.Paths().ServiceWatchJSON(),
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

func defaultPodEnv(planner *pln.Planner) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{
			Name:  "SAMBA_CONTAINER_ID",
			Value: planner.InstanceName(),
		},
		{
			Name:  "SAMBACC_CONFIG",
			Value: joinEnvPaths(planner.Paths().ContainerConfigs()),
		},
	}
	// In the future we may want per-container debug levels. The
	// infrastructure could support that. For the moment we simply have one
	// debug level for all samba containers in the pod.
	if lvl := planner.SambaContainerDebugLevel(); lvl != "" {
		env = append(env, corev1.EnvVar{
			Name:  "SAMBA_DEBUG_LEVEL",
			Value: lvl,
		})
	}
	env = append(env, metaPodEnv()...)
	return env
}

func metaPodEnv() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name: "SAMBA_POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: "SAMBA_POD_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
	}
}

func defaultPodSpec(planner *pln.Planner) corev1.PodSpec {
	shareProcessNamespace := true
	return corev1.PodSpec{
		ServiceAccountName:    planner.GlobalConfig.ServiceAccountName,
		ShareProcessNamespace: &shareProcessNamespace,
	}
}

func ctdbHostnameEnv(_ *pln.Planner) []corev1.EnvVar {
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

func getJoinSources(planner *pln.Planner) joinSources {
	src := joinSources{
		volumes: []volMount{},
		paths:   []string{},
	}
	for i, js := range planner.SecurityConfig.Spec.JoinSources {
		if js.UserJoin != nil {
			vm := joinJSONFileVolumeAndMount(planner, i)
			src.volumes = append(src.volumes, vm)
			src.paths = append(src.paths, planner.Paths().JoinJSONSource(i))
		}
	}
	return src
}

func joinEnvPaths(p []string) string {
	return strings.Join(p, ":")
}

func dupVolMounts(vols []volMount) []volMount {
	return append(make([]volMount, 0, len(vols)), vols...)
}

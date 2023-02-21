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
	smbInitVols := newVolKeeper()

	configVol := configVolumeAndMount(planner)
	volumes.add(configVol)
	smbInitVols.add(configVol)

	stateVol := sambaStateVolumeAndMount(planner)
	volumes.add(stateVol)
	smbInitVols.add(stateVol)

	// for smb server containers (not init containers)
	wbSockVol := wbSocketsVolumeAndMount(planner)
	volumes.add(wbSockVol)
	smbServerVols := smbInitVols.clone().add(wbSockVol)

	// for smbd only
	shareVol := shareVolumeAndMount(planner, pvcName)
	volumes.add(shareVol)
	smbdVols := smbServerVols.clone().add(shareVol)

	jsrc := getJoinSources(planner)
	volumes.extend(jsrc.volumes)
	joinVols := smbInitVols.clone().extend(jsrc.volumes)

	podEnv := defaultPodEnv(planner)
	// nolint:gocritic
	joinEnv := append(
		podEnv,
		corev1.EnvVar{
			Name:  "SAMBACC_JOIN_FILES",
			Value: joinEnvPaths(jsrc.paths),
		},
	)

	containers := buildSmbdCtrs(planner, podEnv, smbdVols)
	containers = append(containers,
		buildWinbinddCtr(planner, podEnv, smbServerVols))

	if planner.DNSRegister() != pln.DNSRegisterNever {
		watchVol := svcWatchVolumeAndMount(
			planner.Paths().ServiceWatchStateDir(),
		)
		volumes.add(watchVol)
		svcWatchVols := newVolKeeper().add(watchVol)
		dnsRegVols := smbServerVols.clone().add(watchVol)
		containers = append(
			containers,
			buildSvcWatchCtr(planner, svcWatchEnv(planner), svcWatchVols),
			buildDNSRegCtr(planner, podEnv, dnsRegVols),
		)
	}

	podSpec := defaultPodSpec(planner)
	podSpec.Volumes = getVolumes(volumes.all())
	podSpec.InitContainers = []corev1.Container{
		buildInitCtr(planner, podEnv, smbInitVols),
		buildEnsureShareCtr(planner, podEnv, smbdVols),
		buildMustJoinCtr(planner, joinEnv, joinVols),
	}
	podSpec.Containers = containers
	// we have no logger to log the json syntax error to. have to ignore it for now
	podSpec.NodeSelector, _ = planner.NodeSelector()
	return podSpec
}

func buildUserPodSpec(
	planner *pln.Planner,
	_ *conf.OperatorConfig,
	pvcName string) corev1.PodSpec {
	// ---
	volumes := newVolKeeper()
	initContainers := []corev1.Container{}

	shareVol := shareVolumeAndMount(planner, pvcName)
	volumes.add(shareVol)

	stateVol := sambaStateVolumeAndMount(planner)
	volumes.add(stateVol)

	configVol := configVolumeAndMount(planner)
	volumes.add(configVol)

	podEnv := defaultPodEnv(planner)
	initContainers = append(initContainers,
		buildEnsureShareCtr(planner, podEnv, volumes))

	osRunVol := osRunVolumeAndMount(planner)
	volumes.add(osRunVol)

	if planner.UserSecuritySource().Configured {
		v := userConfigVolumeAndMount(planner)
		volumes.add(v)
	}
	podSpec := defaultPodSpec(planner)
	podSpec.Volumes = getVolumes(volumes.all())
	podSpec.Containers = buildSmbdCtrs(planner, podEnv, volumes)
	podSpec.InitContainers = initContainers
	// we have no logger to log the json syntax error to. have to ignore it for now
	podSpec.NodeSelector, _ = planner.NodeSelector()
	return podSpec
}

func buildClusteredUserPodSpec(
	planner *pln.Planner,
	dataPVCName, statePVCName string) corev1.PodSpec {
	// ---
	var (
		volumes        = newVolKeeper()
		podCfgVols     = newVolKeeper()
		initContainers []corev1.Container
		containers     []corev1.Container
	)

	shareVol := shareVolumeAndMount(planner, dataPVCName)
	volumes.add(shareVol)

	configVol := configVolumeAndMount(planner)
	volumes.add(configVol)
	podCfgVols.add(configVol)

	stateVol := sambaStateVolumeAndMount(planner)
	volumes.add(stateVol)

	ctdbConfigVol := ctdbConfigVolumeAndMount(planner)
	volumes.add(ctdbConfigVol)

	ctdbPeristentVol := ctdbPersistentVolumeAndMount(planner)
	volumes.add(ctdbPeristentVol)

	ctdbVolatileVol := ctdbVolatileVolumeAndMount(planner)
	volumes.add(ctdbVolatileVol)

	ctdbSocketsVol := ctdbSocketsVolumeAndMount(planner)
	volumes.add(ctdbSocketsVol)

	ctdbSharedVol := ctdbSharedStateVolumeAndMount(planner, statePVCName)
	volumes.add(ctdbSharedVol)

	if planner.UserSecuritySource().Configured {
		v := userConfigVolumeAndMount(planner)
		volumes.add(v)
		podCfgVols.add(v)
	}

	podEnv := defaultPodEnv(planner)
	// nolint:gocritic
	ctdbEnv := append(podEnv, ctdbHostnameEnv(planner)...)

	initContainers = append(
		initContainers,
		buildInitCtr(
			planner,
			podEnv,
			// ctdbSharedVol is needed to decide if real init or not
			podCfgVols.clone().add(stateVol).add(ctdbSharedVol),
		))

	initContainers = append(
		initContainers,
		buildEnsureShareCtr(
			planner,
			podEnv,
			podCfgVols.clone().add(stateVol).add(shareVol),
		))

	ctdbMigrateVols := podCfgVols.clone().
		add(stateVol).
		add(ctdbSharedVol).
		add(ctdbConfigVol).
		add(ctdbPeristentVol)
	initContainers = append(
		initContainers,
		buildCTDBMigrateCtr(planner, ctdbEnv, ctdbMigrateVols),
	)

	ctdbInitVols := podCfgVols.clone().
		add(stateVol).
		add(ctdbSharedVol).
		add(ctdbConfigVol)
	initContainers = append(
		initContainers,
		buildCTDBSetNodeCtr(planner, ctdbEnv, ctdbInitVols),
		buildCTDBMustHaveNodeCtr(planner, ctdbEnv, ctdbInitVols),
	)

	ctdbdVols := podCfgVols.clone().
		add(ctdbConfigVol).
		add(ctdbPeristentVol).
		add(ctdbVolatileVol).
		add(ctdbSocketsVol).
		add(ctdbSharedVol)
	containers = append(
		containers,
		buildCTDBDaemonCtr(planner, ctdbEnv, ctdbdVols))

	ctdbManageNodesVols := podCfgVols.clone().
		add(ctdbConfigVol).
		add(ctdbSocketsVol).
		add(ctdbSharedVol)
	containers = append(
		containers,
		buildCTDBManageNodesCtr(planner, ctdbEnv, ctdbManageNodesVols))

	// smbd
	containers = append(
		containers,
		buildSmbdCtrs(planner, podEnv, volumes)...)

	podSpec := defaultPodSpec(planner)
	podSpec.Volumes = getVolumes(volumes.all())
	podSpec.InitContainers = initContainers
	podSpec.Containers = containers
	// we have no logger to log the json syntax error to. have to ignore it for now
	podSpec.NodeSelector, _ = planner.NodeSelector()
	return podSpec
}

func buildClusteredADPodSpec(
	planner *pln.Planner,
	dataPVCName, statePVCName string) corev1.PodSpec {
	// ---
	var (
		volumes        = newVolKeeper()
		podCfgVols     = newVolKeeper()
		initContainers []corev1.Container
		containers     []corev1.Container
	)

	shareVol := shareVolumeAndMount(planner, dataPVCName)
	volumes.add(shareVol)

	configVol := configVolumeAndMount(planner)
	volumes.add(configVol)
	podCfgVols.add(configVol)

	stateVol := sambaStateVolumeAndMount(planner)
	volumes.add(stateVol)

	ctdbConfigVol := ctdbConfigVolumeAndMount(planner)
	volumes.add(ctdbConfigVol)

	ctdbPeristentVol := ctdbPersistentVolumeAndMount(planner)
	volumes.add(ctdbPeristentVol)

	ctdbVolatileVol := ctdbVolatileVolumeAndMount(planner)
	volumes.add(ctdbVolatileVol)

	ctdbSocketsVol := ctdbSocketsVolumeAndMount(planner)
	volumes.add(ctdbSocketsVol)

	ctdbSharedVol := ctdbSharedStateVolumeAndMount(planner, statePVCName)
	volumes.add(ctdbSharedVol)

	// the winbind sockets volume is needed for winbind and clients (smbd)
	wbSockVol := wbSocketsVolumeAndMount(planner)
	volumes.add(wbSockVol)

	jsrc := getJoinSources(planner)
	joinEnv := []corev1.EnvVar{{
		Name:  "SAMBACC_JOIN_FILES",
		Value: joinEnvPaths(jsrc.paths),
	}}
	volumes.extend(jsrc.volumes)

	podEnv := defaultPodEnv(planner)
	// nolint:gocritic
	ctdbEnv := append(podEnv, ctdbHostnameEnv(planner)...)

	initContainers = append(
		initContainers,
		buildInitCtr(
			planner,
			podEnv,
			// ctdbSharedVol is needed to decide if real init or not
			podCfgVols.clone().add(stateVol).add(ctdbSharedVol),
		))

	initContainers = append(
		initContainers,
		buildEnsureShareCtr(
			planner,
			podEnv,
			podCfgVols.clone().add(stateVol).add(shareVol),
		))

	joinVols := podCfgVols.clone().
		add(stateVol).
		add(ctdbSharedVol).
		extend(jsrc.volumes)
	initContainers = append(
		initContainers,
		buildMustJoinCtr(planner, joinEnv, joinVols),
	)

	ctdbMigrateVols := podCfgVols.clone().
		add(stateVol).
		add(ctdbSharedVol).
		add(ctdbConfigVol).
		add(ctdbPeristentVol)
	initContainers = append(
		initContainers,
		buildCTDBMigrateCtr(planner, ctdbEnv, ctdbMigrateVols),
	)

	ctdbInitVols := podCfgVols.clone().
		add(stateVol).
		add(ctdbSharedVol).
		add(ctdbConfigVol)
	initContainers = append(
		initContainers,
		buildCTDBSetNodeCtr(planner, ctdbEnv, ctdbInitVols),
		buildCTDBMustHaveNodeCtr(planner, ctdbEnv, ctdbInitVols),
	)

	ctdbdVols := podCfgVols.clone().
		add(ctdbConfigVol).
		add(ctdbPeristentVol).
		add(ctdbVolatileVol).
		add(ctdbSocketsVol).
		add(ctdbSharedVol)
	containers = append(
		containers,
		buildCTDBDaemonCtr(planner, ctdbEnv, ctdbdVols))

	ctdbManageNodesVols := podCfgVols.clone().
		add(ctdbConfigVol).
		add(ctdbSocketsVol).
		add(ctdbSharedVol)
	containers = append(
		containers,
		buildCTDBManageNodesCtr(planner, ctdbEnv, ctdbManageNodesVols))

	// winbindd
	wbVols := podCfgVols.clone().
		add(stateVol).
		add(wbSockVol).
		add(ctdbConfigVol).
		add(ctdbPeristentVol).
		add(ctdbVolatileVol).
		add(ctdbSocketsVol).
		add(ctdbSharedVol)
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
		volumes.add(watchVol)
		svcWatchVols := newVolKeeper().add(watchVol)
		dnsRegVols := wbVols.clone().add(watchVol)
		containers = append(
			containers,
			buildSvcWatchCtr(planner, svcWatchEnv(planner), svcWatchVols),
			buildDNSRegCtr(planner, podEnv, dnsRegVols),
		)
	}

	podSpec := defaultPodSpec(planner)
	podSpec.Volumes = getVolumes(volumes.all())
	podSpec.InitContainers = initContainers
	podSpec.Containers = containers
	// we have no logger to log the json syntax error to. have to ignore it for now
	podSpec.NodeSelector, _ = planner.NodeSelector()
	return podSpec
}

func buildSmbdCtrs(
	planner *pln.Planner,
	env []corev1.EnvVar,
	vols *volKeeper) []corev1.Container {
	// ---
	ctrs := []corev1.Container{}
	ctrs = append(ctrs, buildSmbdCtr(planner, env, vols))
	metaOnlyVols := vols.exclude(tagData)
	// Insert a container to watch for changes to the config json
	// and apply those changes to samba.
	// We need to pass data vols to the config update container as it is
	// responsible for creating missing dirs and setting perms when new shares
	// appear.
	ctrs = append(ctrs, buildUpdateConfigWatchCtr(
		planner, env, vols))
	if withMetricsExporter(planner.GlobalConfig) {
		ctrs = append(ctrs, buildSmbdMetricsCtr(
			planner, metaPodEnv(), metaOnlyVols))
	}
	return ctrs
}

func buildSmbdCtr(
	planner *pln.Planner,
	env []corev1.EnvVar,
	vols *volKeeper) corev1.Container {
	// ---
	portnum := planner.GlobalConfig.SmbdPort
	mounts := getMounts(vols.all())
	return corev1.Container{
		Image:           planner.GlobalConfig.SmbdContainerImage,
		ImagePullPolicy: imagePullPolicy(planner),
		Name:            planner.GlobalConfig.SmbdContainerName,
		Command:         []string{"samba-container"},
		Args:            planner.Args().Run("smbd"),
		Env:             env,
		Ports: []corev1.ContainerPort{{
			ContainerPort: int32(portnum),
			Name:          "smb",
		}},
		VolumeMounts: mounts,
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
	vols *volKeeper) corev1.Container {
	// ---
	mounts := getMounts(vols.all())
	return buildSmbMetricsContainer(
		planner.GlobalConfig.SmbdMetricsContainerImage, env, mounts)
}

func buildWinbinddCtr(
	planner *pln.Planner,
	env []corev1.EnvVar,
	vols *volKeeper) corev1.Container {
	// ---
	mounts := getMounts(vols.all())
	return corev1.Container{
		Image:           planner.GlobalConfig.SmbdContainerImage,
		ImagePullPolicy: imagePullPolicy(planner),
		Name:            planner.GlobalConfig.WinbindContainerName,
		Args:            planner.Args().Run("winbindd"),
		Env:             env,
		VolumeMounts:    mounts,
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
	vols *volKeeper) corev1.Container {
	// ---
	mounts := getMounts(vols.all())
	return corev1.Container{
		Image:           planner.GlobalConfig.SmbdContainerImage,
		ImagePullPolicy: imagePullPolicy(planner),
		Name:            "ctdb",
		Args:            planner.Args().CTDBDaemon(),
		Env:             env,
		VolumeMounts:    mounts,
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
	vols *volKeeper) corev1.Container {
	// ---
	mounts := getMounts(vols.all())
	return corev1.Container{
		Image:           planner.GlobalConfig.SmbdContainerImage,
		ImagePullPolicy: imagePullPolicy(planner),
		Name:            "ctdb-manage-nodes",
		Args:            planner.Args().CTDBManageNodes(),
		Env:             env,
		VolumeMounts:    mounts,
	}
}

func buildDNSRegCtr(
	planner *pln.Planner,
	env []corev1.EnvVar,
	vols *volKeeper) corev1.Container {
	// ---
	mounts := getMounts(vols.all())
	return corev1.Container{
		Image:           planner.GlobalConfig.SmbdContainerImage,
		ImagePullPolicy: imagePullPolicy(planner),
		Name:            "dns-register",
		Args:            planner.Args().DNSRegister(),
		Env:             env,
		VolumeMounts:    mounts,
	}
}

func buildSvcWatchCtr(
	planner *pln.Planner,
	env []corev1.EnvVar,
	vols *volKeeper) corev1.Container {
	// ---
	mounts := getMounts(vols.all())
	return corev1.Container{
		Image:           planner.GlobalConfig.SvcWatchContainerImage,
		ImagePullPolicy: imagePullPolicy(planner),
		Name:            "svc-watch",
		Env:             env,
		VolumeMounts:    mounts,
	}
}

func buildInitCtr(
	planner *pln.Planner,
	env []corev1.EnvVar,
	vols *volKeeper) corev1.Container {
	// ---
	mounts := getMounts(vols.all())
	return corev1.Container{
		Image:           planner.GlobalConfig.SmbdContainerImage,
		ImagePullPolicy: imagePullPolicy(planner),
		Name:            "init",
		Args:            planner.Args().Initializer("init"),
		Env:             env,
		VolumeMounts:    mounts,
	}
}

func buildEnsureShareCtr(
	planner *pln.Planner,
	env []corev1.EnvVar,
	vols *volKeeper) corev1.Container {
	// ---
	mounts := getMounts(vols.all())
	return corev1.Container{
		Image:           planner.GlobalConfig.SmbdContainerImage,
		ImagePullPolicy: imagePullPolicy(planner),
		Name:            "ensure-share-paths",
		Args:            planner.Args().EnsureSharePaths(),
		Env:             env,
		VolumeMounts:    mounts,
	}
}

func buildMustJoinCtr(
	planner *pln.Planner,
	env []corev1.EnvVar,
	vols *volKeeper) corev1.Container {
	// ---
	mounts := getMounts(vols.all())
	return corev1.Container{
		Image:           planner.GlobalConfig.SmbdContainerImage,
		ImagePullPolicy: imagePullPolicy(planner),
		Name:            "must-join",
		Args:            planner.Args().Initializer("must-join"),
		Env:             env,
		VolumeMounts:    mounts,
	}
}

func buildCTDBMigrateCtr(
	planner *pln.Planner,
	env []corev1.EnvVar,
	vols *volKeeper) corev1.Container {
	// ---
	mounts := getMounts(vols.all())
	return corev1.Container{
		Image:           planner.GlobalConfig.SmbdContainerImage,
		ImagePullPolicy: imagePullPolicy(planner),
		Name:            "ctdb-migrate",
		Args:            planner.Args().CTDBMigrate(),
		Env:             env,
		VolumeMounts:    mounts,
	}
}

func buildCTDBSetNodeCtr(
	planner *pln.Planner,
	env []corev1.EnvVar,
	vols *volKeeper) corev1.Container {
	// ---
	mounts := getMounts(vols.all())
	return corev1.Container{
		Image:           planner.GlobalConfig.SmbdContainerImage,
		ImagePullPolicy: imagePullPolicy(planner),
		Name:            "ctdb-set-node",
		Args:            planner.Args().CTDBSetNode(),
		Env:             env,
		VolumeMounts:    mounts,
	}
}

func buildCTDBMustHaveNodeCtr(
	planner *pln.Planner,
	env []corev1.EnvVar,
	vols *volKeeper) corev1.Container {
	// ---
	mounts := getMounts(vols.all())
	return corev1.Container{
		Image:           planner.GlobalConfig.SmbdContainerImage,
		ImagePullPolicy: imagePullPolicy(planner),
		Name:            "ctdb-must-have-node",
		Args:            planner.Args().CTDBMustHaveNode(),
		Env:             env,
		VolumeMounts:    mounts,
	}
}

func buildUpdateConfigWatchCtr(
	planner *pln.Planner,
	env []corev1.EnvVar,
	vols *volKeeper) corev1.Container {
	// ---
	mounts := getMounts(vols.all())
	return corev1.Container{
		Image:        planner.GlobalConfig.SmbdContainerImage,
		Name:         "watch-update-config",
		Args:         planner.Args().UpdateConfigWatch(),
		Env:          env,
		VolumeMounts: mounts,
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

func imagePullPolicy(pl *pln.Planner) corev1.PullPolicy {
	pullPolicy := corev1.PullPolicy(pl.GlobalConfig.ImagePullPolicy)
	switch {
	case pullPolicy == corev1.PullAlways:
	case pullPolicy == corev1.PullNever:
	case pullPolicy == corev1.PullIfNotPresent:
	default:
		pullPolicy = corev1.PullIfNotPresent
	}
	return pullPolicy
}

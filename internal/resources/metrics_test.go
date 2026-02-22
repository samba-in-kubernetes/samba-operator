// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestBuildSmbMetricsContainer(t *testing.T) {
	ctr := buildSmbMetricsContainer("quay.io/samba.org/samba-metrics:latest", nil, nil)

	t.Run("name", func(t *testing.T) {
		assert.Equal(t, "samba-metrics", ctr.Name)
	})

	t.Run("image", func(t *testing.T) {
		assert.Equal(t, "quay.io/samba.org/samba-metrics:latest", ctr.Image)
	})

	t.Run("portArgMatchesProbes", func(t *testing.T) {
		// The --port arg passed to the binary must match the port used by the
		// liveness and readiness probes, otherwise the probes always fail and
		// the container enters a CrashLoopBackOff.
		expectedArg := fmt.Sprintf("--port=%d", defaultMetricsPort)
		assert.Contains(t, ctr.Args, expectedArg,
			"smbmetrics binary must receive --port matching the probe port")

		assert.Equal(t, int32(defaultMetricsPort),
			ctr.LivenessProbe.TCPSocket.Port.IntVal,
			"liveness probe port must match --port arg")
		assert.Equal(t, int32(defaultMetricsPort),
			ctr.ReadinessProbe.TCPSocket.Port.IntVal,
			"readiness probe port must match --port arg")
	})

	t.Run("containerPort", func(t *testing.T) {
		assert.Len(t, ctr.Ports, 1)
		assert.Equal(t, int32(defaultMetricsPort), ctr.Ports[0].ContainerPort)
		assert.Equal(t, "smbmetrics", ctr.Ports[0].Name)
	})

	t.Run("envAndMounts", func(t *testing.T) {
		env := []corev1.EnvVar{{Name: "FOO", Value: "bar"}}
		mounts := []corev1.VolumeMount{{Name: "vol", MountPath: "/mnt"}}
		ctr2 := buildSmbMetricsContainer("img", env, mounts)
		assert.Equal(t, env, ctr2.Env)
		assert.Equal(t, mounts, ctr2.VolumeMounts)
	})
}

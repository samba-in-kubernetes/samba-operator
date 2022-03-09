// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var (
	// defaultMetricsPort is the default port used to export prometheus metrics
	defaultMetricsPort = int(8080)
	// defaultMetricsPath is the default HTTP path to export prometheus metrics
	defaultMetricsPath = "/metrics"
)

// annotationsForSmbMetricsPod returns the default annotation which are
// required on the pod which executes the smbmetrics container, in order for
// Prometheus to scrape it.
func annotationsForSmbMetricsPod() map[string]string {
	return map[string]string{
		"prometheus.io/scrape": "true",
		"prometheus.io/port":   strconv.Itoa(defaultMetricsPort),
		"prometheus.io/path":   defaultMetricsPath,
	}
}

// buildSmbMetricsContainer returns the appropriate Container definition for
// smbmetrics exporter.
func buildSmbMetricsContainer(image string,
	volmnts []corev1.VolumeMount) corev1.Container {
	portnum := defaultMetricsPort
	return corev1.Container{
		Image:   image,
		Name:    "samba-metrics",
		Command: []string{"/bin/smbmetrics"},
		Ports: []corev1.ContainerPort{{
			ContainerPort: int32(portnum),
			Name:          "smbmetrics",
		}},
		VolumeMounts: volmnts,
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

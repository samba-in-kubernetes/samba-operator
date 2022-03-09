package metrics

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// AnnotationsForSmbMetricsPod returns the default annotation which are
// required on the pod which executes the smbmetrics container, in order for
// Prometheus to scrape it.
func AnnotationsForSmbMetricsPod() map[string]string {
	return map[string]string{
		"prometheus.io/scrape": "true",
		"prometheus.io/port":   strconv.Itoa(DefaultMetricsPort),
		"prometheus.io/path":   DefaultMetricsPath,
	}
}

// BuildSmbMetricsContainer returns the appropriate Container definition for
// smbmetrics exporter.
func BuildSmbMetricsContainer(image string,
	volmnts []corev1.VolumeMount) corev1.Container {
	portnum := DefaultMetricsPort
	return corev1.Container{
		// ImagePullPolicy: corev1.PullAlways,
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

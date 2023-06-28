// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"
	"strconv"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	rtclient "sigs.k8s.io/controller-runtime/pkg/client"

	pln "github.com/samba-in-kubernetes/samba-operator/internal/planner"
)

var (
	// defaultMetricsPort is the default port used to export prometheus metrics
	defaultMetricsPort = int(8080)
	// defaultMetricsPath is the default HTTP path to export prometheus metrics
	defaultMetricsPath = "/metrics"
	// defaultMetricsPortName is the name of the metrics port which is exported
	// via k8s service (OpenShift)
	defaultMetricsPortName = "samba-metrics"
	// defaultMetricsScrapeInterval defines the default interval for prometheus
	// metrics-scraping (Openshift)
	defaultMetricsScrapeInterval = "1m"
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
	env []corev1.EnvVar,
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
		Env:          env,
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(portnum),
				},
			},
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(portnum),
				},
			},
		},
	}
}

func (m *SmbShareManager) getOrCreateMetricsService(
	ctx context.Context, pl *pln.Planner, ns string) (
	*corev1.Service, bool, error) {
	// ---
	if !IsOpenShiftCluster(ctx, m.client, m.cfg) {
		return nil, false, nil // OK (not on OpenShift)
	}
	srvCurr, srvKey, err := m.getMetricsServiceOf(ctx, pl, ns)
	if err == nil {
		return srvCurr, false, nil // OK
	}
	if !errors.IsNotFound(err) {
		m.logger.Error(err, "Failed to get metrics Service", "key", srvKey)
		return nil, false, err
	}
	inst := pl.SmbShare
	labels := labelsForManagedResource(metricsInstanceName(pl))
	srvWant := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: srvKey.Namespace,
			Name:      srvKey.Name,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: inst.APIVersion,
					Kind:       inst.Kind,
					Name:       inst.Name,
					UID:        inst.UID,
				},
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:     defaultMetricsPortName,
					Port:     int32(defaultMetricsPort),
					Protocol: corev1.ProtocolTCP,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: int32(defaultMetricsPort),
					},
				},
			},
			Selector: labels,
		},
	}
	err = m.client.Create(ctx, srvWant, &rtclient.CreateOptions{})
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			m.logger.Error(err, "Failed to create metrics Service",
				"key", srvKey)
			return nil, false, err
		}
		m.logger.Info("Retry to get metrics Service", "key", srvKey)
		srvCurr, srvKey, err = m.getMetricsServiceOf(ctx, pl, ns)
		if err != nil {
			m.logger.Error(err, "Failed to get existing metrics Service",
				"key", srvKey)
			return nil, false, err
		}
		return srvCurr, false, err
	}
	return srvWant, true, nil
}

func (m *SmbShareManager) getMetricsServiceOf(
	ctx context.Context, pl *pln.Planner, ns string) (
	*corev1.Service, types.NamespacedName, error) {
	// ---
	srvKey := types.NamespacedName{
		Namespace: ns,
		Name:      metricsInstanceName(pl),
	}
	srv := &corev1.Service{}
	err := m.client.Get(ctx, srvKey, srv)
	if err != nil {
		return nil, srvKey, err
	}
	return srv, srvKey, err
}

func (m *SmbShareManager) getOrCreateMetricsServiceMonitor(
	ctx context.Context, pl *pln.Planner, ns string) (
	*monitoringv1.ServiceMonitor, bool, error) {
	// ---
	if !IsOpenShiftCluster(ctx, m.client, m.cfg) {
		return nil, false, nil // OK (not on OpenShift)
	}
	smCurr, smKey, err := m.getMetricsServiceMonitorOf(ctx, pl, ns)
	if err == nil {
		return smCurr, false, nil // OK
	}
	if !errors.IsNotFound(err) {
		m.logger.Error(err, "Failed to get metrics ServiceMonitor",
			"key", smKey)
		return nil, false, err
	}
	inst := pl.SmbShare
	labels := labelsForManagedResource(metricsInstanceName(pl))
	smWant := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: smKey.Namespace,
			Name:      smKey.Name,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: inst.APIVersion,
					Kind:       inst.Kind,
					Name:       inst.Name,
					UID:        inst.UID,
				},
			},
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{smKey.Namespace},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: labels,
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Port:     defaultMetricsPortName,
					Path:     defaultMetricsPath,
					Interval: defaultMetricsScrapeInterval,
				},
			},
		},
	}
	err = m.client.Create(ctx, smWant, &rtclient.CreateOptions{})
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			m.logger.Error(err, "Failed to create metrics ServiceMonitor",
				"key", smKey)
			return nil, false, err
		}
		m.logger.Info("Retry to get metrics ServiceMonitor", "key", smKey)
		smCurr, smKey, err = m.getMetricsServiceMonitorOf(ctx, pl, ns)
		if err != nil {
			m.logger.Error(err, "Failed to get existing metrics ServiceMonitor",
				"key", smKey)
			return nil, false, err
		}
		return smCurr, false, nil
	}
	return smWant, true, nil
}

func metricsInstanceName(pl *pln.Planner) string {
	return pl.InstanceName() + "-metrics"
}

func (m *SmbShareManager) getMetricsServiceMonitorOf(
	ctx context.Context, pl *pln.Planner, ns string) (
	*monitoringv1.ServiceMonitor, types.NamespacedName, error) {
	// ---
	smKey := types.NamespacedName{
		Namespace: ns,
		Name:      fmt.Sprintf("%s-metrics-monitor", pl.InstanceName()),
	}
	sm := &monitoringv1.ServiceMonitor{}
	err := m.client.Get(ctx, smKey, sm)
	if err != nil {
		return nil, smKey, err
	}
	return sm, smKey, err
}

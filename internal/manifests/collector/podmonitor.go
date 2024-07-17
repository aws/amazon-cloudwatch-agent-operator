// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"fmt"

	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1beta1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

// ServiceMonitor returns the service monitor for the given instance.
func PodMonitor(params manifests.Params) (*monitoringv1.PodMonitor, error) {
	if !params.OtelCol.Spec.Observability.Metrics.EnableMetrics {
		params.Log.V(2).Info("Metrics disabled for this OTEL Collector",
			"params.OtelCol.name", params.OtelCol.Name,
			"params.OtelCol.namespace", params.OtelCol.Namespace,
		)
		return nil, nil
	}
	var pm monitoringv1.PodMonitor

	if params.OtelCol.Spec.Mode != v1beta1.ModeSidecar {
		return nil, nil
	}

	pm = monitoringv1.PodMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: params.OtelCol.Namespace,
			Name:      naming.PodMonitor(params.OtelCol.Name),
			Labels: map[string]string{
				"app.kubernetes.io/name":       naming.PodMonitor(params.OtelCol.Name),
				"app.kubernetes.io/instance":   fmt.Sprintf("%s.%s", params.OtelCol.Namespace, params.OtelCol.Name),
				"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
			},
		},
		Spec: monitoringv1.PodMonitorSpec{
			JobLabel:        "app.kubernetes.io/instance",
			PodTargetLabels: []string{"app.kubernetes.io/name", "app.kubernetes.io/instance", "app.kubernetes.io/managed-by"},
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{params.OtelCol.Namespace},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
					"app.kubernetes.io/instance":   fmt.Sprintf("%s.%s", params.OtelCol.Namespace, params.OtelCol.Name),
				},
			},
			PodMetricsEndpoints: append(
				[]monitoringv1.PodMetricsEndpoint{
					{
						Port: "monitoring",
					},
				}, metricsEndpointsFromConfig(params.Log, params.OtelCol)...),
		},
	}

	return &pm, nil
}

// Not used in amazon-cloudwatch-agent-operator
func metricsEndpointsFromConfig(logger logr.Logger, otelcol v1beta1.AmazonCloudWatchAgent) []monitoringv1.PodMetricsEndpoint {
	metricsEndpoints := []monitoringv1.PodMetricsEndpoint{}
	return metricsEndpoints
}

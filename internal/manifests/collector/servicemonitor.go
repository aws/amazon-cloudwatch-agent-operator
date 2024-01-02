// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/adapters"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

// ServiceMonitor returns the service monitor for the given instance.
func ServiceMonitor(params manifests.Params) (*monitoringv1.ServiceMonitor, error) {
	if !params.OtelCol.Spec.Observability.Metrics.EnableMetrics {
		params.Log.V(2).Info("Metrics disabled for this OTEL Collector",
			"params.OtelCol.name", params.OtelCol.Name,
			"params.OtelCol.namespace", params.OtelCol.Namespace,
		)
		return nil, nil
	}
	var sm monitoringv1.ServiceMonitor

	if params.OtelCol.Spec.Mode == v1alpha1.ModeSidecar {
		return nil, nil
	}
	sm = monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: params.OtelCol.Namespace,
			Name:      naming.ServiceMonitor(params.OtelCol.Name),
			Labels: map[string]string{
				"app.kubernetes.io/name":       naming.ServiceMonitor(params.OtelCol.Name),
				"app.kubernetes.io/instance":   fmt.Sprintf("%s.%s", params.OtelCol.Namespace, params.OtelCol.Name),
				"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
			},
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Endpoints: append([]monitoringv1.Endpoint{
				{
					Port: "monitoring",
				},
			}, endpointsFromConfig(params.Log, params.OtelCol)...),
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{params.OtelCol.Namespace},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
					"app.kubernetes.io/instance":   fmt.Sprintf("%s.%s", params.OtelCol.Namespace, params.OtelCol.Name),
				},
			},
		},
	}

	return &sm, nil
}

func endpointsFromConfig(logger logr.Logger, otelcol v1alpha1.AmazonCloudWatchAgent) []monitoringv1.Endpoint {
	c, err := adapters.ConfigFromString(otelcol.Spec.Config)
	if err != nil {
		logger.V(2).Error(err, "Error while parsing the configuration")
		return []monitoringv1.Endpoint{}
	}

	exporterPorts, err := adapters.ConfigToComponentPorts(logger, adapters.ComponentTypeExporter, c)
	if err != nil {
		logger.Error(err, "couldn't build service monitors from configuration")
		return []monitoringv1.Endpoint{}
	}

	endpoints := []monitoringv1.Endpoint{}

	for _, port := range exporterPorts {
		if strings.Contains(port.Name, "prometheus") {
			e := monitoringv1.Endpoint{
				Port: port.Name,
			}
			endpoints = append(endpoints, e)
		}
	}
	return endpoints
}

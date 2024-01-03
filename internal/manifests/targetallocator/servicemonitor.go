// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

// ServiceMonitor returns the service monitor for the given instance.
func ServiceMonitor(params manifests.Params) *monitoringv1.ServiceMonitor {
	name := naming.TargetAllocator(params.OtelCol.Name)
	labels := Labels(params.OtelCol, name)

	return &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: params.OtelCol.Namespace,
			Name:      name,
			Labels:    labels,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Endpoints: []monitoringv1.Endpoint{
				{
					Port: "targetallocation",
				},
			},

			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{params.OtelCol.Namespace},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: SelectorLabels(params.OtelCol),
			},
		},
	}
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

func Ingress(params manifests.Params) (*networkingv1.Ingress, error) {
	if params.OtelCol.Spec.Ingress.Type != v1alpha1.IngressTypeNginx {
		return nil, nil
	}

	ports, err := servicePortsFromCfg(params.Log, params.OtelCol)

	// if we have no ports, we don't need a ingress entry
	if len(ports) == 0 || err != nil {
		params.Log.V(1).Info(
			"the instance's configuration didn't yield any ports to open, skipping ingress",
			"instance.name", params.OtelCol.Name,
			"instance.namespace", params.OtelCol.Namespace,
		)
		return nil, err
	}

	var rules []networkingv1.IngressRule
	switch params.OtelCol.Spec.Ingress.RuleType {
	case v1alpha1.IngressRuleTypePath, "":
		rules = []networkingv1.IngressRule{createPathIngressRules(params.OtelCol.Name, params.OtelCol.Spec.Ingress.Hostname, ports)}
	case v1alpha1.IngressRuleTypeSubdomain:
		rules = createSubdomainIngressRules(params.OtelCol.Name, params.OtelCol.Spec.Ingress.Hostname, ports)
	}

	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        naming.Ingress(params.OtelCol.Name),
			Namespace:   params.OtelCol.Namespace,
			Annotations: params.OtelCol.Spec.Ingress.Annotations,
			Labels: map[string]string{
				"app.kubernetes.io/name":       naming.Ingress(params.OtelCol.Name),
				"app.kubernetes.io/instance":   fmt.Sprintf("%s.%s", params.OtelCol.Namespace, params.OtelCol.Name),
				"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
			},
		},
		Spec: networkingv1.IngressSpec{
			TLS:              params.OtelCol.Spec.Ingress.TLS,
			Rules:            rules,
			IngressClassName: params.OtelCol.Spec.Ingress.IngressClassName,
		},
	}, nil
}

func createPathIngressRules(otelcol string, hostname string, ports []corev1.ServicePort) networkingv1.IngressRule {
	pathType := networkingv1.PathTypePrefix
	paths := make([]networkingv1.HTTPIngressPath, len(ports))
	for i, port := range ports {
		portName := naming.PortName(port.Name, port.Port)
		paths[i] = networkingv1.HTTPIngressPath{
			Path:     "/" + port.Name,
			PathType: &pathType,
			Backend: networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: naming.Service(otelcol),
					Port: networkingv1.ServiceBackendPort{
						Name: portName,
					},
				},
			},
		}
	}
	return networkingv1.IngressRule{
		Host: hostname,
		IngressRuleValue: networkingv1.IngressRuleValue{
			HTTP: &networkingv1.HTTPIngressRuleValue{
				Paths: paths,
			},
		},
	}
}

func createSubdomainIngressRules(otelcol string, hostname string, ports []corev1.ServicePort) []networkingv1.IngressRule {
	var rules []networkingv1.IngressRule
	pathType := networkingv1.PathTypePrefix
	for _, port := range ports {
		portName := naming.PortName(port.Name, port.Port)

		host := fmt.Sprintf("%s.%s", portName, hostname)
		// This should not happen due to validation in the webhook.
		if hostname == "" || hostname == "*" {
			host = portName
		}
		rules = append(rules, networkingv1.IngressRule{
			Host: host,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path:     "/",
							PathType: &pathType,
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: naming.Service(otelcol),
									Port: networkingv1.ServiceBackendPort{
										Name: portName,
									},
								},
							},
						},
					},
				},
			},
		})
	}
	return rules
}

func servicePortsFromCfg(logger logr.Logger, otelcol v1alpha1.AmazonCloudWatchAgent) ([]corev1.ServicePort, error) {
	var ports []corev1.ServicePort
	if len(otelcol.Spec.Ports) > 0 {
		// we should add all the ports from the CR
		// there are two cases where problems might occur:
		// 1) when the port number is already being used by a receiver
		// 2) same, but for the port name
		//
		// in the first case, we remove the port we inferred from the list
		// in the second case, we rename our inferred port to something like "port-%d"
		portNumbers, portNames := extractPortNumbersAndNames(otelcol.Spec.Ports)
		var resultingInferredPorts []corev1.ServicePort
		for _, inferred := range ports {
			if filtered := filterPort(logger, inferred, portNumbers, portNames); filtered != nil {
				resultingInferredPorts = append(resultingInferredPorts, *filtered)
			}
		}
		ports = append(otelcol.Spec.Ports, resultingInferredPorts...)
	}
	return ports, nil
}

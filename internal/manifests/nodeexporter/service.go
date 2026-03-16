// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package nodeexporter

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/manifestutils"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

func Service(params manifests.Params) (*corev1.Service, error) {
	name := naming.Service(params.NodeExp.Name)
	if len(name) == 0 {
		name = ComponentNodeExporter
	}
	labels := manifestutils.Labels(params.NodeExp.ObjectMeta, name, params.NodeExp.Spec.Image, ComponentNodeExporter, []string{})
	//this label is used by scraper config in the agent.
	labels["k8s-app"] = "node-exporter-service"
	annotations := Annotations(params.NodeExp)
	annotations["prometheus.io/scrape"] = "true"
	var ports []corev1.ServicePort
	nodeExpPort := corev1.ServicePort{
		Name:       "metrics",
		Port:       9100,
		TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 9100},
		Protocol:   corev1.ProtocolTCP,
	}
	if len(params.NodeExp.Spec.Ports) > 0 {
		// update default service values with what's from CR
		nodeExpPort.Name = params.NodeExp.Spec.Ports[0].Name
		nodeExpPort.Port = params.NodeExp.Spec.Ports[0].Port
		nodeExpPort.TargetPort.IntVal = params.NodeExp.Spec.Ports[0].Port
	}
	ports = append(ports, nodeExpPort)
	trafficPolicy := corev1.ServiceInternalTrafficPolicyLocal

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-service", name),
			Namespace:   params.NodeExp.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Type:                  corev1.ServiceTypeClusterIP,
			InternalTrafficPolicy: &trafficPolicy,
			Selector:              manifestutils.SelectorLabels(params.NodeExp.ObjectMeta, ComponentNodeExporter),
			Ports:                 ports,
		},
	}, nil
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package neuronmonitor

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/manifestutils"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

func Service(params manifests.Params) (*corev1.Service, error) {
	name := serviceBaseName(params.NeuronExp)
	labels := manifestutils.Labels(params.NeuronExp.ObjectMeta, name, params.NeuronExp.Spec.Image, ComponentNeuronExporter, []string{})
	//this label is used by scraper config in the agent.
	labels["k8s-app"] = "neuron-monitor-service"
	annotations := Annotations(params.NeuronExp)
	annotations["prometheus.io/scrape"] = "true"
	var ports []corev1.ServicePort
	neuronPort := corev1.ServicePort{
		Name:       "metrics",
		Port:       8000,
		TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 8000},
		Protocol:   corev1.ProtocolTCP,
	}
	if len(params.NeuronExp.Spec.Ports) > 0 {
		// update default service values with what's from CR
		neuronPort.Name = params.NeuronExp.Spec.Ports[0].Name
		neuronPort.Port = params.NeuronExp.Spec.Ports[0].Port
		neuronPort.TargetPort.IntVal = params.NeuronExp.Spec.Ports[0].Port
	}
	ports = append(ports, neuronPort)
	trafficPolicy := corev1.ServiceInternalTrafficPolicyLocal

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ServiceName(params.NeuronExp),
			Namespace:   params.NeuronExp.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Type:                  corev1.ServiceTypeClusterIP,
			InternalTrafficPolicy: &trafficPolicy,
			Selector:              manifestutils.SelectorLabels(params.NeuronExp.ObjectMeta, ComponentNeuronExporter),
			Ports:                 ports,
		},
	}, nil
}

// ServiceName returns the name of the Service that exposes the exporter's metrics.
func ServiceName(exporter v1alpha1.NeuronMonitor) string {
	return fmt.Sprintf("%s-service", serviceBaseName(exporter))
}

func serviceBaseName(exporter v1alpha1.NeuronMonitor) string {
	name := naming.Service(exporter.Name)
	if len(name) == 0 {
		return ComponentNeuronExporter
	}
	return name
}

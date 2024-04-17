// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dcgmexporter

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
	name := naming.Service(params.DcgmExp.Name)
	if len(name) == 0 {
		name = ComponentDcgmExporter
	}
	labels := manifestutils.Labels(params.DcgmExp.ObjectMeta, name, params.DcgmExp.Spec.Image, ComponentDcgmExporter, []string{})
	//this label is used by scraper config in the agent.
	labels["k8s-app"] = "dcgm-exporter-service"
	annotations := Annotations(params.DcgmExp)
	annotations["prometheus.io/scrape"] = "true"
	var ports []corev1.ServicePort
	dcgmPort := corev1.ServicePort{
		Name:       "metrics",
		Port:       9400,
		TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 9400},
		Protocol:   corev1.ProtocolTCP,
	}
	if len(params.DcgmExp.Spec.Ports) > 0 {
		// update default service values with what's from CR
		dcgmPort.Name = params.DcgmExp.Spec.Ports[0].Name
		dcgmPort.Port = params.DcgmExp.Spec.Ports[0].Port
		dcgmPort.TargetPort.IntVal = params.DcgmExp.Spec.Ports[0].Port
	}
	ports = append(ports, dcgmPort)
	trafficPolicy := corev1.ServiceInternalTrafficPolicyLocal

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-service", name),
			Namespace:   params.DcgmExp.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Type:                  corev1.ServiceTypeClusterIP,
			InternalTrafficPolicy: &trafficPolicy,
			Selector:              manifestutils.SelectorLabels(params.DcgmExp.ObjectMeta, ComponentDcgmExporter),
			Ports:                 ports,
		},
	}, nil
}

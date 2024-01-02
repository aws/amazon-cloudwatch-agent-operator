// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package opampbridge

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"strings"

	"gopkg.in/yaml.v2"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/manifestutils"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

func ConfigMap(params manifests.Params) (*corev1.ConfigMap, error) {
	name := naming.OpAMPBridgeConfigMap(params.OpAMPBridge.Name)
	version := strings.Split(params.OpAMPBridge.Spec.Image, ":")
	labels := manifestutils.Labels(params.OpAMPBridge.ObjectMeta, name, params.OpAMPBridge.Spec.Image, ComponentOpAMPBridge, []string{})

	if len(version) > 1 {
		labels["app.kubernetes.io/version"] = version[len(version)-1]
	} else {
		labels["app.kubernetes.io/version"] = "latest"
	}

	config := make(map[interface{}]interface{})

	if len(params.OpAMPBridge.Spec.Endpoint) > 0 {
		config["endpoint"] = params.OpAMPBridge.Spec.Endpoint
	}

	if len(params.OpAMPBridge.Spec.Headers) > 0 {
		config["headers"] = params.OpAMPBridge.Spec.Headers
	}

	if params.OpAMPBridge.Spec.Capabilities != nil {
		config["capabilities"] = params.OpAMPBridge.Spec.Capabilities
	}

	if params.OpAMPBridge.Spec.ComponentsAllowed != nil {
		config["componentsAllowed"] = params.OpAMPBridge.Spec.ComponentsAllowed
	}

	configYAML, err := yaml.Marshal(config)
	if err != nil {
		return &corev1.ConfigMap{}, err
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   params.OpAMPBridge.Namespace,
			Labels:      labels,
			Annotations: params.OpAMPBridge.Annotations,
		},
		Data: map[string]string{
			"remoteconfiguration.yaml": string(configYAML),
		},
	}, nil
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

func TestServicePorts(t *testing.T) {
	otelcol := collectorInstance()
	cfg := config.New()

	params := manifests.Params{
		OtelCol: otelcol,
		Config:  cfg,
		Log:     logger,
	}

	ports := []v1.ServicePort{{Name: "targetallocation", Port: naming.TargetAllocatorServicePort, TargetPort: intstr.FromInt32(naming.TargetAllocatorContainerPort)}}

	expectedLabels := map[string]string{
		"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
		"app.kubernetes.io/instance":   "default.my-instance",
		"app.kubernetes.io/part-of":    "amazon-cloudwatch-agent",
		"app.kubernetes.io/component":  "amazon-cloudwatch-agent-target-allocator",
		"app.kubernetes.io/name":       "my-instance-target-allocator",
	}

	s := Service(params)

	assert.Equal(t, ports[0].Name, s.Spec.Ports[0].Name)
	assert.Equal(t, ports[0].Port, s.Spec.Ports[0].Port)
	assert.Equal(t, ports[0].TargetPort, s.Spec.Ports[0].TargetPort)
	assert.Equal(t, expectedLabels, s.Spec.Selector)
}

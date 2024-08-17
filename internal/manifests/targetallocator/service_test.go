// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
)

func TestServicePorts(t *testing.T) {
	targetAllocator := targetAllocatorInstance()
	cfg := config.New()

	params := Params{
		TargetAllocator: targetAllocator,
		Config:          cfg,
		Log:             logger,
	}

	ports := []v1.ServicePort{{Name: "targetallocation", Port: 80, TargetPort: intstr.FromString("http")}}

	s := Service(params)

	assert.Equal(t, ports[0].Name, s.Spec.Ports[0].Name)
	assert.Equal(t, ports[0].Port, s.Spec.Ports[0].Port)
	assert.Equal(t, ports[0].TargetPort, s.Spec.Ports[0].TargetPort)
}

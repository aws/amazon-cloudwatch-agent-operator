// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package neuronmonitor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1beta1"
)

func TestServiceAccountNewDefault(t *testing.T) {
	exporter := v1beta1.NeuronMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance",
		},
	}
	sa := ServiceAccountName(exporter)
	assert.Equal(t, "neuron-monitor-service-acct", sa)
}

func TestServiceAccountOverride(t *testing.T) {
	exporter := v1beta1.NeuronMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance",
		},
		Spec: v1beta1.NeuronMonitorSpec{
			ServiceAccount: "my-special-sa",
		},
	}
	sa := ServiceAccountName(exporter)
	assert.Equal(t, "my-special-sa", sa)
}

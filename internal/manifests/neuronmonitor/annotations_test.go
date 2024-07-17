// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package neuronmonitor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1beta1"
)

func TestDefaultAnnotations(t *testing.T) {
	// prepare
	exporter := v1beta1.NeuronMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "my-ns",
		},
		Spec: v1beta1.NeuronMonitorSpec{},
	}
	// test
	annotations := Annotations(exporter)

	//verify
	assert.Equal(t, "neuron-monitor", annotations["k8s-app"])
	assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", annotations["amazon-cloudwatch-agent-operator-config/sha256"])
}

func TestUserAnnotations(t *testing.T) {
	// prepare
	exporter := v1beta1.NeuronMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "my-ns",
			Annotations: map[string]string{
				"prometheus.io/test":                             "test",
				"amazon-cloudwatch-agent-operator-config/sha256": "shouldBeOverwritten",
			},
		},
		Spec: v1beta1.NeuronMonitorSpec{},
	}

	// test
	annotations := Annotations(exporter)

	//verify
	assert.Equal(t, "test", annotations["prometheus.io/test"])
	assert.Equal(t, "neuron-monitor", annotations["k8s-app"])
	assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", annotations["amazon-cloudwatch-agent-operator-config/sha256"])
}

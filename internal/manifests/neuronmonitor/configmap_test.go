// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package neuronmonitor

import (
	"testing"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/tools/record"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func TestDesiredConfigMap(t *testing.T) {
	expectedLables := map[string]string{
		"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
		"app.kubernetes.io/instance":   "default.test",
		"app.kubernetes.io/part-of":    "amazon-cloudwatch-agent",
		"app.kubernetes.io/version":    "0.1.0",
	}

	t.Run("should return expected neuron monitor config map", func(t *testing.T) {
		expectedLables["app.kubernetes.io/component"] = "neuron-monitor"
		expectedLables["app.kubernetes.io/name"] = "neuron-monitor-config-map"
		expectedLables["app.kubernetes.io/version"] = "0.1.0"

		expectedData := map[string]string{
			NeuronMonitorJson: `{"period":"5s","neuron_runtimes":[{"tag_filter":".*","metrics":[{"type":"neuroncore_counters"},{"type":"memory_used"},{"type":"neuron_runtime_vcpu_usage"},{"type":"execution_stats"}]}],"system_metrics":[{"type":"memory_info"},{"period":"5s","type":"neuron_hw_counters"}]}`,
		}

		param := getParams()
		actual, err := ConfigMap(param)

		assert.NoError(t, err)
		assert.Equal(t, NeuronConfigMapName, actual.Name)
		assert.Equal(t, expectedLables, actual.Labels)
		assert.Equal(t, expectedData, actual.Data)

	})
}

func getParams() manifests.Params {
	return manifests.Params{
		Config: config.New(config.WithNeuronMonitorImage("default-exporter")),
		NeuronExp: v1alpha1.NeuronMonitor{
			TypeMeta: metav1.TypeMeta{
				Kind:       "cloudwatch.aws.amazon.com",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "default",
				UID:       uuid.NewUUID(),
			},
			Spec: v1alpha1.NeuronMonitorSpec{
				Image:         "public.ecr.aws/cloudwatch-agent/neuron-monitor:0.1.0",
				MonitorConfig: `{"period":"5s","neuron_runtimes":[{"tag_filter":".*","metrics":[{"type":"neuroncore_counters"},{"type":"memory_used"},{"type":"neuron_runtime_vcpu_usage"},{"type":"execution_stats"}]}],"system_metrics":[{"type":"memory_info"},{"period":"5s","type":"neuron_hw_counters"}]}`,
			},
		},
		Log:      logf.Log.WithName("unit-tests"),
		Recorder: record.NewFakeRecorder(10),
	}
}

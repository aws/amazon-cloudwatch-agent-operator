// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package neuronmonitor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
)

func TestNeuronDaemonSetPodLabels(t *testing.T) {
	neuron := v1alpha1.NeuronMonitor{
		ObjectMeta: metav1.ObjectMeta{Name: "neuron"},
		Spec: v1alpha1.NeuronMonitorSpec{
			PodLabels: map[string]string{
				"custom-label": "custom-value",
			},
		},
	}
	ds := DaemonSet(manifests.Params{
		Log:       logger,
		Config:    config.New(),
		NeuronExp: neuron,
	})
	assert.Equal(t, "custom-value", ds.Spec.Template.Labels["custom-label"])
	assert.Equal(t, ComponentNeuronExporter, ds.Spec.Template.Labels["app.kubernetes.io/component"])
}

func TestNeuronDaemonSetPodAnnotations(t *testing.T) {
	neuron := v1alpha1.NeuronMonitor{
		ObjectMeta: metav1.ObjectMeta{Name: "neuron"},
		Spec: v1alpha1.NeuronMonitorSpec{
			PodAnnotations: map[string]string{
				"custom-anno": "custom-value",
			},
		},
	}
	ds := DaemonSet(manifests.Params{
		Log:       logger,
		Config:    config.New(),
		NeuronExp: neuron,
	})
	assert.Equal(t, "custom-value", ds.Spec.Template.Annotations["custom-anno"])
	// operator-managed config-sha256 annotation is always present so that
	// changes to MonitorConfig roll the DaemonSet
	assert.Contains(t, ds.Spec.Template.Annotations, "amazon-cloudwatch-agent-operator-config/sha256")
	assert.NotEmpty(t, ds.Spec.Template.Annotations["amazon-cloudwatch-agent-operator-config/sha256"])
}

func TestNeuronDaemonSetConfigSha256AnnotationChangesWithMonitorConfig(t *testing.T) {
	neuronA := v1alpha1.NeuronMonitor{
		ObjectMeta: metav1.ObjectMeta{Name: "neuron"},
		Spec: v1alpha1.NeuronMonitorSpec{
			MonitorConfig: `{"period":"5s"}`,
		},
	}
	neuronB := neuronA
	neuronB.Spec.MonitorConfig = `{"period":"10s"}`

	dsA := DaemonSet(manifests.Params{Log: logger, Config: config.New(), NeuronExp: neuronA})
	dsB := DaemonSet(manifests.Params{Log: logger, Config: config.New(), NeuronExp: neuronB})

	shaA := dsA.Spec.Template.Annotations["amazon-cloudwatch-agent-operator-config/sha256"]
	shaB := dsB.Spec.Template.Annotations["amazon-cloudwatch-agent-operator-config/sha256"]
	assert.NotEmpty(t, shaA)
	assert.NotEmpty(t, shaB)
	assert.NotEqual(t, shaA, shaB, "MonitorConfig change must roll the DaemonSet")
}

func TestNeuronDaemonSetTopologySpreadConstraints(t *testing.T) {
	tsc := []corev1.TopologySpreadConstraint{
		{
			MaxSkew:           1,
			TopologyKey:       "topology.kubernetes.io/zone",
			WhenUnsatisfiable: corev1.DoNotSchedule,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/component": ComponentNeuronExporter,
				},
			},
		},
	}
	neuron := v1alpha1.NeuronMonitor{
		ObjectMeta: metav1.ObjectMeta{Name: "neuron"},
		Spec: v1alpha1.NeuronMonitorSpec{
			TopologySpreadConstraints: tsc,
		},
	}
	ds := DaemonSet(manifests.Params{
		Log:       logger,
		Config:    config.New(),
		NeuronExp: neuron,
	})
	assert.Equal(t, tsc, ds.Spec.Template.Spec.TopologySpreadConstraints)
}

func TestNeuronDaemonSetPriorityClassName(t *testing.T) {
	neuron := v1alpha1.NeuronMonitor{
		ObjectMeta: metav1.ObjectMeta{Name: "neuron"},
		Spec: v1alpha1.NeuronMonitorSpec{
			PriorityClassName: "system-node-critical",
		},
	}
	ds := DaemonSet(manifests.Params{
		Log:       logger,
		Config:    config.New(),
		NeuronExp: neuron,
	})
	assert.Equal(t, "system-node-critical", ds.Spec.Template.Spec.PriorityClassName)
}

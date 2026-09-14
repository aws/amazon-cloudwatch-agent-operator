// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dcgmexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
)

func TestDcgmDaemonSetPodLabels(t *testing.T) {
	dcgm := v1alpha1.DcgmExporter{
		ObjectMeta: metav1.ObjectMeta{Name: "dcgm"},
		Spec: v1alpha1.DcgmExporterSpec{
			PodLabels: map[string]string{
				"custom-label": "custom-value",
			},
		},
	}
	ds := DaemonSet(manifests.Params{
		Log:     logger,
		Config:  config.New(),
		DcgmExp: dcgm,
	})
	assert.Equal(t, "custom-value", ds.Spec.Template.Labels["custom-label"])
	// operator selector labels preserved
	assert.Equal(t, ComponentDcgmExporter, ds.Spec.Template.Labels["app.kubernetes.io/component"])
}

func TestDcgmDaemonSetPodAnnotations(t *testing.T) {
	dcgm := v1alpha1.DcgmExporter{
		ObjectMeta: metav1.ObjectMeta{Name: "dcgm"},
		Spec: v1alpha1.DcgmExporterSpec{
			PodAnnotations: map[string]string{
				"custom-anno": "custom-value",
			},
		},
	}
	ds := DaemonSet(manifests.Params{
		Log:     logger,
		Config:  config.New(),
		DcgmExp: dcgm,
	})
	// user-provided annotation is present
	assert.Equal(t, "custom-value", ds.Spec.Template.Annotations["custom-anno"])
	// operator-managed config-sha256 annotation is always present so that
	// changes to MetricsConfig roll the DaemonSet
	assert.Contains(t, ds.Spec.Template.Annotations, "amazon-cloudwatch-agent-operator-config/sha256")
	assert.NotEmpty(t, ds.Spec.Template.Annotations["amazon-cloudwatch-agent-operator-config/sha256"])
}

func TestDcgmDaemonSetConfigSha256AnnotationChangesWithMetricsConfig(t *testing.T) {
	// Same DcgmExporter with two different MetricsConfig values should produce
	// different sha256 annotations, ensuring pods roll when the config changes.
	dcgmA := v1alpha1.DcgmExporter{
		ObjectMeta: metav1.ObjectMeta{Name: "dcgm"},
		Spec: v1alpha1.DcgmExporterSpec{
			MetricsConfig: "DCGM_FI_DEV_GPU_UTIL, gauge, GPU utilization",
		},
	}
	dcgmB := dcgmA
	dcgmB.Spec.MetricsConfig = "DCGM_FI_DEV_MEM_COPY_UTIL, gauge, Memory utilization"

	dsA := DaemonSet(manifests.Params{Log: logger, Config: config.New(), DcgmExp: dcgmA})
	dsB := DaemonSet(manifests.Params{Log: logger, Config: config.New(), DcgmExp: dcgmB})

	shaA := dsA.Spec.Template.Annotations["amazon-cloudwatch-agent-operator-config/sha256"]
	shaB := dsB.Spec.Template.Annotations["amazon-cloudwatch-agent-operator-config/sha256"]
	assert.NotEmpty(t, shaA)
	assert.NotEmpty(t, shaB)
	assert.NotEqual(t, shaA, shaB, "MetricsConfig change must roll the DaemonSet")
}

func TestDcgmDaemonSetTopologySpreadConstraints(t *testing.T) {
	tsc := []corev1.TopologySpreadConstraint{
		{
			MaxSkew:           1,
			TopologyKey:       "topology.kubernetes.io/zone",
			WhenUnsatisfiable: corev1.DoNotSchedule,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/component": ComponentDcgmExporter,
				},
			},
		},
	}
	dcgm := v1alpha1.DcgmExporter{
		ObjectMeta: metav1.ObjectMeta{Name: "dcgm"},
		Spec: v1alpha1.DcgmExporterSpec{
			TopologySpreadConstraints: tsc,
		},
	}
	ds := DaemonSet(manifests.Params{
		Log:     logger,
		Config:  config.New(),
		DcgmExp: dcgm,
	})
	assert.Equal(t, tsc, ds.Spec.Template.Spec.TopologySpreadConstraints)
}

func TestDcgmDaemonSetPriorityClassName(t *testing.T) {
	dcgm := v1alpha1.DcgmExporter{
		ObjectMeta: metav1.ObjectMeta{Name: "dcgm"},
		Spec: v1alpha1.DcgmExporterSpec{
			PriorityClassName: "system-node-critical",
		},
	}
	ds := DaemonSet(manifests.Params{
		Log:     logger,
		Config:  config.New(),
		DcgmExp: dcgm,
	})
	assert.Equal(t, "system-node-critical", ds.Spec.Template.Spec.PriorityClassName)
}

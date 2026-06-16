// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
)

func TestDefaultAnnotations(t *testing.T) {
	// prepare
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "my-ns",
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			Config: "test",
		},
	}

	// test
	annotations := Annotations(otelcol)
	podAnnotations := PodAnnotations(otelcol)

	//verify
	assert.Equal(t, "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", annotations["amazon-cloudwatch-agent-operator-config/sha256"])
	//verify propagation from metadata.annotations to spec.template.spec.metadata.annotations
	assert.Equal(t, "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", podAnnotations["amazon-cloudwatch-agent-operator-config/sha256"])
}

func TestUserAnnotations(t *testing.T) {
	// prepare
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "my-ns",
			Annotations: map[string]string{
				"amazon-cloudwatch-agent-operator-config/sha256": "shouldBeOverwritten",
			},
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			Config: "test",
		},
	}

	// test
	annotations := Annotations(otelcol)
	podAnnotations := PodAnnotations(otelcol)

	//verify
	assert.Equal(t, "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", annotations["amazon-cloudwatch-agent-operator-config/sha256"])
	assert.Equal(t, "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", podAnnotations["amazon-cloudwatch-agent-operator-config/sha256"])
}

func TestAnnotationsPropagateDown(t *testing.T) {
	// prepare
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"myapp": "mycomponent"},
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			PodAnnotations: map[string]string{"pod_annotation": "pod_annotation_value"},
		},
	}

	// test
	annotations := Annotations(otelcol)
	podAnnotations := PodAnnotations(otelcol)

	// verify
	assert.Len(t, annotations, 2)
	assert.Equal(t, "mycomponent", annotations["myapp"])
	assert.Equal(t, "mycomponent", podAnnotations["myapp"])
	assert.Equal(t, "pod_annotation_value", podAnnotations["pod_annotation"])
}

func promConfig(t *testing.T, replacement string) v1alpha1.PrometheusConfig {
	t.Helper()
	cfg := map[string]interface{}{
		"scrape_configs": []interface{}{
			map[string]interface{}{
				"job_name": "kubernetes-pods-annotated",
				"relabel_configs": []interface{}{
					map[string]interface{}{
						"target_label": "bug2probe",
						"replacement":  replacement,
					},
				},
			},
		},
	}
	return v1alpha1.PrometheusConfig{
		Config: &v1alpha1.AnyConfig{Object: cfg},
	}
}

// TestPrometheusConfigChangeBumpsHash asserts that changing only Spec.Prometheus
// changes the pod-template config hash (so the pods roll on a Prometheus-only
// change), while an unchanged spec yields a stable hash.
func TestPrometheusConfigChangeBumpsHash(t *testing.T) {
	base := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "my-instance", Namespace: "my-ns"},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			Config:     "agent-config",
			Prometheus: promConfig(t, "value2"),
		},
	}

	// same spec twice -> stable hash
	h1 := PodAnnotations(base)["amazon-cloudwatch-agent-operator-config/sha256"]
	h2 := PodAnnotations(base)["amazon-cloudwatch-agent-operator-config/sha256"]
	assert.Equal(t, h1, h2, "hash must be stable when nothing changes")

	// change ONLY the prometheus config -> hash must change
	changed := base
	changed.Spec.Prometheus = promConfig(t, "value3")
	h3 := PodAnnotations(changed)["amazon-cloudwatch-agent-operator-config/sha256"]
	assert.NotEqual(t, h1, h3, "pod annotation hash must change when only Spec.Prometheus changes")

	// metadata annotations hash must also reflect the prometheus change
	a1 := Annotations(base)["amazon-cloudwatch-agent-operator-config/sha256"]
	a3 := Annotations(changed)["amazon-cloudwatch-agent-operator-config/sha256"]
	assert.NotEqual(t, a1, a3, "metadata annotation hash must change when only Spec.Prometheus changes")
}

// TestEmptyPrometheusHashUnchanged asserts that when no Prometheus config is set,
// the hash is byte-identical to the agent-config-only sha256 (non-Prometheus
// agents are unaffected by the fix).
func TestEmptyPrometheusHashUnchanged(t *testing.T) {
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "my-instance", Namespace: "my-ns"},
		Spec:       v1alpha1.AmazonCloudWatchAgentSpec{Config: "test"},
	}

	// sha256("test") == 9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08
	assert.Equal(t, "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
		Annotations(otelcol)["amazon-cloudwatch-agent-operator-config/sha256"])
	assert.Equal(t, "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
		PodAnnotations(otelcol)["amazon-cloudwatch-agent-operator-config/sha256"])
}

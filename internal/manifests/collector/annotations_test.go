// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	colfeaturegate "go.opentelemetry.io/collector/featuregate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/featuregate"
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
						"target_label": "annotationprobe",
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

// TestPrometheusConfigChangeBumpsHash verifies a Prometheus-only change bumps the pod-template hash.
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

// TestEmptyPrometheusHashUnchanged verifies non-Prometheus agents keep a stable hash.
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

const configHashKey = "amazon-cloudwatch-agent-operator-config/sha256"

func taAgent(t *testing.T, name string, taEnabled bool) v1alpha1.AmazonCloudWatchAgent {
	return v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "my-ns"},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			Config:          "agent-config",
			Prometheus:      promConfig(t, "value2"),
			TargetAllocator: v1alpha1.AmazonCloudWatchAgentTargetAllocator{Enabled: taEnabled},
		},
	}
}

// The rendered Prometheus config switches between the user's scrape configs and
// the Target Allocator http_sd form when targetAllocator.enabled flips, so the
// pods must roll even though Spec.Config and Spec.Prometheus are unchanged.
func TestTargetAllocatorEnabledChangeBumpsHash(t *testing.T) {
	off := taAgent(t, "my-instance", false)
	on := taAgent(t, "my-instance", true)
	assert.NotEqual(t, PodAnnotations(off)[configHashKey], PodAnnotations(on)[configHashKey])
	assert.NotEqual(t, Annotations(off)[configHashKey], Annotations(on)[configHashKey])
}

// With the Target Allocator enabled, the rendered config embeds the TA service
// name, which is derived from the instance name.
func TestInstanceNameChangeBumpsHashWithTargetAllocator(t *testing.T) {
	assert.NotEqual(t,
		PodAnnotations(taAgent(t, "a", true))[configHashKey],
		PodAnnotations(taAgent(t, "b", true))[configHashKey])
}

func TestTargetAllocatorRewriteGateChangeBumpsHash(t *testing.T) {
	agent := taAgent(t, "my-instance", true)
	withGate := PodAnnotations(agent)[configHashKey]

	require.NoError(t, colfeaturegate.GlobalRegistry().Set(featuregate.EnableTargetAllocatorRewrite.ID(), false))
	t.Cleanup(func() {
		_ = colfeaturegate.GlobalRegistry().Set(featuregate.EnableTargetAllocatorRewrite.ID(), true)
	})
	assert.NotEqual(t, withGate, PodAnnotations(agent)[configHashKey])
}

// The hash covers exactly the Prometheus data the operator writes to the
// ConfigMap the agent mounts.
func TestConfigHashMatchesRenderedPrometheusConfigMap(t *testing.T) {
	for _, taEnabled := range []bool{false, true} {
		agent := taAgent(t, "my-instance", taEnabled)
		agent.Spec.Config = "{}"
		cfg := config.New()
		cms, err := ConfigMaps(manifests.Params{Config: cfg, OtelCol: agent, Log: logr.Discard()})
		require.NoError(t, err)
		require.Len(t, cms, 2)
		promData := cms[1].Data[cfg.PrometheusConfigMapEntry()]
		require.NotEmpty(t, promData)
		assert.Equal(t, getConfigMapSHA(agent.Spec.Config+"\x00"+promData), PodAnnotations(agent)[configHashKey],
			"taEnabled=%v", taEnabled)
	}
}

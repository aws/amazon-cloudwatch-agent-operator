// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// This file intentionally uses the internal `package collector` (matching
// suite_test.go) rather than the external `package collector_test` used by
// the neighboring legacy `*_test.go` files. The legacy external test files
// all carry `//go:build ignore_test`, which means they are excluded from
// the default test binary; that also means their package-level `logger`
// var is not reliably present in the external-test package on a normal
// `go test` build. Using the internal package here lets these tests reach
// the `logger` defined in suite_test.go and stay independent of the
// legacy files.

package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
)

func TestDaemonsetPodLabels(t *testing.T) {
	testPodLabels := map[string]string{
		"custom-label": "custom-value",
		"team":         "cloudwatch",
	}
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance",
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			PodLabels: testPodLabels,
		},
	}

	params := manifests.Params{
		Config:  config.New(),
		OtelCol: otelcol,
		Log:     logger,
	}
	ds := DaemonSet(params)

	// user labels are present
	assert.Equal(t, "custom-value", ds.Spec.Template.Labels["custom-label"])
	assert.Equal(t, "cloudwatch", ds.Spec.Template.Labels["team"])
	// operator-managed labels are preserved (base-wins merge)
	assert.Equal(t, "amazon-cloudwatch-agent-operator", ds.Spec.Template.Labels["app.kubernetes.io/managed-by"])
	assert.Equal(t, "amazon-cloudwatch-agent", ds.Spec.Template.Labels["app.kubernetes.io/component"])
}

func TestDaemonsetPodLabelsCannotOverrideOperatorManagedLabels(t *testing.T) {
	// User tries to hijack any of the operator-managed labels.
	testPodLabels := map[string]string{
		"app.kubernetes.io/component":  "hijack",
		"app.kubernetes.io/managed-by": "hijack",
		"app.kubernetes.io/instance":   "hijack",
		"app.kubernetes.io/part-of":    "hijack",
		"app.kubernetes.io/name":       "hijack",
	}
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance",
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			PodLabels: testPodLabels,
		},
	}

	params := manifests.Params{
		Config:  config.New(),
		OtelCol: otelcol,
		Log:     logger,
	}
	ds := DaemonSet(params)

	// Operator-managed labels win.
	assert.Equal(t, "amazon-cloudwatch-agent", ds.Spec.Template.Labels["app.kubernetes.io/component"])
	assert.Equal(t, "amazon-cloudwatch-agent-operator", ds.Spec.Template.Labels["app.kubernetes.io/managed-by"])
	assert.NotEqual(t, "hijack", ds.Spec.Template.Labels["app.kubernetes.io/instance"])
	assert.NotEqual(t, "hijack", ds.Spec.Template.Labels["app.kubernetes.io/part-of"])
	assert.NotEqual(t, "hijack", ds.Spec.Template.Labels["app.kubernetes.io/name"])
}

func TestDaemonsetTopologySpreadConstraints(t *testing.T) {
	// Regression test for the DaemonSet bug: Deployment/StatefulSet already
	// applied Spec.TopologySpreadConstraints to the pod spec, but DaemonSet
	// dropped it silently.
	tsc := []corev1.TopologySpreadConstraint{
		{
			MaxSkew:           1,
			TopologyKey:       "kubernetes.io/hostname",
			WhenUnsatisfiable: corev1.DoNotSchedule,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/component": "amazon-cloudwatch-agent",
				},
			},
		},
	}
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance",
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			TopologySpreadConstraints: tsc,
		},
	}

	params := manifests.Params{
		Config:  config.New(),
		OtelCol: otelcol,
		Log:     logger,
	}
	ds := DaemonSet(params)

	assert.Equal(t, tsc, ds.Spec.Template.Spec.TopologySpreadConstraints)
}

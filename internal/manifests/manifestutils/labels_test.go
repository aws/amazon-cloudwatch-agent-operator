// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifestutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
)

const (
	collectorName      = "my-instance"
	collectorNamespace = "my-ns"
)

func TestLabelsCommonSet(t *testing.T) {
	// prepare
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      collectorName,
			Namespace: collectorNamespace,
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			Image: "ghcr.io/aws/amazon-cloudwatch-agent-operator/amazon-cloudwatch-agent-operator:0.47.0",
		},
	}

	// test
	labels := Labels(otelcol.ObjectMeta, collectorName, otelcol.Spec.Image, "amazon-cloudwatch-agent", []string{})
	assert.Equal(t, "amazon-cloudwatch-agent-operator", labels["app.kubernetes.io/managed-by"])
	assert.Equal(t, "my-ns.my-instance", labels["app.kubernetes.io/instance"])
	assert.Equal(t, "0.47.0", labels["app.kubernetes.io/version"])
	assert.Equal(t, "amazon-cloudwatch-agent", labels["app.kubernetes.io/part-of"])
	assert.Equal(t, "amazon-cloudwatch-agent", labels["app.kubernetes.io/component"])
}
func TestLabelsSha256Set(t *testing.T) {
	// prepare
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      collectorName,
			Namespace: collectorNamespace,
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			Image: "ghcr.io/aws/amazon-cloudwatch-agent-operator/amazon-cloudwatch-agent-operator@sha256:c6671841470b83007e0553cdadbc9d05f6cfe17b3ebe9733728dc4a579a5b532",
		},
	}

	// test
	labels := Labels(otelcol.ObjectMeta, collectorName, otelcol.Spec.Image, "amazon-cloudwatch-agent", []string{})
	assert.Equal(t, "amazon-cloudwatch-agent-operator", labels["app.kubernetes.io/managed-by"])
	assert.Equal(t, "my-ns.my-instance", labels["app.kubernetes.io/instance"])
	assert.Equal(t, "c6671841470b83007e0553cdadbc9d05f6cfe17b3ebe9733728dc4a579a5b53", labels["app.kubernetes.io/version"])
	assert.Equal(t, "amazon-cloudwatch-agent", labels["app.kubernetes.io/part-of"])
	assert.Equal(t, "amazon-cloudwatch-agent", labels["app.kubernetes.io/component"])

	// prepare
	otelcolTag := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      collectorName,
			Namespace: collectorNamespace,
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			Image: "ghcr.io/aws/amazon-cloudwatch-agent-operator/amazon-cloudwatch-agent-operator:0.81.0@sha256:c6671841470b83007e0553cdadbc9d05f6cfe17b3ebe9733728dc4a579a5b532",
		},
	}

	// test
	labelsTag := Labels(otelcolTag.ObjectMeta, collectorName, otelcolTag.Spec.Image, "amazon-cloudwatch-agent", []string{})
	assert.Equal(t, "amazon-cloudwatch-agent-operator", labelsTag["app.kubernetes.io/managed-by"])
	assert.Equal(t, "my-ns.my-instance", labelsTag["app.kubernetes.io/instance"])
	assert.Equal(t, "0.81.0", labelsTag["app.kubernetes.io/version"])
	assert.Equal(t, "amazon-cloudwatch-agent", labelsTag["app.kubernetes.io/part-of"])
	assert.Equal(t, "amazon-cloudwatch-agent", labelsTag["app.kubernetes.io/component"])
}
func TestLabelsTagUnset(t *testing.T) {
	// prepare
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      collectorName,
			Namespace: collectorNamespace,
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			Image: "ghcr.io/aws/amazon-cloudwatch-agent-operator/amazon-cloudwatch-agent-operator",
		},
	}

	// test
	labels := Labels(otelcol.ObjectMeta, collectorName, otelcol.Spec.Image, "amazon-cloudwatch-agent", []string{})
	assert.Equal(t, "amazon-cloudwatch-agent-operator", labels["app.kubernetes.io/managed-by"])
	assert.Equal(t, "my-ns.my-instance", labels["app.kubernetes.io/instance"])
	assert.Equal(t, "latest", labels["app.kubernetes.io/version"])
	assert.Equal(t, "amazon-cloudwatch-agent", labels["app.kubernetes.io/part-of"])
	assert.Equal(t, "amazon-cloudwatch-agent", labels["app.kubernetes.io/component"])
}

func TestLabelsPropagateDown(t *testing.T) {
	// prepare
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"myapp":                  "mycomponent",
				"app.kubernetes.io/name": "test",
			},
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			Image: "ghcr.io/aws/amazon-cloudwatch-agent-operator/amazon-cloudwatch-agent-operator",
		},
	}

	// test
	labels := Labels(otelcol.ObjectMeta, collectorName, otelcol.Spec.Image, "amazon-cloudwatch-agent", []string{})

	// verify
	assert.Len(t, labels, 7)
	assert.Equal(t, "mycomponent", labels["myapp"])
	assert.Equal(t, "test", labels["app.kubernetes.io/name"])
}

func TestLabelsFilter(t *testing.T) {
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"test.bar.io": "foo", "test.foo.io": "bar"},
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			Image: "ghcr.io/aws/amazon-cloudwatch-agent-operator/amazon-cloudwatch-agent-operator",
		},
	}

	// This requires the filter to be in regex match form and not the other simpler wildcard one.
	labels := Labels(otelcol.ObjectMeta, collectorName, otelcol.Spec.Image, "amazon-cloudwatch-agent", []string{".*.bar.io"})

	// verify
	assert.Len(t, labels, 7)
	assert.NotContains(t, labels, "test.bar.io")
	assert.Equal(t, "bar", labels["test.foo.io"])
}

func TestSelectorLabels(t *testing.T) {
	// prepare
	expected := map[string]string{
		"app.kubernetes.io/component":  "amazon-cloudwatch-agent",
		"app.kubernetes.io/instance":   "my-namespace.my-amazon-cloudwatch-agent",
		"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
		"app.kubernetes.io/part-of":    "amazon-cloudwatch-agent",
	}
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "my-amazon-cloudwatch-agent", Namespace: "my-namespace"},
	}

	// test
	result := SelectorLabels(otelcol.ObjectMeta, "amazon-cloudwatch-agent")

	// verify
	assert.Equal(t, expected, result)
}

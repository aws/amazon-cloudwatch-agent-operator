// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

const (
	name      = "my-instance"
	namespace = "my-ns"
)

func TestLabelsCommonSet(t *testing.T) {
	// prepare
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	// test
	labels := Labels(otelcol, name)
	assert.Equal(t, "amazon-cloudwatch-agent-operator", labels["app.kubernetes.io/managed-by"])
	assert.Equal(t, "my-ns.my-instance", labels["app.kubernetes.io/instance"])
	assert.Equal(t, "opentelemetry", labels["app.kubernetes.io/part-of"])
	assert.Equal(t, "opentelemetry-targetallocator", labels["app.kubernetes.io/component"])
	assert.Equal(t, "latest", labels["app.kubernetes.io/version"])
	assert.Equal(t, name, labels["app.kubernetes.io/name"])
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
	}

	// test
	labels := Labels(otelcol, name)

	// verify
	assert.Len(t, labels, 7)
	assert.Equal(t, "mycomponent", labels["myapp"])
	assert.Equal(t, "test", labels["app.kubernetes.io/name"])
}

func TestSelectorLabels(t *testing.T) {
	// prepare
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	// test
	labels := SelectorLabels(otelcol)
	assert.Equal(t, "amazon-cloudwatch-agent-operator", labels["app.kubernetes.io/managed-by"])
	assert.Equal(t, "my-ns.my-instance", labels["app.kubernetes.io/instance"])
	assert.Equal(t, "opentelemetry", labels["app.kubernetes.io/part-of"])
	assert.Equal(t, "opentelemetry-targetallocator", labels["app.kubernetes.io/component"])
	assert.Equal(t, naming.TargetAllocator(otelcol.Name), labels["app.kubernetes.io/name"])
}

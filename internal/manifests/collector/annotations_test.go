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
	otelcol := v1alpha1.OpenTelemetryCollector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "my-ns",
		},
		Spec: v1alpha1.OpenTelemetryCollectorSpec{
			Config: "test",
		},
	}

	// test
	annotations := Annotations(otelcol)
	podAnnotations := PodAnnotations(otelcol)

	//verify
	assert.Equal(t, "true", annotations["prometheus.io/scrape"])
	assert.Equal(t, "8888", annotations["prometheus.io/port"])
	assert.Equal(t, "/metrics", annotations["prometheus.io/path"])
	assert.Equal(t, "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", annotations["opentelemetry-operator-config/sha256"])
	//verify propagation from metadata.annotations to spec.template.spec.metadata.annotations
	assert.Equal(t, "true", podAnnotations["prometheus.io/scrape"])
	assert.Equal(t, "8888", podAnnotations["prometheus.io/port"])
	assert.Equal(t, "/metrics", podAnnotations["prometheus.io/path"])
	assert.Equal(t, "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", podAnnotations["opentelemetry-operator-config/sha256"])
}

func TestUserAnnotations(t *testing.T) {
	// prepare
	otelcol := v1alpha1.OpenTelemetryCollector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "my-ns",
			Annotations: map[string]string{"prometheus.io/scrape": "false",
				"prometheus.io/port":                   "1234",
				"prometheus.io/path":                   "/test",
				"opentelemetry-operator-config/sha256": "shouldBeOverwritten",
			},
		},
		Spec: v1alpha1.OpenTelemetryCollectorSpec{
			Config: "test",
		},
	}

	// test
	annotations := Annotations(otelcol)
	podAnnotations := PodAnnotations(otelcol)

	//verify
	assert.Equal(t, "false", annotations["prometheus.io/scrape"])
	assert.Equal(t, "1234", annotations["prometheus.io/port"])
	assert.Equal(t, "/test", annotations["prometheus.io/path"])
	assert.Equal(t, "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", annotations["opentelemetry-operator-config/sha256"])
	assert.Equal(t, "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", podAnnotations["opentelemetry-operator-config/sha256"])
}

func TestAnnotationsPropagateDown(t *testing.T) {
	// prepare
	otelcol := v1alpha1.OpenTelemetryCollector{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"myapp": "mycomponent"},
		},
		Spec: v1alpha1.OpenTelemetryCollectorSpec{
			PodAnnotations: map[string]string{"pod_annotation": "pod_annotation_value"},
		},
	}

	// test
	annotations := Annotations(otelcol)
	podAnnotations := PodAnnotations(otelcol)

	// verify
	assert.Len(t, annotations, 5)
	assert.Equal(t, "mycomponent", annotations["myapp"])
	assert.Equal(t, "mycomponent", podAnnotations["myapp"])
	assert.Equal(t, "pod_annotation_value", podAnnotations["pod_annotation"])
}

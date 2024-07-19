// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dcgmexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
)

func TestDefaultAnnotations(t *testing.T) {
	// prepare
	exporter := v1alpha1.DcgmExporter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "my-ns",
		},
		Spec: v1alpha1.DcgmExporterSpec{},
	}
	// test
	annotations := Annotations(exporter)

	//verify
	assert.Equal(t, "dcgm-exporter", annotations["k8s-app"])
	assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", annotations["amazon-cloudwatch-agent-operator-config/sha256"])
}

func TestUserAnnotations(t *testing.T) {
	// prepare
	exporter := v1alpha1.DcgmExporter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "my-ns",
			Annotations: map[string]string{
				"prometheus.io/test":                             "test",
				"amazon-cloudwatch-agent-operator-config/sha256": "shouldBeOverwritten",
			},
		},
		Spec: v1alpha1.DcgmExporterSpec{},
	}

	// test
	annotations := Annotations(exporter)

	//verify
	assert.Equal(t, "test", annotations["prometheus.io/test"])
	assert.Equal(t, "dcgm-exporter", annotations["k8s-app"])
	assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", annotations["amazon-cloudwatch-agent-operator-config/sha256"])
}

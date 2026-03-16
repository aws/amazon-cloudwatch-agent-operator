// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package nodeexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
)

func TestVolumeNewDefault(t *testing.T) {
	exporter := v1alpha1.NodeExporter{}
	volumes := Volumes(exporter)
	assert.Len(t, volumes, 1)
	assert.Equal(t, NodeExporterConfigMapVolumeName, volumes[0].Name)
}

func TestVolumeAllowsMoreToBeAdded(t *testing.T) {
	exporter := v1alpha1.NodeExporter{
		Spec: v1alpha1.NodeExporterSpec{
			Volumes: []corev1.Volume{{
				Name: "my-volume",
			}},
		},
	}
	volumes := Volumes(exporter)
	assert.Len(t, volumes, 2)
	assert.Equal(t, "my-volume", volumes[0].Name)
	assert.Equal(t, NodeExporterConfigMapVolumeName, volumes[1].Name)
}

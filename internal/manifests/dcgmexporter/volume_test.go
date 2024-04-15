// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dcgmexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
)

func TestVolumeNewDefault(t *testing.T) {
	exporter := v1alpha1.DcgmExporter{}
	volumes := Volumes(exporter)
	assert.Len(t, volumes, 1)
	assert.Equal(t, DcgmConfigMapVolumeName, volumes[0].Name)
}

func TestVolumeAllowsMoreToBeAdded(t *testing.T) {
	exporter := v1alpha1.DcgmExporter{
		Spec: v1alpha1.DcgmExporterSpec{
			Volumes: []corev1.Volume{{
				Name: "my-volume",
			}},
		},
	}
	volumes := Volumes(exporter)
	assert.Len(t, volumes, 2)
	assert.Equal(t, "my-volume", volumes[0].Name)
	assert.Equal(t, DcgmConfigMapVolumeName, volumes[1].Name)
}

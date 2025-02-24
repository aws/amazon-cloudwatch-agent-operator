// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

func TestVolumeNewDefault(t *testing.T) {
	// prepare
	otelcol := v1alpha1.AmazonCloudWatchAgent{}
	cfg := config.New()

	// test
	volumes := Volumes(cfg, otelcol)

	// verify
	assert.Len(t, volumes, 3)

	// check if the number of elements in the volume source items list is 1
	assert.Len(t, volumes[0].VolumeSource.ConfigMap.Items, 1)

	// check that it's the ta-internal volume, with the config map
	assert.Equal(t, naming.TAConfigMapVolume(), volumes[0].Name)
}

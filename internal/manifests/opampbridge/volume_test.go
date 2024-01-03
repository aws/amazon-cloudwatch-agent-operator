// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package opampbridge

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

func TestVolumeNewDefault(t *testing.T) {
	// prepare
	opampBridge := v1alpha1.OpAMPBridge{}
	cfg := config.New()

	// test
	volumes := Volumes(cfg, opampBridge)

	// verify
	assert.Len(t, volumes, 1)

	// check if the number of elements in the volume source items list is 1
	assert.Len(t, volumes[0].VolumeSource.ConfigMap.Items, 1)

	// check that it's the opamp-bridge-internal volume, with the config map
	assert.Equal(t, naming.OpAMPBridgeConfigMapVolume(), volumes[0].Name)
}

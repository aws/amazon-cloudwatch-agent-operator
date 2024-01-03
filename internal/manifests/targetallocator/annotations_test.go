// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
)

func TestPodAnnotations(t *testing.T) {
	instance := collectorInstance()
	instance.Spec.PodAnnotations = map[string]string{
		"key": "value",
	}
	annotations := Annotations(instance, nil)
	assert.Subset(t, annotations, instance.Spec.PodAnnotations)
}

func TestConfigMapHash(t *testing.T) {
	cfg := config.New()
	instance := collectorInstance()
	params := manifests.Params{
		OtelCol: instance,
		Config:  cfg,
		Log:     logr.Discard(),
	}
	expectedConfigMap, err := ConfigMap(params)
	require.NoError(t, err)
	expectedConfig := expectedConfigMap.Data[targetAllocatorFilename]
	require.NotEmpty(t, expectedConfig)
	expectedHash := sha256.Sum256([]byte(expectedConfig))
	annotations := Annotations(instance, expectedConfigMap)
	require.Contains(t, annotations, configMapHashAnnotationKey)
	cmHash := annotations[configMapHashAnnotationKey]
	assert.Equal(t, fmt.Sprintf("%x", expectedHash), cmHash)
}

func TestInvalidConfigNoHash(t *testing.T) {
	instance := collectorInstance()
	instance.Spec.Config = ""
	annotations := Annotations(instance, nil)
	require.NotContains(t, annotations, configMapHashAnnotationKey)
}

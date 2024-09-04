// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	. "github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

func TestVolumeNewDefault(t *testing.T) {
	// prepare
	otelcol := v1alpha1.AmazonCloudWatchAgent{}
	cfg := config.New()

	// test
	volumes := Volumes(cfg, otelcol)

	// verify
	assert.Len(t, volumes, 1)

	// check that it's the otc-internal volume, with the config map
	assert.Equal(t, naming.ConfigMapVolume(), volumes[0].Name)
}

func TestVolumeAllowsMoreToBeAdded(t *testing.T) {
	// prepare
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			Volumes: []corev1.Volume{{
				Name: "my-volume",
			}},
		},
	}
	cfg := config.New()

	// test
	volumes := Volumes(cfg, otelcol)

	// verify
	assert.Len(t, volumes, 2)

	// check that it's the otc-internal volume, with the config map
	assert.Equal(t, "my-volume", volumes[1].Name)
}

func TestVolumeWithMoreConfigMaps(t *testing.T) {
	// prepare
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			ConfigMaps: []v1alpha1.ConfigMapsSpec{{
				Name:      "configmap-test",
				MountPath: "/",
			}, {
				Name:      "configmap-test2",
				MountPath: "/dir",
			}},
		},
	}
	cfg := config.New()

	// test
	volumes := Volumes(cfg, otelcol)

	// verify
	assert.Len(t, volumes, 3)

	// check if the volume with the configmap prefix is mounted after defining the config map.
	assert.Equal(t, "configmap-configmap-test", volumes[1].Name)
	assert.Equal(t, "configmap-configmap-test2", volumes[2].Name)
}

func TestVolumePrometheus(t *testing.T) {
	// prepare
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			Prometheus: "test",
		},
	}
	cfg := config.New()

	// test
	volumes := Volumes(cfg, otelcol)

	// verify
	assert.Len(t, volumes, 2)

	// check that it's the otc-internal volume, with the config map
	assert.Equal(t, naming.ConfigMapVolume(), volumes[0].Name)

	// check that the second volume is prometheus-config, with the config map
	assert.Equal(t, naming.PrometheusConfigMapVolume(), volumes[1].Name)
}

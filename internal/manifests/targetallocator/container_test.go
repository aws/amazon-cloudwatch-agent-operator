// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

var logger = logf.Log.WithName("unit-tests")

func TestContainerNewDefault(t *testing.T) {
	// prepare
	otelcol := v1alpha1.AmazonCloudWatchAgent{}
	cfg := config.New(config.WithTargetAllocatorImage("default-image"))

	// test
	c := Container(cfg, logger, otelcol)

	// verify
	assert.Equal(t, "default-image", c.Image)
}

func TestContainerWithImageOverridden(t *testing.T) {
	// prepare
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			TargetAllocator: v1alpha1.AmazonCloudWatchAgentTargetAllocator{
				Enabled: true,
				Image:   "overridden-image",
			},
		},
	}
	cfg := config.New(config.WithTargetAllocatorImage("default-image"))

	// test
	c := Container(cfg, logger, otelcol)

	// verify
	assert.Equal(t, "overridden-image", c.Image)
}

func TestContainerPorts(t *testing.T) {
	// prepare
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			TargetAllocator: v1alpha1.AmazonCloudWatchAgentTargetAllocator{
				Enabled: true,
				Image:   "default-image",
			},
		},
	}
	cfg := config.New()

	// test
	c := Container(cfg, logger, otelcol)

	// verify
	assert.Len(t, c.Ports, 1)
	assert.Equal(t, "https", c.Ports[0].Name)
	assert.Equal(t, int32(naming.TargetAllocatorContainerPort), c.Ports[0].ContainerPort)
}

func TestContainerVolumes(t *testing.T) {
	// prepare
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			TargetAllocator: v1alpha1.AmazonCloudWatchAgentTargetAllocator{
				Enabled: true,
				Image:   "default-image",
			},
		},
	}
	cfg := config.New()

	// test
	c := Container(cfg, logger, otelcol)

	// verify
	assert.Len(t, c.VolumeMounts, 2)
	assert.Equal(t, naming.TAConfigMapVolume(), c.VolumeMounts[0].Name)
}

func TestContainerResourceRequirements(t *testing.T) {
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			TargetAllocator: v1alpha1.AmazonCloudWatchAgentTargetAllocator{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128M"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("256M"),
					},
				},
			},
		},
	}

	cfg := config.New()
	resourceTest := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("128M"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("200m"),
			corev1.ResourceMemory: resource.MustParse("256M"),
		},
	}
	// test
	c := Container(cfg, logger, otelcol)
	resourcesValues := c.Resources

	// verify
	assert.Equal(t, resourceTest, resourcesValues)
}

func TestContainerHasEnvVars(t *testing.T) {
	// prepare
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			TargetAllocator: v1alpha1.AmazonCloudWatchAgentTargetAllocator{
				Enabled: true,
				Env: []corev1.EnvVar{
					{
						Name:  "TEST_ENV",
						Value: "test",
					},
				},
			},
		},
	}
	cfg := config.New(config.WithTargetAllocatorImage("default-image"))

	expected := corev1.Container{
		Name:  "ta-container",
		Image: "default-image",
		Env: []corev1.EnvVar{
			{
				Name:  "TEST_ENV",
				Value: "test",
			},
			{
				Name:  "OTELCOL_NAMESPACE",
				Value: "",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						APIVersion: "",
						FieldPath:  "metadata.namespace",
					},
					ResourceFieldRef: nil,
					ConfigMapKeyRef:  nil,
					SecretKeyRef:     nil,
				},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:             "ta-internal",
				ReadOnly:         false,
				MountPath:        "/conf",
				SubPath:          "",
				MountPropagation: nil,
				SubPathExpr:      "",
			},
			{
				Name:      "ta-secret",
				ReadOnly:  true,
				MountPath: TACertMountPath,
			},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "https",
				ContainerPort: naming.TargetAllocatorContainerPort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
	}

	// test
	c := Container(cfg, logger, otelcol)

	// verify
	assert.Equal(t, expected, c)
}

func TestContainerDoesNotOverrideEnvVars(t *testing.T) {
	// prepare
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			TargetAllocator: v1alpha1.AmazonCloudWatchAgentTargetAllocator{
				Enabled: true,
				Env: []corev1.EnvVar{
					{
						Name:  "OTELCOL_NAMESPACE",
						Value: "test",
					},
				},
			},
		},
	}
	cfg := config.New(config.WithTargetAllocatorImage("default-image"))

	expected := corev1.Container{
		Name:  "ta-container",
		Image: "default-image",
		Env: []corev1.EnvVar{
			{
				Name:  "OTELCOL_NAMESPACE",
				Value: "test",
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:             "ta-internal",
				ReadOnly:         false,
				MountPath:        "/conf",
				SubPath:          "",
				MountPropagation: nil,
				SubPathExpr:      "",
			},
			{
				Name:      "ta-secret",
				ReadOnly:  true,
				MountPath: TACertMountPath,
			},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "https",
				ContainerPort: naming.TargetAllocatorContainerPort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
	}

	// test
	c := Container(cfg, logger, otelcol)

	// verify
	assert.Equal(t, expected, c)
}

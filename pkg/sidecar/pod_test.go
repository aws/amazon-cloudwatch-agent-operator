// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build ignore_test

package sidecar

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

var logger = logf.Log.WithName("unit-tests")

func TestAddSidecarWhenNoSidecarExists(t *testing.T) {
	// prepare
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "my-app"},
			},
			InitContainers: []corev1.Container{
				{
					Name: "my-init",
				},
			},
			// cross-test: the pod has a volume already, make sure we don't remove it
			Volumes: []corev1.Volume{{}},
		},
	}

	otelcol := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "otelcol-sample-with-a-name-that-is-longer-than-sixty-three-characters",
			Namespace: "some-app",
		},

		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			AmazonCloudWatchAgentCommonFields: v1alpha1.AmazonCloudWatchAgentCommonFields{
				Ports: []v1alpha1.PortsSpec{
					{
						ServicePort: corev1.ServicePort{
							Name:     "metrics",
							Port:     8888,
							Protocol: corev1.ProtocolTCP,
						},
					},
				},
				InitContainers: []corev1.Container{
					{
						Name: "test",
					},
				},
			},
		},
	}

	otelcolYaml, err := otelcol.Spec.Config.Yaml()
	require.NoError(t, err)
	cfg := config.New(config.WithCollectorImage("some-default-image"))

	// test
	changed, err := add(cfg, logger, otelcol, pod, nil)

	// verify
	assert.NoError(t, err)
	require.Len(t, changed.Spec.Containers, 2)
	require.Len(t, changed.Spec.InitContainers, 2)
	require.Len(t, changed.Spec.Volumes, 1)
	assert.Equal(t, "otelcol-sample-with-a-name-that-is-longer-than-sixty-three-cha",
		changed.Labels["sidecar.opentelemetry.io/injected"])
	assert.Equal(t, corev1.Container{
		Name:  "otc-container",
		Image: "some-default-image",
		Args:  []string{"--config=env:OTEL_CONFIG"},
		Env: []corev1.EnvVar{
			{
				Name: "POD_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
			{
				Name:  "OTEL_CONFIG",
				Value: string(otelcolYaml),
			},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "metrics",
				ContainerPort: 8888,
				Protocol:      corev1.ProtocolTCP,
			},
		},
	}, changed.Spec.Containers[1])
}

// this situation should never happen in the current code path, but it should not fail
// if it's asked to add a new sidecar. The caller is expected to have called existsIn before.
func TestAddSidecarWhenOneExistsAlready(t *testing.T) {
	// prepare
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "my-app"},
				{Name: naming.Container()},
			},
		},
	}
	otelcol := v1alpha1.AmazonCloudWatchAgent{}
	cfg := config.New(config.WithCollectorImage("some-default-image"))

	// test
	changed, err := add(cfg, logger, otelcol, pod, nil)

	// verify
	assert.NoError(t, err)
	assert.Len(t, changed.Spec.Containers, 3)
}

func TestRemoveSidecar(t *testing.T) {
	// prepare
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "my-app"},
				{Name: naming.Container()},
				{Name: naming.Container()}, // two sidecars! should remove both
			},
		},
	}

	// test
	changed := remove(pod)

	// verify
	assert.Len(t, changed.Spec.Containers, 1)
}

func TestRemoveNonExistingSidecar(t *testing.T) {
	// prepare
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "my-app"},
			},
		},
	}

	// test
	changed := remove(pod)

	// verify
	assert.Len(t, changed.Spec.Containers, 1)
}

func TestExistsIn(t *testing.T) {
	for _, tt := range []struct {
		desc     string
		pod      corev1.Pod
		expected bool
	}{
		{"has-sidecar",
			corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "my-app"},
						{Name: naming.Container()},
					},
				},
			},
			true},

		{"does-not-have-sidecar",
			corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{},
				},
			},
			false},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			assert.Equal(t, tt.expected, existsIn(tt.pod))
		})
	}
}

func TestAddSidecarWithAditionalEnv(t *testing.T) {
	// prepare
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "my-app"},
			},
		},
	}

	otelcol := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "otelcol-sample",
			Namespace: "some-app",
		},
	}
	cfg := config.New(config.WithCollectorImage("some-default-image"))

	extraEnv := corev1.EnvVar{
		Name:  "extraenv",
		Value: "extravalue",
	}

	// test
	changed, err := add(cfg, logger, otelcol, pod, []corev1.EnvVar{
		extraEnv,
	})

	// verify
	assert.NoError(t, err)
	assert.Len(t, changed.Spec.Containers, 2)
	assert.Contains(t, changed.Spec.Containers[1].Env, extraEnv)

}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package neuronmonitor

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
)

func TestNeuronContainer(t *testing.T) {
	logger := logf.Log.WithName("unit-tests")
	testCases := []struct {
		name     string
		exporter v1alpha1.NeuronMonitor
		expected corev1.Container
	}{
		{
			name: "default",
			exporter: v1alpha1.NeuronMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "my-ns",
				},
				Spec: v1alpha1.NeuronMonitorSpec{
					Image: "test-image",
					Command: []string{
						"testCommand",
					},
					Args: map[string]string{
						"args1": "val1",
					},
					Env: []corev1.EnvVar{
						{
							Name:  "test",
							Value: "val",
						},
					},
					Ports: []corev1.ServicePort{
						{
							Name:     "test",
							Port:     8000,
							Protocol: "TCP",
						},
					},
				},
			},
			expected: corev1.Container{
				Name:  ComponentNeuronExporter,
				Image: "test-image",
				Command: []string{
					"testCommand",
				},
				Args: []string{
					"--args1", "val1",
					"--neuron-monitor-config", fmt.Sprintf("%s/%s", configmapMountPath, NeuronMonitorJson),
				},
				Env: []corev1.EnvVar{
					{
						Name:  "test",
						Value: "val",
					},
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          "test",
						ContainerPort: 8000,
						Protocol:      "TCP",
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      NeuronConfigMapVolumeName,
						MountPath: configmapMountPath,
					},
				},
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path:   "/healthz",
							Port:   intstr.FromInt32(8000),
							Scheme: corev1.URISchemeHTTPS,
						},
					},
					InitialDelaySeconds: 15,
					PeriodSeconds:       10,
					TimeoutSeconds:      5,
					FailureThreshold:    3,
				},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path:   "/healthz",
							Port:   intstr.FromInt32(8000),
							Scheme: corev1.URISchemeHTTPS,
						},
					},
					InitialDelaySeconds: 5,
					PeriodSeconds:       10,
					TimeoutSeconds:      5,
					FailureThreshold:    3,
				},
			},
		},
	}

	for _, tc := range testCases {
		container := Container(config.Config{}, logger, tc.exporter)
		assert.Equal(t, tc.expected, container)
	}
}
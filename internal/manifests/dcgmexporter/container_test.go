// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dcgmexporter

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1beta1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
)

func TestDcgmContainer(t *testing.T) {
	logger := logf.Log.WithName("unit-tests")
	testCases := []struct {
		name     string
		exporter v1beta1.DcgmExporter
		expected corev1.Container
	}{
		{
			name: "default",
			exporter: v1beta1.DcgmExporter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "my-ns",
				},
				Spec: v1beta1.DcgmExporterSpec{
					Image: "test-image",
					Ports: []corev1.ServicePort{
						{
							Name:     "test",
							Port:     9400,
							Protocol: "TCP",
						},
					},
				},
			},
			expected: corev1.Container{
				Name:  ComponentDcgmExporter,
				Image: "test-image",
				Args:  nil,
				Env: []corev1.EnvVar{
					{
						Name:  metricsConfigEnvVar,
						Value: fmt.Sprintf("%s/%s", configmapMountPath, DcgmMetricsIncludedCsv),
					},
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          "test",
						ContainerPort: 9400,
						Protocol:      "TCP",
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      DcgmConfigMapVolumeName,
						MountPath: configmapMountPath,
					},
				},
			},
		},
		{
			name: "tls",
			exporter: v1beta1.DcgmExporter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "my-ns",
				},
				Spec: v1beta1.DcgmExporterSpec{
					Image: "test-image",
					Ports: []corev1.ServicePort{
						{
							Name:     "test",
							Port:     9400,
							Protocol: "TCP",
						},
					},
					TlsConfig: `tls_server_config:  cert_file: /etc/amazon-cloudwatch-observability-dcgm-cert/server.crt`,
				},
			},
			expected: corev1.Container{
				Name:  ComponentDcgmExporter,
				Image: "test-image",
				Args: []string{
					"--web-config-file=" + fmt.Sprintf("%s/%s", configmapMountPath, DcgmWebConfigYaml),
				},
				Env: []corev1.EnvVar{
					{
						Name:  metricsConfigEnvVar,
						Value: fmt.Sprintf("%s/%s", configmapMountPath, DcgmMetricsIncludedCsv),
					},
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          "test",
						ContainerPort: 9400,
						Protocol:      "TCP",
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      DcgmConfigMapVolumeName,
						MountPath: configmapMountPath,
					},
				},
			},
		},
		{
			name: "tlsWithExtraEnvs",
			exporter: v1beta1.DcgmExporter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "my-ns",
				},
				Spec: v1beta1.DcgmExporterSpec{
					Image: "test-image",
					Ports: []corev1.ServicePort{
						{
							Name:     "test",
							Port:     9400,
							Protocol: "TCP",
						},
					},
					TlsConfig: `tls_server_config:  cert_file: /etc/amazon-cloudwatch-observability-dcgm-cert/server.crt`,
					Args:      map[string]string{"another": "test"},
				},
			},
			expected: corev1.Container{
				Name:  ComponentDcgmExporter,
				Image: "test-image",
				Args: []string{
					"--another=test",
					"--web-config-file=" + fmt.Sprintf("%s/%s", configmapMountPath, DcgmWebConfigYaml),
				},
				Env: []corev1.EnvVar{
					{
						Name:  metricsConfigEnvVar,
						Value: fmt.Sprintf("%s/%s", configmapMountPath, DcgmMetricsIncludedCsv),
					},
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          "test",
						ContainerPort: 9400,
						Protocol:      "TCP",
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      DcgmConfigMapVolumeName,
						MountPath: configmapMountPath,
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		container := Container(config.Config{}, logger, tc.exporter)
		assert.Equal(t, tc.expected, container)
	}
}

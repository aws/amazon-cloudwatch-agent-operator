// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package nodeexporter

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
)

func TestNodeExporterContainer(t *testing.T) {
	logger := logf.Log.WithName("unit-tests")
	testCases := []struct {
		name     string
		exporter v1alpha1.NodeExporter
		cfg      config.Config
		expected corev1.Container
	}{
		{
			name: "default",
			exporter: v1alpha1.NodeExporter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "my-ns",
				},
				Spec: v1alpha1.NodeExporterSpec{
					Image: "test-image",
				},
			},
			cfg: config.Config{},
			expected: corev1.Container{
				Name:  ComponentNodeExporter,
				Image: "test-image",
				Args: []string{
					"--path.procfs=/host/proc",
					"--path.rootfs=/host/root",
					"--path.sysfs=/host/sys",
					"--web.listen-address=:9100",
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          "metrics",
						ContainerPort: 9100,
						HostPort:      9100,
						Protocol:      corev1.ProtocolTCP,
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      NodeExporterConfigMapVolumeName,
						MountPath: configmapMountPath,
						ReadOnly:  true,
					},
				},
			},
		},
		{
			name: "tls",
			exporter: v1alpha1.NodeExporter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "my-ns",
				},
				Spec: v1alpha1.NodeExporterSpec{
					Image:     "test-image",
					TlsConfig: `tls_server_config:  cert_file: /etc/node-exporter-cert/server.crt`,
				},
			},
			cfg: config.Config{},
			expected: corev1.Container{
				Name:  ComponentNodeExporter,
				Image: "test-image",
				Args: []string{
					"--path.procfs=/host/proc",
					"--path.rootfs=/host/root",
					"--path.sysfs=/host/sys",
					fmt.Sprintf("--web.config.file=%s/%s", configmapMountPath, NodeExporterWebConfigYaml),
					"--web.listen-address=:9100",
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          "metrics",
						ContainerPort: 9100,
						HostPort:      9100,
						Protocol:      corev1.ProtocolTCP,
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      NodeExporterConfigMapVolumeName,
						MountPath: configmapMountPath,
						ReadOnly:  true,
					},
				},
			},
		},
		{
			name: "tlsWithCustomArgs",
			exporter: v1alpha1.NodeExporter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "my-ns",
				},
				Spec: v1alpha1.NodeExporterSpec{
					Image:     "test-image",
					TlsConfig: `tls_server_config:  cert_file: /etc/node-exporter-cert/server.crt`,
					Args:      map[string]string{"collector.cpu": "true"},
				},
			},
			cfg: config.Config{},
			expected: corev1.Container{
				Name:  ComponentNodeExporter,
				Image: "test-image",
				Args: []string{
					"--collector.cpu=true",
					"--path.procfs=/host/proc",
					"--path.rootfs=/host/root",
					"--path.sysfs=/host/sys",
					fmt.Sprintf("--web.config.file=%s/%s", configmapMountPath, NodeExporterWebConfigYaml),
					"--web.listen-address=:9100",
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          "metrics",
						ContainerPort: 9100,
						HostPort:      9100,
						Protocol:      corev1.ProtocolTCP,
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      NodeExporterConfigMapVolumeName,
						MountPath: configmapMountPath,
						ReadOnly:  true,
					},
				},
			},
		},
		{
			name: "imageFallback",
			exporter: v1alpha1.NodeExporter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "my-ns",
				},
				Spec: v1alpha1.NodeExporterSpec{},
			},
			cfg: config.New(config.WithNodeExporterImage("fallback-image:latest")),
			expected: corev1.Container{
				Name:  ComponentNodeExporter,
				Image: "fallback-image:latest",
				Args: []string{
					"--path.procfs=/host/proc",
					"--path.rootfs=/host/root",
					"--path.sysfs=/host/sys",
					"--web.listen-address=:9100",
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          "metrics",
						ContainerPort: 9100,
						HostPort:      9100,
						Protocol:      corev1.ProtocolTCP,
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      NodeExporterConfigMapVolumeName,
						MountPath: configmapMountPath,
						ReadOnly:  true,
					},
				},
			},
		},
		{
			name: "portOverride",
			exporter: v1alpha1.NodeExporter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-instance",
					Namespace: "my-ns",
				},
				Spec: v1alpha1.NodeExporterSpec{
					Image: "test-image",
					Ports: []corev1.ServicePort{
						{
							Name:     "custom",
							Port:     9200,
							Protocol: corev1.ProtocolTCP,
						},
					},
				},
			},
			cfg: config.Config{},
			expected: corev1.Container{
				Name:  ComponentNodeExporter,
				Image: "test-image",
				Args: []string{
					"--path.procfs=/host/proc",
					"--path.rootfs=/host/root",
					"--path.sysfs=/host/sys",
					"--web.listen-address=:9100",
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          "custom",
						ContainerPort: 9200,
						HostPort:      9200,
						Protocol:      corev1.ProtocolTCP,
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      NodeExporterConfigMapVolumeName,
						MountPath: configmapMountPath,
						ReadOnly:  true,
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			container := Container(tc.cfg, logger, tc.exporter)
			assert.Equal(t, tc.expected, container)
		})
	}
}

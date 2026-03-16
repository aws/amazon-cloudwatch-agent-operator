// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package nodeexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/tools/record"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
)

func TestDesiredConfigMapWithTlsConfig(t *testing.T) {
	expectedLabels := map[string]string{
		"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
		"app.kubernetes.io/instance":   "default.test",
		"app.kubernetes.io/part-of":    "amazon-cloudwatch-agent",
		"app.kubernetes.io/version":    "0.1.0",
		"app.kubernetes.io/component":  "node-exporter",
		"app.kubernetes.io/name":       "node-exporter-config-map",
	}

	t.Run("should return config map with web.yml when TlsConfig is set", func(t *testing.T) {
		expectedData := map[string]string{
			"web.yml": `tls_server_config:
  cert_file: /etc/node-exporter-cert/server.crt`,
		}

		param := getParams()
		param.NodeExp.Spec.TlsConfig = "tls_server_config:\n  cert_file: /etc/node-exporter-cert/server.crt"
		actual, err := ConfigMap(param)

		assert.NoError(t, err)
		assert.Equal(t, "node-exporter-config-map", actual.Name)
		assert.Equal(t, "default", actual.Namespace)
		assert.Equal(t, expectedLabels, actual.Labels)
		assert.Equal(t, expectedData, actual.Data)
	})
}

func TestDesiredConfigMapWithoutTlsConfig(t *testing.T) {
	t.Run("should return config map with empty data when TlsConfig is empty", func(t *testing.T) {
		param := getParams()
		actual, err := ConfigMap(param)

		assert.NoError(t, err)
		assert.Equal(t, "node-exporter-config-map", actual.Name)
		assert.Equal(t, "default", actual.Namespace)
		assert.Empty(t, actual.Data)
	})
}

func getParams() manifests.Params {
	return manifests.Params{
		Config: config.New(config.WithNodeExporterImage("default-exporter")),
		NodeExp: v1alpha1.NodeExporter{
			TypeMeta: metav1.TypeMeta{
				Kind:       "NodeExporter",
				APIVersion: "cloudwatch.aws.amazon.com/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "default",
				UID:       uuid.NewUUID(),
			},
			Spec: v1alpha1.NodeExporterSpec{
				Image: "quay.io/prometheus/node-exporter:0.1.0",
			},
		},
		Log:      logf.Log.WithName("unit-tests"),
		Recorder: record.NewFakeRecorder(10),
	}
}

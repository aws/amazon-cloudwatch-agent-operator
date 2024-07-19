// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dcgmexporter

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

func TestDesiredConfigMap(t *testing.T) {
	expectedLables := map[string]string{
		"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
		"app.kubernetes.io/instance":   "default.test",
		"app.kubernetes.io/part-of":    "amazon-cloudwatch-agent",
		"app.kubernetes.io/version":    "0.1.0",
	}

	t.Run("should return expected dcgm exporter config map", func(t *testing.T) {
		expectedLables["app.kubernetes.io/component"] = "dcgm-exporter"
		expectedLables["app.kubernetes.io/name"] = "dcgm-exporter-config-map"
		expectedLables["app.kubernetes.io/version"] = "0.1.0"

		expectedData := map[string]string{
			"dcp-metrics-included.csv": `DCGM_FI_DEV_GPU_UTIL,      gauge, GPU utilization (in %).`,
		}

		param := getParams()
		actual, err := ConfigMap(param)

		assert.NoError(t, err)
		assert.Equal(t, "dcgm-exporter-config-map", actual.Name)
		assert.Equal(t, expectedLables, actual.Labels)
		assert.Equal(t, expectedData, actual.Data)

	})
}

func TestDesiredConfigMapWithTls(t *testing.T) {
	expectedLables := map[string]string{
		"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
		"app.kubernetes.io/instance":   "default.test",
		"app.kubernetes.io/part-of":    "amazon-cloudwatch-agent",
		"app.kubernetes.io/version":    "0.1.0",
	}

	t.Run("should return expected dcgm exporter config map", func(t *testing.T) {
		expectedLables["app.kubernetes.io/component"] = "dcgm-exporter"
		expectedLables["app.kubernetes.io/name"] = "dcgm-exporter-config-map"
		expectedLables["app.kubernetes.io/version"] = "0.1.0"

		expectedData := map[string]string{
			"dcp-metrics-included.csv": `DCGM_FI_DEV_GPU_UTIL,      gauge, GPU utilization (in %).`,
			"web-config.yaml":          `tls_server_config:  cert_file: /etc/amazon-cloudwatch-observability-dcgm-cert/server.crt`,
		}

		param := getParams()
		param.DcgmExp.Spec.TlsConfig = `tls_server_config:  cert_file: /etc/amazon-cloudwatch-observability-dcgm-cert/server.crt`
		actual, err := ConfigMap(param)

		assert.NoError(t, err)
		assert.Equal(t, "dcgm-exporter-config-map", actual.Name)
		assert.Equal(t, expectedLables, actual.Labels)
		assert.Equal(t, expectedData, actual.Data)

	})
}

func getParams() manifests.Params {
	return manifests.Params{
		Config: config.New(config.WithDcgmExporterImage("default-exporter")),
		DcgmExp: v1alpha1.DcgmExporter{
			TypeMeta: metav1.TypeMeta{
				Kind:       "cloudwatch.aws.amazon.com",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "default",
				UID:       uuid.NewUUID(),
			},
			Spec: v1alpha1.DcgmExporterSpec{
				Image:         "public.ecr.aws/cloudwatch-agent/dcgm-exporter:0.1.0",
				MetricsConfig: "DCGM_FI_DEV_GPU_UTIL,      gauge, GPU utilization (in %).",
			},
		},
		Log:      logf.Log.WithName("unit-tests"),
		Recorder: record.NewFakeRecorder(10),
	}
}

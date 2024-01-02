// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package upgrade_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/version"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/collector/upgrade"
)

func TestInfluxdbReceiverPropertyDrop(t *testing.T) {
	// prepare
	nsn := types.NamespacedName{Name: "my-instance", Namespace: "default"}
	existing := v1alpha1.OpenTelemetryCollector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsn.Name,
			Namespace: nsn.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "opentelemetry-operator",
			},
		},
		Spec: v1alpha1.OpenTelemetryCollectorSpec{
			Config: `
receivers:
  influxdb:
    endpoint: 0.0.0.0:8080
    metrics_schema: telegraf-prometheus-v1

exporters:
  prometheusremotewrite:
    endpoint: "http:hello:4555/hii"

service:
  pipelines:
    metrics:
      receivers: [influxdb]
      exporters: [prometheusremotewrite]
`,
		},
	}
	existing.Status.Version = "0.30.0"

	// test
	up := &upgrade.VersionUpgrade{
		Log:      logger,
		Version:  version.Get(),
		Client:   nil,
		Recorder: record.NewFakeRecorder(upgrade.RecordBufferSize),
	}
	res, err := up.ManagedInstance(context.Background(), existing)
	assert.NoError(t, err)

	// verify
	assert.Equal(t, `exporters:
  prometheusremotewrite:
    endpoint: http:hello:4555/hii
receivers:
  influxdb:
    endpoint: 0.0.0.0:8080
service:
  pipelines:
    metrics:
      exporters:
      - prometheusremotewrite
      receivers:
      - influxdb
`, res.Spec.Config)
}

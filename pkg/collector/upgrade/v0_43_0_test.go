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

func Test0_43_0Upgrade(t *testing.T) {
	// prepare
	nsn := types.NamespacedName{Name: "my-instance", Namespace: "default"}
	existing := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsn.Name,
			Namespace: nsn.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
			},
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			Args: map[string]string{
				"--metrics-addr":   ":8988",
				"--metrics-level":  "detailed",
				"--test-upgrade43": "true",
				"--test-arg1":      "otel",
			},
			Config: `
receivers:
  otlp/mtls:
    protocols:
      http:
        endpoint: mysite.local:55690

exporters:
  otlp:
    endpoint: "example.com"

service:
  pipelines:
    traces: 
      receivers: [otlp/mtls]
      exporters: [otlp]
`,
		},
	}
	existing.Status.Version = "0.42.0"

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
	assert.Equal(t, map[string]string{
		"--test-upgrade43": "true",
		"--test-arg1":      "otel",
	}, res.Spec.Args)

	// verify
	assert.Equal(t, `exporters:
  otlp:
    endpoint: example.com
receivers:
  otlp/mtls:
    protocols:
      http:
        endpoint: mysite.local:55690
service:
  pipelines:
    traces:
      exporters:
      - otlp
      receivers:
      - otlp/mtls
  telemetry:
    metrics:
      address: :8988
      level: detailed
`, res.Spec.Config)

	configWithMetrics := `exporters:
  otlp:
    endpoint: example.com
receivers:
  otlp/mtls:
    protocols:
      http:
        endpoint: mysite.local:55690
service:
  pipelines:
    traces:
      exporters:
      - otlp
      receivers:
      - otlp/mtls
  telemetry:
    metrics:
      address: :8988
      level: detailed
`
	existing.Spec.Config = configWithMetrics
	existing.Spec.Args = map[string]string{
		"--metrics-addr":   ":8988",
		"--metrics-level":  "detailed",
		"--test-upgrade43": "true",
		"--test-arg1":      "otel",
	}
	res, err = up.ManagedInstance(context.Background(), existing)
	assert.NoError(t, err)

	// verify
	assert.Equal(t, configWithMetrics, res.Spec.Config)
	assert.Equal(t, map[string]string{
		"--test-upgrade43": "true",
		"--test-arg1":      "otel",
	}, res.Spec.Args)

}

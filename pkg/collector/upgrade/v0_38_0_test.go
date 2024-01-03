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

func Test0_38_0Upgrade(t *testing.T) {
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
				"--hii":         "hello",
				"--log-profile": "",
				"--log-format":  "hii",
				"--log-level":   "debug",
				"--arg1":        "",
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
	existing.Status.Version = "0.37.0"

	// TESTCASE 1: verify logging args exist and no config logging parameters
	// EXPECTED: drop logging args and configure logging parameters into config from args
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
		"--hii":  "hello",
		"--arg1": "",
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
    logs:
      development: true
      encoding: hii
      level: debug
`, res.Spec.Config)

	// TESTCASE 2: verify logging args exist and also config logging parameters exist
	// EXPECTED: drop logging args and persist logging parameters as configured in config
	configWithLogging := `exporters:
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
    logs:
      development: true
      encoding: hii
      level: debug
`
	existing.Spec.Config = configWithLogging
	existing.Spec.Args = map[string]string{
		"--hii":         "hello",
		"--log-profile": "",
		"--log-format":  "hii",
		"--log-level":   "debug",
		"--arg1":        "",
	}

	res, err = up.ManagedInstance(context.Background(), existing)
	assert.NoError(t, err)

	// verify
	assert.Equal(t, configWithLogging, res.Spec.Config)
	assert.Equal(t, map[string]string{
		"--hii":  "hello",
		"--arg1": "",
	}, res.Spec.Args)
}

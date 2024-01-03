// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package upgrade_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/version"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/collector/upgrade"
)

func Test0_57_0Upgrade(t *testing.T) {
	collectorInstance := v1alpha1.AmazonCloudWatchAgent{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AmazonCloudWatchAgent",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "otel-my-instance",
			Namespace: "somewhere",
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			Config: `receivers:
  otlp:
    protocols:
      http:
        endpoint: mysite.local:55690
extensions:
  health_check:
    endpoint: "localhost"
    port: "4444"
    check_collector_pipeline:
      enabled: false
      exporter_failure_threshold: 5
      interval: 5m
service:
  extensions: [health_check]
  pipelines:
    metrics:
      receivers: [otlp]
      exporters: [nop]
`,
		},
	}

	collectorInstance.Status.Version = "0.56.0"
	//Test to remove port and change endpoint value.
	versionUpgrade := &upgrade.VersionUpgrade{
		Log:      logger,
		Version:  version.Get(),
		Client:   k8sClient,
		Recorder: record.NewFakeRecorder(upgrade.RecordBufferSize),
	}

	upgradedInstance, err := versionUpgrade.ManagedInstance(context.Background(), collectorInstance)
	assert.NoError(t, err)
	assert.Equal(t, `extensions:
  health_check:
    check_collector_pipeline:
      enabled: false
      exporter_failure_threshold: 5
      interval: 5m
    endpoint: localhost:4444
receivers:
  otlp:
    protocols:
      http:
        endpoint: mysite.local:55690
service:
  extensions:
  - health_check
  pipelines:
    metrics:
      exporters:
      - nop
      receivers:
      - otlp
`, upgradedInstance.Spec.Config)
}

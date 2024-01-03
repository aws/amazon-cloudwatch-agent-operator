// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package upgrade_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/version"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/collector/upgrade"
)

func TestRemoveConnectionDelay(t *testing.T) {
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
			Config: `exporters:
  opencensus:
    compression: "on"
    reconnection_delay: 15
    num_workers: 123`,
		},
	}
	existing.Status.Version = "0.8.0"

	// sanity check
	require.Contains(t, existing.Spec.Config, "reconnection_delay")

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
	assert.Contains(t, res.Spec.Config, "opencensus:")
	assert.Contains(t, res.Spec.Config, `compression: "on"`)
	assert.NotContains(t, res.Spec.Config, "reconnection_delay")
	assert.Contains(t, res.Spec.Config, "num_workers: 123")
}

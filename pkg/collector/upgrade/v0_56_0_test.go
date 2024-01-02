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

func Test0_56_0Upgrade(t *testing.T) {
	one := int32(1)
	three := int32(3)

	collectorInstance := v1alpha1.OpenTelemetryCollector{
		TypeMeta: metav1.TypeMeta{
			Kind:       "OpenTelemetryCollector",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "somewhere",
		},
		Spec: v1alpha1.OpenTelemetryCollectorSpec{
			Replicas:    &one,
			MaxReplicas: &three,
		},
	}

	collectorInstance.Status.Version = "0.55.0"
	versionUpgrade := &upgrade.VersionUpgrade{
		Log:      logger,
		Version:  version.Get(),
		Client:   k8sClient,
		Recorder: record.NewFakeRecorder(upgrade.RecordBufferSize),
	}
	upgradedInstance, err := versionUpgrade.ManagedInstance(context.Background(), collectorInstance)
	assert.NoError(t, err)
	assert.Equal(t, one, *upgradedInstance.Spec.MinReplicas)
}

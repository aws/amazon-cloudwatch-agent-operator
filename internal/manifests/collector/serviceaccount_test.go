// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	. "github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector"
)

func TestServiceAccountNewDefault(t *testing.T) {
	// prepare
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance",
		},
	}

	// test
	sa := ServiceAccountName(otelcol)

	// verify
	assert.Equal(t, "my-instance", sa)
}

func TestServiceAccountOverride(t *testing.T) {
	// prepare
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance",
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			AmazonCloudWatchAgentCommonFields: v1alpha1.AmazonCloudWatchAgentCommonFields{
				ServiceAccount: "my-special-sa",
			},
		},
	}

	// test
	sa := ServiceAccountName(otelcol)

	// verify
	assert.Equal(t, "my-special-sa", sa)
}

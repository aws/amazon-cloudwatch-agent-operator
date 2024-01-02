// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"

	"github.com/stretchr/testify/assert"
)

func TestDesiredServiceMonitors(t *testing.T) {
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "my-namespace",
		},
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			Mode:        v1alpha1.ModeStatefulSet,
			Tolerations: testTolerationValues,
		},
	}
	cfg := config.New()

	params := manifests.Params{
		OtelCol: otelcol,
		Config:  cfg,
		Log:     logger,
	}

	actual := ServiceMonitor(params)
	assert.NotNil(t, actual)
	assert.Equal(t, fmt.Sprintf("%s-targetallocator", params.OtelCol.Name), actual.Name)
	assert.Equal(t, params.OtelCol.Namespace, actual.Namespace)
	assert.Equal(t, "targetallocation", actual.Spec.Endpoints[0].Port)

}

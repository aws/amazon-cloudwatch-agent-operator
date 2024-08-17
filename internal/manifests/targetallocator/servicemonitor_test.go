// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
)

func TestDesiredServiceMonitors(t *testing.T) {
	ta := v1alpha1.TargetAllocator{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-instance",
			Namespace: "my-namespace",
		},
		Spec: v1alpha1.TargetAllocatorSpec{
			AmazonCloudWatchAgentCommonFields: v1alpha1.AmazonCloudWatchAgentCommonFields{
				Tolerations: testTolerationValues,
			},
		},
	}
	cfg := config.New()

	params := Params{
		TargetAllocator: ta,
		Config:          cfg,
		Log:             logger,
	}

	actual := ServiceMonitor(params)
	assert.NotNil(t, actual)
	assert.Equal(t, fmt.Sprintf("%s-targetallocator", params.TargetAllocator.Name), actual.Name)
	assert.Equal(t, params.TargetAllocator.Namespace, actual.Namespace)
	assert.Equal(t, "targetallocation", actual.Spec.Endpoints[0].Port)

}

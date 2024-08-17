// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/manifestutils"
)

func TestServiceAccountDefaultName(t *testing.T) {
	// prepare
	targetAllocator := v1alpha1.TargetAllocator{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance",
		},
	}

	// test
	saName := ServiceAccountName(targetAllocator)

	// verify
	assert.Equal(t, "my-instance-targetallocator", saName)
}

func TestServiceAccountOverrideName(t *testing.T) {
	// prepare
	targetAllocator := v1alpha1.TargetAllocator{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance",
		},
		Spec: v1alpha1.TargetAllocatorSpec{
			AmazonCloudWatchAgentCommonFields: v1alpha1.AmazonCloudWatchAgentCommonFields{
				ServiceAccount: "my-special-sa",
			},
		},
	}

	// test
	sa := ServiceAccountName(targetAllocator)

	// verify
	assert.Equal(t, "my-special-sa", sa)
}

func TestServiceAccountDefault(t *testing.T) {
	params := Params{
		TargetAllocator: v1alpha1.TargetAllocator{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-instance",
			},
		},
	}
	expected := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "my-instance-targetallocator",
			Namespace:   params.Collector.Namespace,
			Labels:      manifestutils.Labels(params.TargetAllocator.ObjectMeta, "my-instance-targetallocator", params.TargetAllocator.Spec.Image, ComponentAmazonCloudWatchAgentTargetAllocator, nil),
			Annotations: params.Collector.Annotations,
		},
	}

	saName := ServiceAccountName(params.TargetAllocator)
	sa := ServiceAccount(params)

	assert.Equal(t, saName, sa.Name)
	assert.Equal(t, expected, sa)
}

func TestServiceAccountOverride(t *testing.T) {
	params := Params{
		TargetAllocator: v1alpha1.TargetAllocator{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-instance",
			},
			Spec: v1alpha1.TargetAllocatorSpec{
				AmazonCloudWatchAgentCommonFields: v1alpha1.AmazonCloudWatchAgentCommonFields{
					ServiceAccount: "my-special-sa",
				},
			},
		},
	}
	sa := ServiceAccount(params)

	assert.Nil(t, sa)
}

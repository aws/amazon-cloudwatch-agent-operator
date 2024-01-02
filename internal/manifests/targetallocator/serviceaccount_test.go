// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
)

func TestServiceAccountDefaultName(t *testing.T) {
	// prepare
	otelcol := v1alpha1.OpenTelemetryCollector{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance",
		},
	}

	// test
	saName := ServiceAccountName(otelcol)

	// verify
	assert.Equal(t, "my-instance-targetallocator", saName)
}

func TestServiceAccountOverrideName(t *testing.T) {
	// prepare
	otelcol := v1alpha1.OpenTelemetryCollector{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-instance",
		},
		Spec: v1alpha1.OpenTelemetryCollectorSpec{
			TargetAllocator: v1alpha1.OpenTelemetryTargetAllocator{
				ServiceAccount: "my-special-sa",
			},
		},
	}

	// test
	sa := ServiceAccountName(otelcol)

	// verify
	assert.Equal(t, "my-special-sa", sa)
}

func TestServiceAccountDefault(t *testing.T) {
	params := manifests.Params{
		OtelCol: v1alpha1.OpenTelemetryCollector{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-instance",
			},
		},
	}
	expected := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "my-instance-targetallocator",
			Namespace:   params.OtelCol.Namespace,
			Labels:      Labels(params.OtelCol, "my-instance-targetallocator"),
			Annotations: params.OtelCol.Annotations,
		},
	}

	saName := ServiceAccountName(params.OtelCol)
	sa := ServiceAccount(params)

	assert.Equal(t, sa.Name, saName)
	assert.Equal(t, expected, sa)
}

func TestServiceAccountOverride(t *testing.T) {
	params := manifests.Params{
		OtelCol: v1alpha1.OpenTelemetryCollector{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-instance",
			},
			Spec: v1alpha1.OpenTelemetryCollectorSpec{
				TargetAllocator: v1alpha1.OpenTelemetryTargetAllocator{
					ServiceAccount: "my-special-sa",
				},
			},
		},
	}
	sa := ServiceAccount(params)

	assert.Nil(t, sa)
}

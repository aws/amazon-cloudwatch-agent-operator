// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	. "github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector"
)

func TestVolumeClaimAllowsUserToAdd(t *testing.T) {
	// prepare
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			Mode: "statefulset",
			StatefulSetCommonFields: v1alpha1.StatefulSetCommonFields{
				VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
					ObjectMeta: metav1.ObjectMeta{
						Name: "added-volume",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{"storage": resource.MustParse("1Gi")},
						},
					},
				}},
			},
		},
	}

	// test
	volumeClaims := VolumeClaimTemplates(otelcol)

	// verify that volume claim replaces
	assert.Len(t, volumeClaims, 1)

	// check that it's the added volume
	assert.Equal(t, "added-volume", volumeClaims[0].Name)

	// check the access mode is correct
	assert.Equal(t, corev1.PersistentVolumeAccessMode("ReadWriteOnce"), volumeClaims[0].Spec.AccessModes[0])

	// check the storage is correct
	assert.Equal(t, resource.MustParse("1Gi"), volumeClaims[0].Spec.Resources.Requests["storage"])
}

func TestVolumeClaimChecksForStatefulset(t *testing.T) {
	// prepare
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			Mode: "daemonset",
			StatefulSetCommonFields: v1alpha1.StatefulSetCommonFields{
				VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
					ObjectMeta: metav1.ObjectMeta{
						Name: "added-volume",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{"storage": resource.MustParse("1Gi")},
						},
					},
				}},
			},
		},
	}

	// test
	volumeClaims := VolumeClaimTemplates(otelcol)

	// verify that volume claim replaces
	assert.Len(t, volumeClaims, 0)
}

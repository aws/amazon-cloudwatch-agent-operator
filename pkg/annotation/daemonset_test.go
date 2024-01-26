// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package annotation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
)

func TestAddAnnotationWhenNoneExist(t *testing.T) {
	// prepare
	ds := appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testing-ds",
			Namespace: "default",
		},
		Spec: appsv1.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test-container",
						},
					},
				},
			},
		},
	}

	cfg := config.New(config.WithAnnotationConfig("testing-ds"))

	// test
	changed, err := add(cfg, ds)

	// verify
	assert.NoError(t, err)
	require.Len(t, changed.Annotations, 1)
	require.Len(t, changed.Spec.Template.Annotations, 1)
}

func TestRemoveSidecar(t *testing.T) {
	// prepare
	ds := appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "testing-ds",
			Namespace:   "default",
			Annotations: map[string]string{},
		},

		Spec: appsv1.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test-container",
						},
					},
				},
			},
		},
	}

	cfg := config.New(config.WithAnnotationConfig(""))

	// test
	changed, err := remove(cfg, ds)

	// verify
	assert.NoError(t, err)
	require.Len(t, changed.Annotations, 0)
	require.Len(t, changed.Spec.Template.Annotations, 0)
}

func TestExistsIn(t *testing.T) {
	for _, tt := range []struct {
		desc     string
		ds       appsv1.DaemonSet
		expected bool
	}{
		{"no-annotation",
			appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "testing-ds",
					Namespace:   "default",
					Annotations: map[string]string{},
				},
			},
			false},

		{"has-annotation",
			appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "testing-ds",
					Namespace:   "default",
					Annotations: map[string]string{autoAnnotation: "true"},
				},
			},
			true},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			assert.Equal(t, tt.expected, existsIn(tt.ds))
		})
	}
}

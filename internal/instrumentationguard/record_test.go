// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentationguard

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
)

// exampleRecordJSON is a worked example of the annotation a customer writes to silence the guard
// for a workload: python was turned off, java had no annotation at all. Nothing in the operator
// writes this - the guard only reads whether the annotation is there - so this is the documented
// shape, not an output of the code.
const exampleRecordJSON = `{
  "backedOutAt": "2026-09-24T18:00:00Z",
  "reason": "pod default/orders-7d9f-abc: container app restarted 3 times (CrashLoopBackOff)",
  "failedImages": { "python": "public.ecr.aws/aws-observability/adot-autoinstrumentation-python:v0.0.1" },
  "previous": { "instrumentation.opentelemetry.io/inject-python": "true",
                "instrumentation.opentelemetry.io/inject-java":   null }
}`

func TestReadRecord(t *testing.T) {
	t.Run("absent annotation", func(t *testing.T) {
		rec, ok, err := ReadRecord(&appsv1.Deployment{})
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Nil(t, rec)
	})

	t.Run("unparseable annotation", func(t *testing.T) {
		deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{RecordAnnotationKey: "{not json"},
		}}
		_, ok, err := ReadRecord(deployment)
		assert.True(t, ok, "the annotation is present, it is just broken")
		assert.Error(t, err)
	})

	t.Run("worked example", func(t *testing.T) {
		deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{RecordAnnotationKey: exampleRecordJSON},
		}}
		assert.True(t, HasRecord(deployment))

		rec, ok, err := ReadRecord(deployment)
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, time.Date(2026, time.September, 24, 18, 0, 0, 0, time.UTC), rec.BackedOutAt.UTC())
		assert.Equal(t, "pod default/orders-7d9f-abc: container app restarted 3 times (CrashLoopBackOff)", rec.Reason)
		assert.Equal(t, map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}, rec.FailedImages)

		require.Len(t, rec.Previous, 2)
		require.NotNil(t, rec.Previous[instrumentation.InjectAnnotationKey(instrumentation.TypePython)])
		assert.Equal(t, "true", *rec.Previous[instrumentation.InjectAnnotationKey(instrumentation.TypePython)])
		javaPrevious, present := rec.Previous[instrumentation.InjectAnnotationKey(instrumentation.TypeJava)]
		assert.True(t, present, "the java key must be recorded even though it was absent")
		assert.Nil(t, javaPrevious, "null means the annotation was absent")
	})
}

func TestRecordRoundTrip(t *testing.T) {
	deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Annotations: map[string]string{RecordAnnotationKey: exampleRecordJSON},
	}}
	original, _, err := ReadRecord(deployment)
	require.NoError(t, err)

	encoded, err := original.Marshal()
	require.NoError(t, err)
	deployment.Annotations[RecordAnnotationKey] = encoded

	roundTripped, _, err := ReadRecord(deployment)
	require.NoError(t, err)
	assert.Equal(t, original, roundTripped)

	// The record is a marker, not a state machine: the guard never writes it, so there is no
	// state for it to carry.
	assert.NotContains(t, encoded, "state")
}

// TestInjectionDisabledOnTemplate pins what the record annotation silences the guard FOR: a pod
// template that still turns injection off for every language in the pod. The record alone is not
// that, because an apply or a GitOps reconcile puts the inject annotations back without touching
// the record.
func TestInjectionDisabledOnTemplate(t *testing.T) {
	javaKey := instrumentation.InjectAnnotationKey(instrumentation.TypeJava)
	pythonKey := instrumentation.InjectAnnotationKey(instrumentation.TypePython)

	for _, tt := range []struct {
		name                string
		templateAnnotations map[string]string
		images              map[instrumentation.Type]string
		want                bool
	}{
		{
			name:                "every injected language is off",
			templateAnnotations: map[string]string{javaKey: "false", pythonKey: "false"},
			images: map[instrumentation.Type]string{
				instrumentation.TypeJava:   "java-image",
				instrumentation.TypePython: "python-image",
			},
			want: true,
		},
		{
			// Partial is not disabled: python is still being injected, which is what the guard is
			// reporting on.
			name:                "one of two languages is back on",
			templateAnnotations: map[string]string{javaKey: "false", pythonKey: "true"},
			images: map[instrumentation.Type]string{
				instrumentation.TypeJava:   "java-image",
				instrumentation.TypePython: "python-image",
			},
		},
		{
			name:                "the only language is back on",
			templateAnnotations: map[string]string{pythonKey: "true"},
			images:              map[instrumentation.Type]string{instrumentation.TypePython: "python-image"},
		},
		{
			// Injection is coming from the namespace or from auto-monitor, so an absent
			// annotation is not the customer saying no.
			name:   "no inject annotation on the template",
			images: map[instrumentation.Type]string{instrumentation.TypePython: "python-image"},
		},
		{
			name:                "a language the pod does not carry is ignored",
			templateAnnotations: map[string]string{javaKey: "true", pythonKey: "false"},
			images:              map[instrumentation.Type]string{instrumentation.TypePython: "python-image"},
			want:                true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, workload := range []client.Object{
				&appsv1.Deployment{Spec: appsv1.DeploymentSpec{Template: templateWithAnnotations(tt.templateAnnotations)}},
				&appsv1.StatefulSet{Spec: appsv1.StatefulSetSpec{Template: templateWithAnnotations(tt.templateAnnotations)}},
				&appsv1.DaemonSet{Spec: appsv1.DaemonSetSpec{Template: templateWithAnnotations(tt.templateAnnotations)}},
			} {
				assert.Equal(t, tt.want, injectionDisabledOnTemplate(workload, tt.images), "%T", workload)
			}
		})
	}

	// A kind with no pod template the guard knows about cannot show injection as disabled, so it
	// is never silenced by the record.
	assert.False(t, injectionDisabledOnTemplate(&appsv1.ReplicaSet{},
		map[instrumentation.Type]string{instrumentation.TypePython: "python-image"}))
}

func templateWithAnnotations(annotations map[string]string) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Annotations: annotations}}
}

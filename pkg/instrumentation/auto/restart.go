// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
)

const (
	restartedAtAnnotation = "cloudwatch.aws.amazon.com/restartedAt"
)

var (
	restartAnnotationMutator = instrumentation.NewAnnotationMutator([]instrumentation.AnnotationMutation{&restartAnnotationMutation{}})
)

type restartAnnotationMutation struct {
}

var _ instrumentation.AnnotationMutation = (*restartAnnotationMutation)(nil)

func (m *restartAnnotationMutation) Mutate(annotations map[string]string) map[string]string {
	restartedAt := time.Now().Format(time.RFC3339)
	annotations[restartedAtAnnotation] = restartedAt
	return map[string]string{restartedAtAnnotation: restartedAt}
}

// restart mutates the object's restartedAtAnnotation with the current time.
func setRestartAnnotation(obj client.Object, _ any) (any, bool) {
	switch o := obj.(type) {
	case *appsv1.Deployment:
		if o.Spec.Paused {
			return nil, false
		}
		restartAnnotationMutator.Mutate(o.Spec.Template.GetObjectMeta())
	case *appsv1.DaemonSet:
		restartAnnotationMutator.Mutate(o.Spec.Template.GetObjectMeta())
	case *appsv1.StatefulSet:
		restartAnnotationMutator.Mutate(o.Spec.Template.GetObjectMeta())
	}
	return nil, true
}

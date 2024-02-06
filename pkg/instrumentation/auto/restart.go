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

func (m *restartAnnotationMutation) Mutate(annotations map[string]string) bool {
	annotations[restartedAtAnnotation] = time.Now().Format(time.RFC3339)
	return true
}

// restart mutates the object's restartedAtAnnotation with the current time.
func setRestartAnnotation(obj client.Object) bool {
	switch o := obj.(type) {
	case *appsv1.Deployment:
		restartAnnotationMutator.Mutate(o.Spec.Template.GetObjectMeta())
	case *appsv1.DaemonSet:
		restartAnnotationMutator.Mutate(o.Spec.Template.GetObjectMeta())
	case *appsv1.StatefulSet:
		restartAnnotationMutator.Mutate(o.Spec.Template.GetObjectMeta())
	}
	return true
}

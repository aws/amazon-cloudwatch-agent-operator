// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package annotation

import (
	appsv1 "k8s.io/api/apps/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
)

// add a new annotation to the given ds
func add(cfg config.Config, ds appsv1.DaemonSet) (appsv1.DaemonSet, error) {
	autoAnnotatedLang := getAutoAnnotatedLang(cfg, ds)
	for _, lang := range autoAnnotatedLang {
		if ds.Annotations == nil {
			ds.Annotations = make(map[string]string)
		}
		ds.Annotations[autoAnnotation] = "true"

		// inject auto instrumentation into the ds spec template annotation
		if ds.Spec.Template.Annotations == nil {
			ds.Spec.Template.Annotations = make(map[string]string)
		}
		ds.Spec.Template.Annotations[languageAnnotationMap[lang]] = "true"
	}

	return ds, nil
}

// remove the annotation from the given ds.
func remove(cfg config.Config, ds appsv1.DaemonSet) (appsv1.DaemonSet, error) {
	if !existsIn(ds) {
		return ds, nil
	}
	if ds.Spec.Template.Annotations == nil {
		return ds, nil
	}

	autoAnnotatedLang := getAutoAnnotatedLang(cfg, ds)
	for _, lang := range autoAnnotatedLang {
		delete(ds.Spec.Template.Annotations, languageAnnotationMap[lang])
	}
	delete(ds.Annotations, autoAnnotation)
	return ds, nil
}

// existsIn checks whether annotations exist in the given ds.
func existsIn(ds appsv1.DaemonSet) bool {
	if ds.Spec.Template.Annotations != nil {
		if ds.Spec.Template.Annotations[autoAnnotation] == "true" {
			return true
		}
	}

	return false
}

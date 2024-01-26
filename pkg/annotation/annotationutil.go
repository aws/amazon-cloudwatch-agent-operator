// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package annotation

import (
	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
)

const (
	// annotationInject<Language> indicates whether language auto-instrumentation should be injected or not.
	// Possible values are "true", "false" or "<Instrumentation>" name.
	annotationInjectJava   = "instrumentation.opentelemetry.io/inject-java"
	annotationInjectPython = "instrumentation.opentelemetry.io/inject-python"
	autoAnnotation         = "auto-annotation"
)

var languageAnnotationMap = map[string]string{
	"java":   annotationInjectJava,
	"python": annotationInjectPython,
}

// isAllowListed returns the if the kubernetes workload (deployment, daemon-set, stateful-set) is allow-listed in the operator config
// TODO can this function be made generic for supporting all workloads instead of just daemon-set
func isAllowListed(cfg config.Config, ds appsv1.DaemonSet) bool {
	autoAnnotateLang := getAutoAnnotatedLang(cfg, ds)
	if len(autoAnnotateLang) > 0 {
		return true
	}
	return false
}

// getAutoAnnotatedLang returns the list of languages to be auto-annotated for the given kubernetes workload (deployment, daemon-set, stateful-set)
// TODO can this function be made generic for supporting all workloads instead of just daemon-set
// TODO can this function be made generic for supporting all languages dynamically instead of checking for each one
func getAutoAnnotatedLang(cfg config.Config, ds appsv1.DaemonSet) []string {
	var autoAnnotateLang []string
	annotationConfig := cfg.AnnotationConfig()
	if slices.Contains(annotationConfig.Java.DaemonSets, ds.Name) || slices.Contains(annotationConfig.Java.Namespaces, ds.Namespace) {
		autoAnnotateLang = append(autoAnnotateLang, "java")
	}
	if slices.Contains(annotationConfig.Python.DaemonSets, ds.Name) || slices.Contains(annotationConfig.Python.Namespaces, ds.Namespace) {
		autoAnnotateLang = append(autoAnnotateLang, "python")
	}
	return autoAnnotateLang
}

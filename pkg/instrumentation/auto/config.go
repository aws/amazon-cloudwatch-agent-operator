// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"slices"
)

// AnnotationConfig details the resources that have enabled
// auto-annotation for each instrumentation type.
type AnnotationConfig struct {
	Java   AnnotationResources `json:"java"`
	Python AnnotationResources `json:"python"`
	DotNet AnnotationResources `json:"dotnet"`
	NodeJS AnnotationResources `json:"nodejs"`
}

func (c AnnotationConfig) getResources(instType instrumentation.Type) AnnotationResources {
	switch instType {
	case instrumentation.TypeJava:
		return c.Java
	case instrumentation.TypePython:
		return c.Python
	case instrumentation.TypeDotNet:
		return c.DotNet
	case instrumentation.TypeNodeJS:
		return c.NodeJS
	default:
		return AnnotationResources{}
	}
}

// GetObjectLanguagesToAnnotate get languages to annotate for an object
func (c AnnotationConfig) GetObjectLanguagesToAnnotate(obj client.Object) instrumentation.TypeSet {
	objName := namespacedName(obj)
	typesSelected := instrumentation.TypeSet{}

	types := instrumentation.SupportedTypes()

	switch obj.(type) {
	case *appsv1.Deployment:
		for _, t := range types {
			if slices.Contains(c.getResources(t).Deployments, objName) {
				typesSelected[t] = nil
			}
		}
	case *appsv1.StatefulSet:
		for _, t := range types {
			if slices.Contains(c.getResources(t).StatefulSets, objName) {
				typesSelected[t] = nil
			}
		}
	case *appsv1.DaemonSet:
		for _, t := range types {
			if slices.Contains(c.getResources(t).DaemonSets, objName) {
				typesSelected[t] = nil
			}
		}
	case *corev1.Namespace:
		for _, t := range types {
			if slices.Contains(c.getResources(t).Namespaces, objName) {
				typesSelected[t] = nil
			}
		}
	}

	return typesSelected
}

// AnnotationResources contains slices of resource names for each
// of the supported workloads.
type AnnotationResources struct {
	Namespaces   []string `json:"namespaces,omitempty"`
	Deployments  []string `json:"deployments,omitempty"`
	DaemonSets   []string `json:"daemonsets,omitempty"`
	StatefulSets []string `json:"statefulsets,omitempty"`
}

func getNamespaces(r AnnotationResources) []string {
	return r.Namespaces
}

func getDeployments(r AnnotationResources) []string {
	return r.Deployments
}

func getDaemonSets(r AnnotationResources) []string {
	return r.DaemonSets
}

func getStatefulSets(r AnnotationResources) []string {
	return r.StatefulSets
}

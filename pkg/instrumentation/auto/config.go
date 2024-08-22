// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package auto

import "github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"

// AnnotationConfig details the resources that have enabled
// auto-annotation for each instrumentation type.
type AnnotationConfig struct {
	Java   AnnotationResources `json:"java"`
	Python AnnotationResources `json:"python"`
<<<<<<< HEAD
	NodeJS AnnotationResources `json:"nodejs"`
=======
	DotNet AnnotationResources `json:"dotnet"`
>>>>>>> 099460aea6622b73557017a14b5c46e1b10de680
}

func (c AnnotationConfig) getResources(instType instrumentation.Type) AnnotationResources {
	switch instType {
	case instrumentation.TypeJava:
		return c.Java
	case instrumentation.TypePython:
		return c.Python
<<<<<<< HEAD
	case instrumentation.TypeNodeJS:
		return c.NodeJS
=======
	case instrumentation.TypeDotNet:
		return c.DotNet
>>>>>>> 099460aea6622b73557017a14b5c46e1b10de680
	default:
		return AnnotationResources{}
	}
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

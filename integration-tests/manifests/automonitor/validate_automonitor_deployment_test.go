// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
package annotations

import (
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/auto"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

const sampleDeploymentServiceYaml = "../sample-deployment-service.yaml"

var (
	defaultAnnotationConfig = auto.AnnotationConfig{
		Java:   auto.AnnotationResources{},
		Python: auto.AnnotationResources{},
		DotNet: auto.AnnotationResources{},
		NodeJS: auto.AnnotationResources{},
	}

	none []string
)

func TestServiceThenDeployment(t *testing.T) {
	helper := NewTestHelper(t, true)

	namespace := helper.Initialize("test-namespace", []string{sampleDeploymentServiceYaml})

	// Update operator
	helper.UpdateAnnotationConfig(defaultAnnotationConfig)

	helper.UpdateMonitorConfig(auto.MonitorConfig{
		MonitorAllServices: true,
		Languages:          instrumentation.SupportedTypes(),
		RestartPods:        false,
	})
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml})
	assert.NoError(t, err)

	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", []string{autoAnnotateJavaAnnotation, autoAnnotatePythonAnnotation, autoAnnotateDotNetAnnotation, autoAnnotateNodeJSAnnotation}, none)
	assert.NoError(t, err)
}

// create deployment, create service,  should not annotate anything
func TestDeploymentThenServiceRestartPodsDisabled(t *testing.T) {
	helper := NewTestHelper(t, true)

	namespace := helper.Initialize("test-namespace", []string{})

	// Update operator
	helper.UpdateAnnotationConfig(defaultAnnotationConfig)

	helper.UpdateMonitorConfig(auto.MonitorConfig{
		MonitorAllServices: true,
		Languages:          instrumentation.SupportedTypes(),
		RestartPods:        false,
	})

	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml})

	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", none, []string{autoAnnotateJavaAnnotation, autoAnnotatePythonAnnotation, autoAnnotateDotNetAnnotation, autoAnnotateNodeJSAnnotation})
	assert.NoError(t, err)

	err = helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentServiceYaml})
	assert.NoError(t, err)

	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", none, []string{autoAnnotateJavaAnnotation, autoAnnotatePythonAnnotation, autoAnnotateDotNetAnnotation, autoAnnotateNodeJSAnnotation})
	assert.NoError(t, err)
}

func TestDeploymentThenServiceRestartPodsEnabled(t *testing.T) {
	helper := NewTestHelper(t, true)

	namespace := helper.Initialize("test-namespace", []string{})

	// Update operator
	helper.UpdateAnnotationConfig(defaultAnnotationConfig)

	helper.UpdateMonitorConfig(auto.MonitorConfig{
		MonitorAllServices: true,
		Languages:          instrumentation.SupportedTypes(),
		RestartPods:        true,
	})

	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml})
	assert.NoError(t, err)

	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", none, []string{autoAnnotateJavaAnnotation, autoAnnotatePythonAnnotation, autoAnnotateDotNetAnnotation, autoAnnotateNodeJSAnnotation})
	assert.NoError(t, err)

	helper.startTime = time.Now()
	err = helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentServiceYaml})
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", []string{autoAnnotateJavaAnnotation, autoAnnotatePythonAnnotation, autoAnnotateDotNetAnnotation, autoAnnotateNodeJSAnnotation}, none)
	assert.NoError(t, err)
}

func TestDeploymentWithCustomSelector(t *testing.T) {
	helper := NewTestHelper(t, true)

	namespace := helper.Initialize("test-namespace", []string{})

	// Set up custom selector config
	customSelectorConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Deployments: []string{namespace + "/sample-deployment"},
		},
		Python: auto.AnnotationResources{
			Deployments: []string{namespace + "/sample-deployment"},
		},
	}

	// Update operator with auto monitor disabled and custom selector
	helper.UpdateMonitorConfig(auto.MonitorConfig{
		MonitorAllServices: false,
		Languages:          instrumentation.SupportedTypes(),
		RestartPods:        false,
		CustomSelector:     customSelectorConfig,
	})

	// Create deployment
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml})
	assert.NoError(t, err)

	// Validate annotations are present
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment",
		[]string{autoAnnotateJavaAnnotation, autoAnnotatePythonAnnotation},
		[]string{autoAnnotateDotNetAnnotation, autoAnnotateNodeJSAnnotation})
	assert.NoError(t, err)
}

func TestDeploymentWithCustomSelectorAfterCreation(t *testing.T) {
	helper := NewTestHelper(t, true)

	namespace := helper.Initialize("test-namespace", []string{})

	// Update operator with auto monitor disabled
	helper.UpdateMonitorConfig(auto.MonitorConfig{
		MonitorAllServices: false,
		Languages:          instrumentation.SupportedTypes(),
		RestartPods:        false,
	})

	// Create deployment
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml})
	assert.NoError(t, err)

	// Validate no annotations present
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment",
		none,
		[]string{autoAnnotateJavaAnnotation, autoAnnotatePythonAnnotation, autoAnnotateDotNetAnnotation, autoAnnotateNodeJSAnnotation})
	assert.NoError(t, err)

	// Update operator with custom selector
	namespacedDeployment := namespace + "/sample-deployment"
	customSelectorConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Deployments: []string{namespacedDeployment},
		},
		Python: auto.AnnotationResources{
			Deployments: []string{namespacedDeployment},
		},
		DotNet: auto.AnnotationResources{
			Deployments: []string{namespacedDeployment},
		},
		NodeJS: auto.AnnotationResources{
			Deployments: []string{namespacedDeployment},
		},
	}

	helper.UpdateMonitorConfig(auto.MonitorConfig{
		MonitorAllServices: false,
		Languages:          instrumentation.SupportedTypes(),
		RestartPods:        false,
		CustomSelector:     customSelectorConfig,
	})

	// Validate annotations are not present
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", none,
		[]string{autoAnnotateJavaAnnotation, autoAnnotatePythonAnnotation, autoAnnotateDotNetAnnotation, autoAnnotateNodeJSAnnotation})
	assert.NoError(t, err)

	err = helper.RestartDeployment(namespace, "sample-deployment")
	assert.NoError(t, err)

	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", none,
		[]string{autoAnnotateJavaAnnotation, autoAnnotatePythonAnnotation, autoAnnotateDotNetAnnotation, autoAnnotateNodeJSAnnotation})
	assert.NoError(t, err)
}

func TestDeploymentWithExcludedThenIncludedService(t *testing.T) {
	helper := NewTestHelper(t, true)

	namespace := helper.Initialize("test-namespace", []string{})

	// Set up config with exclusion
	resources := auto.AnnotationResources{
		Deployments: []string{namespace + "/sample-deployment"},
	}
	monitorConfig := auto.MonitorConfig{
		MonitorAllServices: true,
		Languages:          instrumentation.SupportedTypes(),
		Exclude: auto.AnnotationConfig{
			Java:   resources,
			Python: resources,
			DotNet: resources,
			NodeJS: resources,
		},
	}

	// Update operator config
	helper.UpdateAnnotationConfig(defaultAnnotationConfig)
	helper.UpdateMonitorConfig(monitorConfig)

	// Create service first
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentServiceYaml})
	assert.NoError(t, err)

	// Create deployment
	err = helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml})
	assert.NoError(t, err)

	// Validate that deployment has no annotations
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment",
		none,
		[]string{autoAnnotateJavaAnnotation, autoAnnotatePythonAnnotation, autoAnnotateDotNetAnnotation, autoAnnotateNodeJSAnnotation})
	assert.NoError(t, err)

	// Update config to remove exclusion
	monitorConfig.Exclude = auto.AnnotationConfig{}
	helper.UpdateMonitorConfig(monitorConfig)
	err = helper.RestartDeployment(namespace, "sample-deployment")
	if err != nil {
		return
	}
	// Validate that deployment now has annotations
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment",
		[]string{autoAnnotateJavaAnnotation, autoAnnotatePythonAnnotation, autoAnnotateDotNetAnnotation, autoAnnotateNodeJSAnnotation},
		none)
	assert.NoError(t, err)
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
package annotations

import (
	"fmt"
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
		Languages:          instrumentation.NewTypeSet(instrumentation.SupportedTypes()...),
		AutoRestart:        false,
	})
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYamlNameRelPath})
	assert.NoError(t, err)

	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", []string{autoAnnotateJavaAnnotation, autoAnnotatePythonAnnotation, autoAnnotateDotNetAnnotation, autoAnnotateNodeJSAnnotation}, none)
	assert.NoError(t, err)
}

// create deployment, create service,  should not annotate anything
func TestDeploymentThenServiceAutoRestartDisabled(t *testing.T) {
	helper := NewTestHelper(t, true)

	namespace := helper.Initialize("test-namespace", []string{})

	// Update operator
	helper.UpdateAnnotationConfig(defaultAnnotationConfig)

	helper.UpdateMonitorConfig(auto.MonitorConfig{
		MonitorAllServices: true,
		Languages:          instrumentation.NewTypeSet(instrumentation.SupportedTypes()...),
		AutoRestart:        false,
	})

	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYamlNameRelPath})

	// Check annotations
	fmt.Println("Checking if sample-deployment exists")

	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", none, []string{autoAnnotateJavaAnnotation, autoAnnotatePythonAnnotation, autoAnnotateDotNetAnnotation, autoAnnotateNodeJSAnnotation})
	assert.NoError(t, err)

	err = helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentServiceYaml})
	assert.NoError(t, err)

	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", none, []string{autoAnnotateJavaAnnotation, autoAnnotatePythonAnnotation, autoAnnotateDotNetAnnotation, autoAnnotateNodeJSAnnotation})
	assert.NoError(t, err)
}

func TestDeploymentThenServiceAutoRestartEnabled(t *testing.T) {
	helper := NewTestHelper(t, true)

	namespace := helper.Initialize("test-namespace", []string{})

	// Update operator
	helper.UpdateAnnotationConfig(defaultAnnotationConfig)

	helper.UpdateMonitorConfig(auto.MonitorConfig{
		MonitorAllServices: true,
		Languages:          instrumentation.NewTypeSet(instrumentation.SupportedTypes()...),
		AutoRestart:        true,
	})

	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYamlNameRelPath})
	assert.NoError(t, err)

	// Check annotations
	fmt.Println("Checking if sample-deployment exists")
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", none, []string{autoAnnotateJavaAnnotation, autoAnnotatePythonAnnotation, autoAnnotateDotNetAnnotation, autoAnnotateNodeJSAnnotation})
	assert.NoError(t, err)

	helper.startTime = time.Now()
	err = helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentServiceYaml})
	assert.NoError(t, err)

	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", []string{autoAnnotateJavaAnnotation, autoAnnotatePythonAnnotation, autoAnnotateDotNetAnnotation, autoAnnotateNodeJSAnnotation}, none)
	assert.NoError(t, err)
}

func TestDeploymentWithCustomSelector(t *testing.T) {
	helper := NewTestHelper(t, true)

	namespace := helper.Initialize("test-namespace", []string{})

	// Set up custom selector config
	customSelectorConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Deployments: []string{"sample-deployment"},
		},
		Python: auto.AnnotationResources{
			Deployments: []string{"sample-deployment"},
		},
	}

	// Update operator with auto monitor disabled and custom selector
	helper.UpdateMonitorConfig(auto.MonitorConfig{
		MonitorAllServices: false,
		Languages:          instrumentation.NewTypeSet(instrumentation.SupportedTypes()...),
		AutoRestart:        false,
		CustomSelector:     customSelectorConfig,
	})

	// Create deployment
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYamlNameRelPath})
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
		Languages:          instrumentation.NewTypeSet(instrumentation.SupportedTypes()...),
		AutoRestart:        false,
	})

	// Create deployment
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYamlNameRelPath})
	assert.NoError(t, err)

	// Validate no annotations present
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment",
		none,
		[]string{autoAnnotateJavaAnnotation, autoAnnotatePythonAnnotation, autoAnnotateDotNetAnnotation, autoAnnotateNodeJSAnnotation})
	assert.NoError(t, err)

	// Update operator with custom selector
	customSelectorConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Deployments: []string{"sample-deployment"},
		},
		Python: auto.AnnotationResources{
			Deployments: []string{"sample-deployment"},
		},
		DotNet: auto.AnnotationResources{
			Deployments: []string{"sample-deployment"},
		},
		NodeJS: auto.AnnotationResources{
			Deployments: []string{"sample-deployment"},
		},
	}

	helper.UpdateMonitorConfig(auto.MonitorConfig{
		MonitorAllServices: false,
		Languages:          instrumentation.NewTypeSet(instrumentation.SupportedTypes()...),
		AutoRestart:        false,
		CustomSelector:     customSelectorConfig,
	})

	// Validate annotations are present
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment",
		[]string{autoAnnotateJavaAnnotation, autoAnnotatePythonAnnotation, autoAnnotateDotNetAnnotation, autoAnnotateNodeJSAnnotation},
		none)
	assert.NoError(t, err)
}

func TestDeploymentWithExcludedThenIncludedService(t *testing.T) {
	helper := NewTestHelper(t, true)

	namespace := helper.Initialize("test-namespace", []string{})

	// Set up config with service exclusion
	monitorConfig := auto.MonitorConfig{
		MonitorAllServices: true,
		Languages:          instrumentation.NewTypeSet(instrumentation.SupportedTypes()...),
		Exclude: struct {
			Namespaces []string `json:"namespaces"`
			Services   []string `json:"services"`
		}{
			Services: []string{namespace + "/sample-deployment-service"}, // assuming this is the service name in sampleDeploymentServiceYaml
		},
	}

	// Update operator config
	helper.UpdateAnnotationConfig(defaultAnnotationConfig)
	helper.UpdateMonitorConfig(monitorConfig)

	// Create service first
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentServiceYaml})
	assert.NoError(t, err)

	// Create deployment
	err = helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYamlNameRelPath})
	assert.NoError(t, err)

	fmt.Println("Sleeping!")
	time.Sleep(1 * time.Minute)
	// Validate that deployment has no annotations
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment",
		none,
		[]string{autoAnnotateJavaAnnotation, autoAnnotatePythonAnnotation, autoAnnotateDotNetAnnotation, autoAnnotateNodeJSAnnotation})
	assert.NoError(t, err)

	// Update config to remove exclusion
	monitorConfig.Exclude.Services = []string{}
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

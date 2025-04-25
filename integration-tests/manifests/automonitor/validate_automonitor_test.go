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

const (
	sampleDeploymentWithoutServiceYaml = "../sample-deployment-without-service.yaml"
	customerServiceYaml                = "../customer-service.yaml"
	frontendAppYaml                    = "../frontend-app.yaml"
	adminDashboardYaml                 = "../admin-dashboard.yaml"
	conflictingDeploymentYaml          = "../conflicting-deployment.yaml"
)

var (
	allLanguages     = []string{autoAnnotateJavaAnnotation, autoAnnotatePythonAnnotation, autoAnnotateDotNetAnnotation, autoAnnotateNodeJSAnnotation}
	javaPythonOnly   = []string{autoAnnotateJavaAnnotation, autoAnnotatePythonAnnotation}
	dotnetNodejsOnly = []string{autoAnnotateDotNetAnnotation, autoAnnotateNodeJSAnnotation}
)

// Permutation 1 [HIGH]: Enable monitoring for all services without auto restarts
func TestPermutation1_MonitorAllServicesNoAutoRestarts(t *testing.T) {
	helper := NewTestHelper(t, false)
	namespace := helper.Initialize("test-namespace", []string{sampleDeploymentYaml})

	helper.UpdateAnnotationConfig(nil)
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		RestartPods:        false,
	})

	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentServiceYaml})
	assert.NoError(t, err)

	// Verify no annotations without restart
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", none, allLanguages)
	assert.NoError(t, err)

	// Manually restart and verify annotations
	err = helper.RestartDeployment(namespace, "sample-deployment")
	assert.NoError(t, err)
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", allLanguages, none)
	assert.NoError(t, err)
}

// Permutation 2 [HIGH]: Disable automatic monitoring for all services
func TestPermutation2_DisableMonitoringNoAutoRestarts(t *testing.T) {
	helper := NewTestHelper(t, false)
	namespace := helper.Initialize("test-namespace", []string{sampleDeploymentServiceYaml})

	// First enable monitoring with auto-restart
	helper.UpdateAnnotationConfig(nil)
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		RestartPods:        true,
	})

	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml})
	assert.NoError(t, err)

	// Verify initial annotations
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", allLanguages, none)
	assert.NoError(t, err)

	// Disable monitoring without auto-restart
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: false,
		RestartPods:        false,
	})

	// Verify annotations still present
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", allLanguages, none)
	assert.NoError(t, err)

	// Manually restart and verify annotations removed
	err = helper.RestartDeployment(namespace, "sample-deployment")
	assert.NoError(t, err)
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", none, allLanguages)
	assert.NoError(t, err)
}

// Permutation 3 [HIGH]: Monitor all services with pod restarts enabled
func TestPermutation3_MonitorAllServicesWithAutoRestarts(t *testing.T) {
	helper := NewTestHelper(t, false)
	namespace := helper.Initialize("test-namespace", []string{sampleDeploymentServiceYaml})

	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml})
	assert.NoError(t, err)

	// Verify no initial annotations
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", none, allLanguages)
	assert.NoError(t, err)

	// Enable monitoring with auto-restart
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		RestartPods:        true,
	})

	// Verify annotations automatically added
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", allLanguages, none)
	assert.NoError(t, err)
}

// Permutation 4 [MED]: Disable monitoring but allow pod restarts
func TestPermutation4_DisableMonitoringWithAutoRestarts(t *testing.T) {
	helper := NewTestHelper(t, false)
	namespace := helper.Initialize("test-namespace", []string{sampleDeploymentServiceYaml})

	// Start with monitoring enabled
	helper.UpdateAnnotationConfig(nil)
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		RestartPods:        true,
	})

	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml})
	assert.NoError(t, err)

	// Verify initial annotations
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", allLanguages, none)
	assert.NoError(t, err)

	// Disable monitoring with auto-restart
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: false,
		RestartPods:        true,
	})

	// Verify annotations automatically removed
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", none, allLanguages)
	assert.NoError(t, err)
}

// Permutation 5 [HIGH]: Monitor only Java and Python services without pod restarts
func TestPermutation5_MonitorSelectedLanguagesNoAutoRestarts(t *testing.T) {
	helper := NewTestHelper(t, false)
	namespace := helper.Initialize("test-namespace", []string{sampleDeploymentServiceYaml, sampleDeploymentYaml})
	// Start with all languages enabled
	helper.UpdateAnnotationConfig(nil)
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		RestartPods:        true,
	})

	time.Sleep(time.Second * 5)

	// Verify all annotations present
	err := helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", allLanguages, none)
	assert.NoError(t, err)

	// Update to Java and Python only without auto-restart
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		Languages:          instrumentation.NewTypeSet(instrumentation.TypeJava, instrumentation.TypePython), RestartPods: false,
	})

	// Verify annotations unchanged without restart
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", allLanguages, none)
	assert.NoError(t, err)

	// Manually restart and verify only Java/Python remain
	err = helper.RestartDeployment(namespace, "sample-deployment")
	assert.NoError(t, err)
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", javaPythonOnly, dotnetNodejsOnly)
	assert.NoError(t, err)
}

// Permutation 6 [MED]: Monitor Java and Python with pod restarts enabled
func TestPermutation6_MonitorSelectedLanguagesWithAutoRestarts(t *testing.T) {
	helper := NewTestHelper(t, false)
	namespace := helper.Initialize("test-namespace", []string{sampleDeploymentServiceYaml})

	// Start with all languages enabled
	helper.UpdateAnnotationConfig(nil)
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		RestartPods:        true,
	})

	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml})
	assert.NoError(t, err)

	// Verify all annotations present
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", allLanguages, none)
	assert.NoError(t, err)

	// Update to Java and Python only with auto-restart
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		Languages:          instrumentation.NewTypeSet(instrumentation.TypeJava, instrumentation.TypePython), RestartPods: true,
	})

	// Verify only Java/Python annotations remain
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", javaPythonOnly, dotnetNodejsOnly)
	assert.NoError(t, err)
}

// Permutation 9 [HIGH]: Monitor all services but exclude specific Java workloads
func TestPermutation9_MonitorWithExclusionsNoAutoRestarts(t *testing.T) {
	helper := NewTestHelper(t, false)

	namespace := helper.Initialize("test-namespace", []string{sampleDeploymentServiceYaml})

	// Set up exclusions
	excludeConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Deployments: []string{namespace + "/customer-service"},
		},
		Python: auto.AnnotationResources{
			Namespaces: []string{namespace},
		},
	}

	// Update config with exclusions
	helper.UpdateAnnotationConfig(nil)
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		RestartPods:        false,
		Exclude:            excludeConfig,
	})

	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml, customerServiceYaml})
	assert.NoError(t, err)

	// Manually restart deployments
	err = helper.RestartDeployment(namespace, "sample-deployment")
	assert.NoError(t, err)
	err = helper.RestartDeployment(namespace, "customer-service")
	assert.NoError(t, err)

	nonPythonAnnotations := []string{autoAnnotateJavaAnnotation, injectJavaAnnotation, autoAnnotateDotNetAnnotation, injectDotNetAnnotation, autoAnnotateNodeJSAnnotation, injectNodeJSAnnotation}
	// Verify regular deployment has all annotations except python
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", nonPythonAnnotations, []string{autoAnnotatePythonAnnotation})
	assert.NoError(t, err)

	// Verify excluded customer-service has no Java annotations
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "customer-service", []string{autoAnnotateDotNetAnnotation, autoAnnotateNodeJSAnnotation}, []string{autoAnnotateJavaAnnotation, autoAnnotatePythonAnnotation})
	assert.NoError(t, err)

	// Verify kube-system deployment has no Java annotations
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", nonPythonAnnotations, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation})
	assert.NoError(t, err)
}

// Permutation 10 [HIGH]: Monitor all services with auto-restarts but exclude specific Java workloads
func TestPermutation10_MonitorWithExclusionsWithAutoRestarts(t *testing.T) {
	helper := NewTestHelper(t, false)

	// Create single namespace
	namespace := helper.Initialize("test-namespace", []string{sampleDeploymentServiceYaml})

	// Create deployments before enabling monitoring
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml, customerServiceYaml})
	assert.NoError(t, err)

	// Set up exclusions and enable monitoring with auto-restart
	excludeConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Deployments: []string{namespace + "/customer-service"},
		},
	}

	helper.UpdateAnnotationConfig(nil)
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		RestartPods:        true,
		Exclude:            excludeConfig,
	})

	// Verify regular deployment has all annotations
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment", allLanguages, none)
	assert.NoError(t, err)

	// Verify excluded customer-service has no Java annotations
	nonJavaAnnotations := []string{autoAnnotatePythonAnnotation, autoAnnotateDotNetAnnotation, autoAnnotateNodeJSAnnotation}
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "customer-service", nonJavaAnnotations, []string{autoAnnotateJavaAnnotation})
	assert.NoError(t, err)
}

// Permutation 18 [HIGH]: Monitor all services with customSelector and specific languages
func TestPermutation18_MonitorWithCustomSelectorAndAutoRestarts(t *testing.T) {
	helper := NewTestHelper(t, false)

	// Create single namespace
	namespace := helper.Initialize("test-namespace", []string{sampleDeploymentServiceYaml})

	// Create deployments
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml, sampleDeploymentWithoutServiceYaml})
	assert.NoError(t, err)

	// Set up custom selector config
	customSelectorConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces: []string{namespace},
		},
		Python: auto.AnnotationResources{
			Deployments: []string{namespace + "/sample-deployment-without-service"},
		},
	}

	// Update config with custom selector
	helper.UpdateAnnotationConfig(nil)
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		Languages:          instrumentation.NewTypeSet(instrumentation.TypeDotNet),
		RestartPods:        true,
		CustomSelector:     customSelectorConfig,
	})

	// Verify service-selected deployment has dotnet annotation
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment",
		[]string{autoAnnotateDotNetAnnotation},
		[]string{autoAnnotateJavaAnnotation, autoAnnotatePythonAnnotation, autoAnnotateNodeJSAnnotation})
	assert.NoError(t, err)

	// Verify non-service deployment has python annotation
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment-without-service",
		[]string{autoAnnotatePythonAnnotation},
		[]string{autoAnnotateJavaAnnotation, autoAnnotateDotNetAnnotation, autoAnnotateNodeJSAnnotation})
	assert.NoError(t, err)
}

// Permutation 19 [HIGH++]: Test that exclude takes precedence over all
func TestPermutation19_ConflictingCustomSelectorExclude(t *testing.T) {
	helper := NewTestHelper(t, false)

	// Create single namespace
	namespace := helper.Initialize("test-namespace", []string{sampleDeploymentServiceYaml})

	// Create deployments in the namespace
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{
		sampleDeploymentYaml,
		customerServiceYaml,
		conflictingDeploymentYaml,
	})
	assert.NoError(t, err)

	// exclude config without namespace-level exclusion, will add later
	excludeConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Deployments: []string{namespace + "/customer-service"},
		},
	}

	customSelectorConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces: []string{namespace},
		},
		Python: auto.AnnotationResources{
			Deployments: []string{namespace + "/conflicting-deployment"},
		},
		DotNet: auto.AnnotationResources{
			Namespaces: []string{namespace},
		},
	}

	// Update operator config
	helper.UpdateAnnotationConfig(nil)
	monitorConfig := &auto.MonitorConfig{
		MonitorAllServices: true,
		Languages:          instrumentation.NewTypeSet(instrumentation.TypeJava, instrumentation.TypePython, instrumentation.TypeNodeJS),
		RestartPods:        true,
		Exclude:            excludeConfig,
		CustomSelector:     customSelectorConfig,
	}

	helper.UpdateMonitorConfig(monitorConfig)

	// Verify conflicting-deployment has Python and NodeJS
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "conflicting-deployment",
		[]string{autoAnnotatePythonAnnotation, autoAnnotateNodeJSAnnotation, autoAnnotateJavaAnnotation},
		[]string{autoAnnotateDotNetAnnotation})
	assert.NoError(t, err)

	// Verify customer-service has Python and NodeJS (Java excluded)
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "customer-service",
		[]string{autoAnnotatePythonAnnotation, autoAnnotateNodeJSAnnotation},
		[]string{autoAnnotateJavaAnnotation, autoAnnotateDotNetAnnotation})
	assert.NoError(t, err)

	// Verify sample-deployment has Java, Python, and NodeJS
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment",
		[]string{autoAnnotateJavaAnnotation, autoAnnotatePythonAnnotation, autoAnnotateNodeJSAnnotation},
		[]string{autoAnnotateDotNetAnnotation})
	assert.NoError(t, err)
}

// Permutation 20 [HIGH]: Disable general monitoring but enable specific instrumentation
func TestPermutation20_SelectiveMonitoringWithCustomSelector(t *testing.T) {
	helper := NewTestHelper(t, false)

	// Create single namespace
	namespace := helper.Initialize("test-namespace", []string{})

	// Create deployments
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{
		frontendAppYaml,
		adminDashboardYaml,
		sampleDeploymentYaml,
		sampleDeploymentWithoutServiceYaml,
	})
	assert.NoError(t, err)

	// Set up custom selector config
	customSelectorConfig := auto.AnnotationConfig{
		Python: auto.AnnotationResources{
			Deployments: []string{namespace + "/sample-deployment-without-service"},
		},
		NodeJS: auto.AnnotationResources{
			Deployments: []string{namespace + "/frontend-app", namespace + "/admin-dashboard"},
		},
	}

	// Update operator config
	helper.UpdateAnnotationConfig(nil)
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: false,
		Languages:          instrumentation.NewTypeSet(instrumentation.TypeJava),
		RestartPods:        true,
		CustomSelector:     customSelectorConfig,
	})

	// Verify frontend-app and admin-dashboard have NodeJS only
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "frontend-app",
		[]string{autoAnnotateNodeJSAnnotation},
		[]string{autoAnnotateJavaAnnotation, autoAnnotatePythonAnnotation, autoAnnotateDotNetAnnotation})
	assert.NoError(t, err)

	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "admin-dashboard",
		[]string{autoAnnotateNodeJSAnnotation},
		[]string{autoAnnotateJavaAnnotation, autoAnnotatePythonAnnotation, autoAnnotateDotNetAnnotation})
	assert.NoError(t, err)

	// Verify sample-deployment has no annotations
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment",
		none,
		allLanguages)
	assert.NoError(t, err)

	// Verify sample-deployment-without-service has Python
	err = helper.ValidateWorkloadAnnotations("deployment", namespace, "sample-deployment-without-service",
		[]string{autoAnnotatePythonAnnotation},
		[]string{autoAnnotateJavaAnnotation, autoAnnotateDotNetAnnotation, autoAnnotateNodeJSAnnotation})
	assert.NoError(t, err)
}

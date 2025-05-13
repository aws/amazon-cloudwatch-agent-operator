// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
package annotations

import (
	"fmt"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/auto"
)

const (
	deploymentName              = "sample-deployment"
	deploymentWithoutService    = "sample-deployment-without-service"
	daemonSetName               = "sample-daemonset"
	customServiceDeploymentName = "customer-service"
	statefulSetName             = "sample-statefulset"

	sampleDaemonsetYaml                = "../sample-daemonset.yaml"
	sampleDeploymentYaml               = "../sample-deployment.yaml"
	sampleDeploymentServiceYaml        = "../sample-deployment-service.yaml"
	sampleDeploymentWithoutServiceYaml = "../sample-deployment-without-service.yaml"
	sampleStatefulsetYaml              = "../sample-statefulset.yaml"
	customerServiceYaml                = "../customer-service.yaml"
	frontendAppYaml                    = "../frontend-app.yaml"
	adminDashboardYaml                 = "../admin-dashboard.yaml"
	conflictingDeploymentYaml          = "../conflicting-deployment.yaml"
)

var all = slices.Collect(maps.Keys(instrumentation.SupportedTypes))
var allAnnotations = getAnnotations(all...)
var none []string

// getAnnotations returns both auto and inject annotations for the specified language types
func getAnnotations(types ...instrumentation.Type) []string {
	var annotations []string
	for _, t := range types {
		switch t {
		case instrumentation.TypeJava:
			annotations = append(annotations, autoAnnotateJavaAnnotation, injectJavaAnnotation)
		case instrumentation.TypePython:
			annotations = append(annotations, autoAnnotatePythonAnnotation, injectPythonAnnotation)
		case instrumentation.TypeDotNet:
			annotations = append(annotations, autoAnnotateDotNetAnnotation, injectDotNetAnnotation)
		case instrumentation.TypeNodeJS:
			annotations = append(annotations, autoAnnotateNodeJSAnnotation, injectNodeJSAnnotation)
		}
	}
	return annotations
}

// disabled by default
func TestDefault(t *testing.T) {
	helper := NewTestHelper(t)
	// copying Initialize() to skip updating operator with auto config
	newUUID := uuid.New()
	namespace := fmt.Sprintf("%s-%s", "test-namespace", newUUID.String())
	helper.startTime = time.Now()

	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml, sampleDeploymentServiceYaml}, false)
	assert.NoError(t, err)
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, none, allAnnotations)
	assert.NoError(t, err)
}

func TestInvalidConfig(t *testing.T) {
	helper := NewTestHelper(t)

	namespace := helper.Initialize("test-namespace")

	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		Languages: instrumentation.NewTypeSet("perl"),
	})
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml, sampleDeploymentServiceYaml}, false)
	assert.NoError(t, err)

	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, none, allAnnotations)
	assert.NoError(t, err)
}

func TestServiceThenDeployment(t *testing.T) {
	helper := NewTestHelper(t)

	namespace := helper.Initialize("test-namespace")

	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		Languages:          instrumentation.SupportedTypes,
		RestartPods:        false,
	})
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml, sampleDeploymentServiceYaml}, false)
	assert.NoError(t, err)

	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, allAnnotations, none)
	assert.NoError(t, err)
}

// create deployment, create service,  should not annotate anything
func TestDeploymentThenServiceRestartPodsDisabled(t *testing.T) {
	helper := NewTestHelper(t)

	namespace := helper.Initialize("test-namespace")

	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		Languages:          instrumentation.SupportedTypes,
		RestartPods:        false,
	})

	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml}, false)
	assert.NoError(t, err)

	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, none, allAnnotations)
	assert.NoError(t, err)

	err = helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentServiceYaml}, false)
	assert.NoError(t, err)

	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, none, allAnnotations)
	assert.NoError(t, err)
}

func TestDeploymentThenServiceRestartPodsEnabled(t *testing.T) {
	helper := NewTestHelper(t)

	namespace := helper.Initialize("test-namespace")

	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		Languages:          instrumentation.SupportedTypes,
		RestartPods:        true,
	})

	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml}, false)
	assert.NoError(t, err)

	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, none, allAnnotations)
	assert.NoError(t, err)

	helper.startTime = time.Now()
	err = helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentServiceYaml}, false)
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, allAnnotations, none)
	assert.NoError(t, err)
}

func TestDeploymentWithCustomSelector(t *testing.T) {
	helper := NewTestHelper(t)

	namespace := helper.Initialize("test-namespace")

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
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: false,
		Languages:          instrumentation.SupportedTypes,
		RestartPods:        false,
		CustomSelector:     customSelectorConfig,
	})

	// Create deployment
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml}, false)
	assert.NoError(t, err)

	// Validate annotations are present
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName,
		[]string{autoAnnotateJavaAnnotation, autoAnnotatePythonAnnotation},
		[]string{autoAnnotateDotNetAnnotation, autoAnnotateNodeJSAnnotation})
	assert.NoError(t, err)
}

func TestDeploymentWithCustomSelectorAfterCreation(t *testing.T) {
	helper := NewTestHelper(t)

	namespace := helper.Initialize("test-namespace")

	// Update operator with auto monitor disabled
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: false,
		Languages:          instrumentation.SupportedTypes,
		RestartPods:        false,
	})

	// Create deployment
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml}, false)
	assert.NoError(t, err)

	// Validate no annotations present
	all := allAnnotations
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName,
		none,
		all)
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

	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: false,
		Languages:          instrumentation.SupportedTypes,
		RestartPods:        false,
		CustomSelector:     customSelectorConfig,
	})

	// Validate annotations are not present (custom selector obeys restart pods)
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, none, all)
	assert.NoError(t, err)

	err = helper.RestartWorkload(Deployment, namespace, deploymentName)
	assert.NoError(t, err)

	// validate annotations are present
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, all, none)
	assert.NoError(t, err)
}

func TestDeploymentWithExcludedThenIncludedService(t *testing.T) {
	helper := NewTestHelper(t)

	namespace := helper.Initialize("test-namespace")

	// Set up config with exclusion
	resources := auto.AnnotationResources{
		Deployments: []string{namespace + "/sample-deployment"},
	}
	monitorConfig := auto.MonitorConfig{
		MonitorAllServices: true,
		Languages:          instrumentation.SupportedTypes,
		Exclude: auto.AnnotationConfig{
			Java:   resources,
			Python: resources,
			DotNet: resources,
			NodeJS: resources,
		},
	}

	// Update operator config
	helper.UpdateMonitorConfig(&monitorConfig)

	// Create service first
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentServiceYaml}, false)
	assert.NoError(t, err)

	// Create deployment
	err = helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml}, false)
	assert.NoError(t, err)

	// Validate that deployment has no annotations
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName,
		none,
		allAnnotations)
	assert.NoError(t, err)

	// Update config to remove exclusion
	monitorConfig.Exclude = auto.AnnotationConfig{}
	helper.UpdateMonitorConfig(&monitorConfig)
	err = helper.RestartWorkload(Deployment, namespace, deploymentName)
	assert.NoError(t, err)
	// Validate that deployment now has annotations
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName,
		allAnnotations,
		none)
	assert.NoError(t, err)
}

// Adding more scenarios/permutations
// Permutation 1 [HIGH]: Enable monitoring for all services without auto restarts
func TestPermutation1_MonitorAllServicesNoAutoRestarts(t *testing.T) {
	helper := NewTestHelper(t)
	namespace := helper.Initialize("test-namespace")

	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		RestartPods:        false,
	})

	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml, sampleDeploymentServiceYaml}, false)
	assert.NoError(t, err)

	// Verify no annotations without restart
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, none, allAnnotations)
	assert.NoError(t, err)

	// Manually restart and verify annotations
	err = helper.RestartWorkload(Deployment, namespace, deploymentName)
	assert.NoError(t, err)
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, allAnnotations, none)
	assert.NoError(t, err)
}

// Permutation 2 [HIGH]: Disable automatic monitoring for all services
func TestPermutation2_DisableMonitoringNoAutoRestarts(t *testing.T) {
	helper := NewTestHelper(t)
	namespace := helper.Initialize("test-namespace")

	// First enable monitoring with auto-restart
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		RestartPods:        true,
	})

	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml, sampleDeploymentServiceYaml}, false)
	assert.NoError(t, err)

	// Verify initial annotations
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, allAnnotations, none)
	assert.NoError(t, err)

	// Disable monitoring without auto-restart
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: false,
		RestartPods:        false,
	})

	// Verify annotations still present
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, allAnnotations, none)
	assert.NoError(t, err)

	// Manually restart and verify annotations removed
	err = helper.RestartWorkload(Deployment, namespace, deploymentName)
	assert.NoError(t, err)
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, none, allAnnotations)
	assert.NoError(t, err)
}

// Permutation 3 [HIGH]: Monitor all services with pod restarts enabled
func TestPermutation3_MonitorAllServicesWithAutoRestarts(t *testing.T) {
	helper := NewTestHelper(t)
	namespace := helper.Initialize("test-namespace")

	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml, sampleDeploymentServiceYaml}, false)
	assert.NoError(t, err)

	// Verify no initial annotations
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, none, allAnnotations)
	assert.NoError(t, err)

	// Enable monitoring with auto-restart
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		RestartPods:        true,
	})

	// Verify annotations automatically added
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, allAnnotations, none)
	assert.NoError(t, err)
}

// Permutation 4 [MED]: Disable monitoring but allow pod restarts
func TestPermutation4_DisableMonitoringWithAutoRestarts(t *testing.T) {
	helper := NewTestHelper(t)
	namespace := helper.Initialize("test-namespace")

	// Start with monitoring enabled
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		RestartPods:        true,
	})

	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml, sampleDeploymentServiceYaml}, false)
	assert.NoError(t, err)

	// Verify initial annotations
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, allAnnotations, none)
	assert.NoError(t, err)

	// Disable monitoring with auto-restart
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: false,
		RestartPods:        true,
	})

	// Verify annotations automatically removed
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, none, allAnnotations)
	assert.NoError(t, err)
}

// Permutation 5 [HIGH]: Monitor only Java and Python services without pod restarts
func TestPermutation5_MonitorSelectedLanguagesNoAutoRestarts(t *testing.T) {
	helper := NewTestHelper(t)
	namespace := helper.Initialize("test-namespace")
	// Start with all languages enabled
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		RestartPods:        true,
	})

	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml, sampleDeploymentServiceYaml}, false)
	assert.NoError(t, err)

	// Verify all annotations present
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, allAnnotations, none)
	assert.NoError(t, err)

	// Update to Java and Python only without auto-restart
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		Languages:          instrumentation.NewTypeSet(instrumentation.TypeJava, instrumentation.TypePython),
		RestartPods:        false,
	})

	// Verify annotations unchanged without restart
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, allAnnotations, none)
	assert.NoError(t, err)

	// Manually restart and verify only Java/Python remain
	err = helper.RestartWorkload(Deployment, namespace, deploymentName)
	assert.NoError(t, err)
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName,
		getAnnotations(instrumentation.TypeJava, instrumentation.TypePython),
		getAnnotations(instrumentation.TypeDotNet, instrumentation.TypeNodeJS))
	assert.NoError(t, err)
}

// Permutation 6 [MED]: Monitor Java and Python with pod restarts enabled
func TestPermutation6_MonitorSelectedLanguagesWithAutoRestarts(t *testing.T) {
	helper := NewTestHelper(t)
	namespace := helper.Initialize("test-namespace")

	// Start with all languages enabled
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		RestartPods:        true,
	})

	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml, sampleDeploymentServiceYaml}, false)
	assert.NoError(t, err)

	// Verify all annotations present
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, allAnnotations, none)
	assert.NoError(t, err)

	// Update to Java and Python only with auto-restart
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		Languages:          instrumentation.NewTypeSet(instrumentation.TypeJava, instrumentation.TypePython),
		RestartPods:        true,
	})

	// Verify only Java/Python annotations remain
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName,
		getAnnotations(instrumentation.TypeJava, instrumentation.TypePython),
		getAnnotations(instrumentation.TypeDotNet, instrumentation.TypeNodeJS))
	assert.NoError(t, err)
}

// Permutation 7 [LOW]: Monitor Java and Python with pod restarts disabled
func TestPermutation7_MonitorSelectedLanguagesWithoutAutoRestarts(t *testing.T) {
	helper := NewTestHelper(t)
	namespace := helper.Initialize("test-namespace")

	// Start with all languages enabled
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		Languages:          instrumentation.NewTypeSet(instrumentation.TypeJava, instrumentation.TypePython),
		RestartPods:        true,
	})

	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml, sampleDeploymentServiceYaml}, false)
	assert.NoError(t, err)

	// Verify all annotations present
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName,
		getAnnotations(instrumentation.TypeJava, instrumentation.TypePython),
		getAnnotations(instrumentation.TypeDotNet, instrumentation.TypeNodeJS))
	assert.NoError(t, err)

	// Update to Java and Python only with auto-restart
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: false,
		Languages:          instrumentation.NewTypeSet(instrumentation.TypeJava, instrumentation.TypePython),
		RestartPods:        false,
	})

	// Verify only Java/Python annotations remain
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName,
		getAnnotations(instrumentation.TypeJava, instrumentation.TypePython),
		getAnnotations(instrumentation.TypeDotNet, instrumentation.TypeNodeJS))
	assert.NoError(t, err)
}

// Permutation 8 [LOW]: Monitor Java and Python with pod restarts enabled which should remove annotations
func TestPermutation8_MonitorSelectedLanguagesWithAutoRestarts(t *testing.T) {
	helper := NewTestHelper(t)
	namespace := helper.Initialize("test-namespace")

	// Start with all languages enabled
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		RestartPods:        true,
	})

	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml, sampleDeploymentServiceYaml}, false)
	assert.NoError(t, err)

	// Verify all annotations present
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, allAnnotations, none)
	assert.NoError(t, err)

	// Update to Java and Python only with auto-restart
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: false,
		Languages:          instrumentation.NewTypeSet(instrumentation.TypeJava, instrumentation.TypePython),
		RestartPods:        true,
	})

	// Verify only Java/Python annotations remain
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, none, allAnnotations)
	assert.NoError(t, err)
}

// Permutation 9 [HIGH]: Monitor all services but exclude specific Java workloads
func TestPermutation9_MonitorWithExclusionsNoAutoRestarts(t *testing.T) {
	helper := NewTestHelper(t)

	namespace := helper.Initialize("test-namespace")

	// Set up exclusions
	excludeConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Deployments: []string{namespace + "/" + customServiceDeploymentName},
		},
		Python: auto.AnnotationResources{
			Namespaces: []string{namespace},
		},
	}

	// Update config with exclusions
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		RestartPods:        false,
		Exclude:            excludeConfig,
	})

	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml, sampleDeploymentServiceYaml, customerServiceYaml}, false)
	assert.NoError(t, err)

	// Manually restart deployments
	err = helper.RestartWorkload(Deployment, namespace, deploymentName)
	assert.NoError(t, err)
	err = helper.RestartWorkload(Deployment, namespace, customServiceDeploymentName)
	assert.NoError(t, err)

	// Verify regular deployment has all annotations except python
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName,
		getAnnotations(instrumentation.TypeJava, instrumentation.TypeDotNet, instrumentation.TypeNodeJS),
		getAnnotations(instrumentation.TypePython))
	assert.NoError(t, err)

	// Verify excluded customer-service has no Java annotations
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, customServiceDeploymentName,
		getAnnotations(instrumentation.TypeDotNet, instrumentation.TypeNodeJS),
		getAnnotations(instrumentation.TypeJava, instrumentation.TypePython))
	assert.NoError(t, err)

	// Verify kube-system deployment has no Python annotations
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName,
		getAnnotations(instrumentation.TypeJava, instrumentation.TypeDotNet, instrumentation.TypeNodeJS),
		getAnnotations(instrumentation.TypePython))
	assert.NoError(t, err)
}

// Permutation 10 [HIGH]: Monitor all services with auto-restarts but exclude specific Java workloads
func TestPermutation10_MonitorWithExclusionsWithAutoRestarts(t *testing.T) {
	helper := NewTestHelper(t)

	// Create single namespace
	namespace := helper.Initialize("test-namespace")

	// Create deployments before enabling monitoring
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml, sampleDeploymentServiceYaml, customerServiceYaml}, false)
	assert.NoError(t, err)

	// Set up exclusions and enable monitoring with auto-restart
	excludeConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Deployments: []string{namespace + "/" + customServiceDeploymentName},
		},
	}

	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		RestartPods:        true,
		Exclude:            excludeConfig,
	})

	// Verify regular deployment has all annotations
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, allAnnotations, none)
	assert.NoError(t, err)

	// Verify excluded customer-service has no Java annotations
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, customServiceDeploymentName,
		getAnnotations(instrumentation.TypePython, instrumentation.TypeDotNet, instrumentation.TypeNodeJS),
		getAnnotations(instrumentation.TypeJava))
	assert.NoError(t, err)
}

// Permutation 11 [LOW]: Disabling monitoring but keep exclusion rules
func TestPermutation11_DisableMonitorWithExclusionsNoAutoRestarts(t *testing.T) {
	helper := NewTestHelper(t)

	namespace := helper.Initialize("test-namespace")

	// Start with all languages enabled
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		RestartPods:        true,
	})

	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml, sampleDeploymentServiceYaml, sampleDaemonsetYaml}, false)
	assert.NoError(t, err)

	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, allAnnotations, none)
	assert.NoError(t, err)

	// Set up exclusions
	excludeConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:  []string{kubeSystemNamespace},
			Deployments: []string{namespace + "/sample-deployment"},
			DaemonSets:  []string{namespace + "/sample-daemonset"},
		},
	}

	// Update config with exclusions
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: false,
		RestartPods:        false,
		Exclude:            excludeConfig,
	})

	// Manually restart deployments
	err = helper.RestartWorkload(Deployment, namespace, deploymentName)
	assert.NoError(t, err)
	// Verify regular deployment has no annotations
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, none, allAnnotations)
	assert.NoError(t, err)

	// Verify regular daemonset still has all annotations since it's not restarted
	err = helper.ValidateWorkloadAnnotations(DaemonSet, namespace, "sample-daemonset", allAnnotations, none)
	assert.NoError(t, err)
	// Manually restart deployments
	err = helper.RestartWorkload(DaemonSet, namespace, "sample-daemonset")
	assert.NoError(t, err)
	// Verify regular daemonset still has all annotations since it's not restarted
	err = helper.ValidateWorkloadAnnotations(DaemonSet, namespace, "sample-daemonset", none, allAnnotations)
	assert.NoError(t, err)
}

// Permutation 12 [LOW]: Disabling monitoring but keep exclusion rules
func TestPermutation12_DisableMonitorWithExclusionsAutoRestarts(t *testing.T) {
	helper := NewTestHelper(t)

	namespace := helper.Initialize("test-namespace")

	// Start with all languages enabled
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		RestartPods:        true,
	})

	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml, sampleDeploymentServiceYaml, customerServiceYaml, sampleDaemonsetYaml}, false)
	assert.NoError(t, err)

	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, allAnnotations, none)
	assert.NoError(t, err)

	// Set up exclusions
	excludeConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:  []string{kubeSystemNamespace},
			Deployments: []string{namespace + "/" + customServiceDeploymentName},
		},
	}

	// Update config with exclusions
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: false,
		RestartPods:        true,
		Exclude:            excludeConfig,
	})

	// Verify regular deployment has no annotations
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, none, allAnnotations)
	assert.NoError(t, err)
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, customServiceDeploymentName, none, allAnnotations)
	assert.NoError(t, err)
}

// Permutation 13 [MED]: Disabling restart but keep exclusion rules and languages
func TestPermutation13_MonitorAndNoAutoRestartsWithLanguagesAndExclusion(t *testing.T) {
	helper := NewTestHelper(t)

	// Create single namespace
	namespace := helper.Initialize("test-namespace")

	// Create deployments
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml, sampleDeploymentServiceYaml, customerServiceYaml}, false)
	assert.NoError(t, err)

	// Start with all languages enabled
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		RestartPods:        true,
	})

	//precheck
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, allAnnotations, none)
	assert.NoError(t, err)
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, customServiceDeploymentName, allAnnotations, none)
	assert.NoError(t, err)

	// Update config with custom selector
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		Languages:          instrumentation.NewTypeSet(instrumentation.TypeJava, instrumentation.TypePython),
		RestartPods:        false,
		Exclude: auto.AnnotationConfig{
			Java: auto.AnnotationResources{
				Namespaces:  []string{kubeSystemNamespace},
				Deployments: []string{namespace + "/" + customServiceDeploymentName},
				DaemonSets:  []string{namespace + "/sample-daemonset"},
			},
		},
	})

	//postcheck with no restart
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, allAnnotations, none)
	assert.NoError(t, err)
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, customServiceDeploymentName, allAnnotations, none)
	assert.NoError(t, err)

	// verify service associated sample-deployment
	err = helper.RestartWorkload(Deployment, namespace, deploymentName)
	assert.NoError(t, err)
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName,
		getAnnotations(instrumentation.TypeJava, instrumentation.TypePython),
		getAnnotations(instrumentation.TypeDotNet, instrumentation.TypeNodeJS))
	assert.NoError(t, err)

	// verify excluded custom-deployment
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, customServiceDeploymentName, allAnnotations, none)
	assert.NoError(t, err)
}

// Permutation 14 [MED]: monitor java and python workloads while keeping specific java workload excluded
func TestPermutation14_MonitorAndAutoRestartsWithLanguagesAndExclusion(t *testing.T) {
	helper := NewTestHelper(t)
	nsAnother := "another"

	// Create single namespace
	namespace := helper.Initialize("test-namespace")
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml, sampleDeploymentServiceYaml, sampleDeploymentWithoutServiceYaml, customerServiceYaml}, false)
	assert.NoError(t, err)
	// ns deployment
	err = helper.CreateNamespaceAndApplyResources(nsAnother, []string{customerServiceYaml}, false)
	assert.NoError(t, err)

	// Update config with custom selector
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		Languages:          instrumentation.NewTypeSet(instrumentation.TypeJava, instrumentation.TypePython),
		RestartPods:        true,
		Exclude: auto.AnnotationConfig{
			Java: auto.AnnotationResources{
				Namespaces:  []string{nsAnother},
				Deployments: []string{namespace + "/" + customServiceDeploymentName},
				DaemonSets:  []string{namespace + "/" + daemonSetName},
			},
		},
	})

	// postcheck
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName,
		getAnnotations(instrumentation.TypeJava, instrumentation.TypePython),
		getAnnotations(instrumentation.TypeDotNet, instrumentation.TypeNodeJS))
	assert.NoError(t, err)
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentWithoutService, none, allAnnotations)
	assert.NoError(t, err)
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, customServiceDeploymentName,
		getAnnotations(instrumentation.TypePython),
		getAnnotations(instrumentation.TypeJava, instrumentation.TypeDotNet, instrumentation.TypeNodeJS))
	assert.NoError(t, err)
	err = helper.ValidateWorkloadAnnotations(Deployment, nsAnother, customServiceDeploymentName,
		getAnnotations(instrumentation.TypePython),
		getAnnotations(instrumentation.TypeJava, instrumentation.TypeDotNet, instrumentation.TypeNodeJS))
	assert.NoError(t, err)
}

// Permutation 15 [MED]: disable monitor while keeping specific java workload excluded
func TestPermutation15_NoMonitorAndNoAutoRestartsWithLanguagesAndExclusion(t *testing.T) {
	helper := NewTestHelper(t)

	// Create single namespace
	namespace := helper.Initialize("test-namespace")
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentServiceYaml, sampleDeploymentYaml, customerServiceYaml}, false)
	assert.NoError(t, err)

	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		RestartPods:        true,
	})

	// precheck
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, allAnnotations, none)
	assert.NoError(t, err)

	// Update config with custom selector
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: false,
		Languages:          instrumentation.NewTypeSet(instrumentation.TypeJava, instrumentation.TypePython),
		RestartPods:        false,
		Exclude: auto.AnnotationConfig{
			Java: auto.AnnotationResources{
				Namespaces:  []string{kubeSystemNamespace},
				Deployments: []string{namespace + "/" + customServiceDeploymentName},
				DaemonSets:  []string{namespace + "/" + daemonSetName},
			},
		},
	})

	// postcheck
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, allAnnotations, none)
	assert.NoError(t, err)
	err = helper.RestartWorkload(Deployment, namespace, deploymentName)
	assert.NoError(t, err)
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, none, allAnnotations)
	assert.NoError(t, err)

	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, customServiceDeploymentName, allAnnotations, none)
	assert.NoError(t, err)
	err = helper.RestartWorkload(Deployment, namespace, customServiceDeploymentName)
	assert.NoError(t, err)
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, customServiceDeploymentName, none, allAnnotations)
	assert.NoError(t, err)
}

// Permutation 16 [LOW]: disable monitor while keeping specific java workload excluded
func TestPermutation16_NoMonitorAndAutoRestartsWithLanguagesAndExclusion(t *testing.T) {
	helper := NewTestHelper(t)

	// Create single namespace
	namespace := helper.Initialize("test-namespace")
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentServiceYaml, sampleDeploymentYaml, customerServiceYaml}, false)
	assert.NoError(t, err)

	// Update config with custom selector
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		Languages:          instrumentation.NewTypeSet(instrumentation.TypeJava, instrumentation.TypePython),
		RestartPods:        true,
	})

	// precheck
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName,
		getAnnotations(instrumentation.TypeJava, instrumentation.TypePython),
		getAnnotations(instrumentation.TypeDotNet, instrumentation.TypeNodeJS))
	assert.NoError(t, err)

	// Update config with custom selector
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: false,
		Languages:          instrumentation.NewTypeSet(instrumentation.TypeJava, instrumentation.TypePython),
		RestartPods:        true,
		Exclude: auto.AnnotationConfig{
			Java: auto.AnnotationResources{
				Namespaces:  []string{kubeSystemNamespace},
				Deployments: []string{namespace + "/" + customServiceDeploymentName},
				DaemonSets:  []string{namespace + "/" + daemonSetName},
			},
		},
	})

	// postcheck
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, none, allAnnotations)
	assert.NoError(t, err)
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, customServiceDeploymentName, none, allAnnotations)
	assert.NoError(t, err)
}

// Permutation 17 [MED]: enable monitor with no restart while adding custom selector on namespace
func TestPermutation17_MonitorAndNoAutoRestartsWithNamespaceCustomSelector(t *testing.T) {
	helper := NewTestHelper(t)

	nsAnother := "another"
	// Create single namespace
	namespace := helper.Initialize("test-namespace")
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{customerServiceYaml}, false)
	assert.NoError(t, err)

	// Update config with custom selector
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		RestartPods:        false,
		CustomSelector: auto.AnnotationConfig{
			Java: auto.AnnotationResources{
				Namespaces: []string{nsAnother},
			},
			Python: auto.AnnotationResources{
				Namespaces: []string{nsAnother},
			},
		},
	})

	// precheck
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, customServiceDeploymentName, none, allAnnotations)
	assert.NoError(t, err)
	err = helper.RestartWorkload(Deployment, namespace, customServiceDeploymentName)
	assert.NoError(t, err)
	//include all after restart
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, customServiceDeploymentName, allAnnotations, none)
	assert.NoError(t, err)

	// postcheck
	// include all
	err = helper.RestartWorkload(Deployment, namespace, customServiceDeploymentName)
	assert.NoError(t, err)
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, customServiceDeploymentName, allAnnotations, none)
	assert.NoError(t, err)
	// include java and python by custom selector
	err = helper.CreateNamespaceAndApplyResources(nsAnother, []string{customerServiceYaml}, false)
	assert.NoError(t, err)
	err = helper.ValidatePodsAnnotations(nsAnother,
		getAnnotations(instrumentation.TypeJava, instrumentation.TypePython),
		getAnnotations(instrumentation.TypeDotNet, instrumentation.TypeNodeJS))
	assert.NoError(t, err)
	// include java and python by custom selector
	err = helper.CreateNamespaceAndApplyResources(nsAnother, []string{sampleDeploymentWithoutServiceYaml}, false)
	assert.NoError(t, err)
	err = helper.ValidatePodsAnnotations(nsAnother,
		getAnnotations(instrumentation.TypeJava, instrumentation.TypePython),
		getAnnotations(instrumentation.TypeDotNet, instrumentation.TypeNodeJS))
	assert.NoError(t, err)
	//check ns
	err = helper.ValidateNamespaceAnnotations(nsAnother,
		getAnnotations(instrumentation.TypeJava, instrumentation.TypePython),
		getAnnotations(instrumentation.TypeDotNet, instrumentation.TypeNodeJS))
	assert.NoError(t, err)
}

// Permutation 18 [HIGH]: Monitor all services with customSelector and specific languages
func TestPermutation18_MonitorWithCustomSelectorAndAutoRestarts(t *testing.T) {
	helper := NewTestHelper(t)

	// Create single namespace
	namespace := helper.Initialize("test-namespace")
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml, sampleDeploymentServiceYaml, sampleDeploymentWithoutServiceYaml}, false)
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
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		Languages:          instrumentation.NewTypeSet(instrumentation.TypeDotNet),
		RestartPods:        true,
		CustomSelector:     customSelectorConfig,
	})

	// Verify service-selected deployment has dotnet annotation
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName,
		getAnnotations(instrumentation.TypeDotNet),
		getAnnotations(instrumentation.TypeJava, instrumentation.TypePython, instrumentation.TypeNodeJS))
	assert.NoError(t, err)

	// Verify non-service deployment has python annotation
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentWithoutService,
		getAnnotations(instrumentation.TypePython),
		getAnnotations(instrumentation.TypeJava, instrumentation.TypeDotNet, instrumentation.TypeNodeJS))
	assert.NoError(t, err)
}

// Permutation 19 [HIGH++]: Test that exclude takes precedence over all
func TestPermutation19_ConflictingCustomSelectorExclude(t *testing.T) {
	helper := NewTestHelper(t)

	// Create single namespace
	namespace := helper.Initialize("test-namespace")

	// Create deployments in the namespace
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{
		sampleDeploymentYaml,
		sampleDeploymentServiceYaml,
		customerServiceYaml,
		conflictingDeploymentYaml,
	}, false)
	assert.NoError(t, err)

	// exclude config without namespace-level exclusion, will add later
	excludeConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Deployments: []string{namespace + "/" + customServiceDeploymentName},
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
	monitorConfig := &auto.MonitorConfig{
		MonitorAllServices: true,
		Languages:          instrumentation.NewTypeSet(instrumentation.TypeJava, instrumentation.TypePython, instrumentation.TypeNodeJS),
		RestartPods:        true,
		Exclude:            excludeConfig,
		CustomSelector:     customSelectorConfig,
	}

	helper.UpdateMonitorConfig(monitorConfig)

	// Verify conflicting-deployment has Python, NodeJS and Java
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, "conflicting-deployment",
		getAnnotations(instrumentation.TypePython, instrumentation.TypeNodeJS, instrumentation.TypeJava),
		getAnnotations(instrumentation.TypeDotNet))
	assert.NoError(t, err)

	// Verify customer-service has Python and NodeJS (Java excluded)
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, customServiceDeploymentName,
		getAnnotations(instrumentation.TypePython, instrumentation.TypeNodeJS),
		getAnnotations(instrumentation.TypeJava, instrumentation.TypeDotNet))
	assert.NoError(t, err)

	// Verify sample-deployment has Java, Python, and NodeJS
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName,
		getAnnotations(instrumentation.TypeJava, instrumentation.TypePython, instrumentation.TypeNodeJS),
		getAnnotations(instrumentation.TypeDotNet))
	assert.NoError(t, err)
}

// Permutation 20 [HIGH]: Disable general monitoring but enable specific instrumentation
func TestPermutation20_SelectiveMonitoringWithCustomSelector(t *testing.T) {
	helper := NewTestHelper(t)

	// Create single namespace
	namespace := helper.Initialize("test-namespace")

	// Create deployments
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{
		frontendAppYaml,
		adminDashboardYaml,
		sampleDeploymentYaml,
		sampleDeploymentWithoutServiceYaml,
	}, false)
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
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: false,
		Languages:          instrumentation.NewTypeSet(instrumentation.TypeJava),
		RestartPods:        true,
		CustomSelector:     customSelectorConfig,
	})

	// Verify frontend-app and admin-dashboard have NodeJS only
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, "frontend-app",
		getAnnotations(instrumentation.TypeNodeJS),
		getAnnotations(instrumentation.TypeJava, instrumentation.TypePython, instrumentation.TypeDotNet))
	assert.NoError(t, err)

	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, "admin-dashboard",
		getAnnotations(instrumentation.TypeNodeJS),
		getAnnotations(instrumentation.TypeJava, instrumentation.TypePython, instrumentation.TypeDotNet))
	assert.NoError(t, err)

	// Verify sample-deployment has no annotations
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName,
		none,
		allAnnotations)
	assert.NoError(t, err)

	// Verify sample-deployment-without-service has Python
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentWithoutService,
		getAnnotations(instrumentation.TypePython),
		getAnnotations(instrumentation.TypeJava, instrumentation.TypeDotNet, instrumentation.TypeNodeJS))
	assert.NoError(t, err)
}

// Permutation 21 [MED]: Enable monitoring AND No AutoRestart with granular configuration including languages, exclusion and custom selector
func TestPermutation21_SelectiveMonitoringWithCustomSelector(t *testing.T) {
	helper := NewTestHelper(t)

	nsPerf := "performance-sensitive"
	nsBatch := "batch-processing"
	nsFinance := "finance"
	nsDatabase := "database"
	namespace := helper.Initialize("test-namespace")

	// Update operator config
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		RestartPods:        true,
	})

	// Create deployments
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{customerServiceYaml}, false)
	assert.NoError(t, err)
	// more deployments to different NSes
	err = helper.CreateNamespaceAndApplyResources(nsPerf, []string{customerServiceYaml}, false)
	assert.NoError(t, err)
	err = helper.CreateNamespaceAndApplyResources(nsBatch, []string{customerServiceYaml}, false)
	assert.NoError(t, err)
	err = helper.CreateNamespaceAndApplyResources(nsFinance, []string{customerServiceYaml}, false)
	assert.NoError(t, err)
	err = helper.CreateNamespaceAndApplyResources(nsDatabase, []string{sampleStatefulsetYaml}, false)
	assert.NoError(t, err)

	//precheck on random deployments
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, customServiceDeploymentName, allAnnotations, none)
	assert.NoError(t, err)
	err = helper.ValidateWorkloadAnnotations(StatefulSet, nsDatabase, statefulSetName, allAnnotations, none)
	assert.NoError(t, err)

	// Update operator config
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		Languages:          instrumentation.NewTypeSet(instrumentation.TypeDotNet, instrumentation.TypeNodeJS),
		RestartPods:        false,
		CustomSelector: auto.AnnotationConfig{
			Java: auto.AnnotationResources{
				Namespaces: []string{nsBatch},
			},
		},
		Exclude: auto.AnnotationConfig{
			NodeJS: auto.AnnotationResources{
				Namespaces: []string{nsPerf},
			},
			DotNet: auto.AnnotationResources{
				Deployments:  []string{nsFinance + "/" + customServiceDeploymentName},
				StatefulSets: []string{nsDatabase + "/" + customServiceDeploymentName},
			},
		},
	})

	//postcheck1 with autoRestart disabled
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, customServiceDeploymentName, allAnnotations, none)
	assert.NoError(t, err)
	err = helper.ValidateWorkloadAnnotations(StatefulSet, nsDatabase, statefulSetName, allAnnotations, none)
	assert.NoError(t, err)

	//postcheck2 ns
	err = helper.ValidateNamespaceAnnotations(nsBatch,
		getAnnotations(instrumentation.TypeJava),
		getAnnotations(instrumentation.TypePython, instrumentation.TypeNodeJS, instrumentation.TypeDotNet))
	assert.NoError(t, err)

	//postcheck3 with manual restarts
	//other ns
	err = helper.RestartWorkload(Deployment, namespace, customServiceDeploymentName)
	assert.NoError(t, err)
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, customServiceDeploymentName,
		getAnnotations(instrumentation.TypeNodeJS, instrumentation.TypeDotNet),
		getAnnotations(instrumentation.TypeJava, instrumentation.TypePython))
	assert.NoError(t, err)
	// exclude nodejs by ns
	err = helper.RestartWorkload(Deployment, nsPerf, customServiceDeploymentName)
	assert.NoError(t, err)
	err = helper.ValidateWorkloadAnnotations(Deployment, nsPerf, customServiceDeploymentName,
		getAnnotations(instrumentation.TypeDotNet),
		getAnnotations(instrumentation.TypeJava, instrumentation.TypePython, instrumentation.TypeNodeJS))
	assert.NoError(t, err)
	// exclude dotnet by workloads
	err = helper.RestartWorkload(Deployment, nsFinance, customServiceDeploymentName)
	assert.NoError(t, err)
	err = helper.ValidateWorkloadAnnotations(Deployment, nsFinance, customServiceDeploymentName,
		getAnnotations(instrumentation.TypeNodeJS),
		getAnnotations(instrumentation.TypeJava, instrumentation.TypePython, instrumentation.TypeDotNet))
	assert.NoError(t, err)
	err = helper.RestartWorkload(StatefulSet, nsDatabase, statefulSetName)
	assert.NoError(t, err)
	err = helper.ValidateWorkloadAnnotations(StatefulSet, nsDatabase, statefulSetName, none, allAnnotations)
	assert.NoError(t, err)
	// include java only at pod level by custom selector
	err = helper.RestartWorkload(Deployment, nsBatch, customServiceDeploymentName)
	assert.NoError(t, err)
	err = helper.ValidatePodsAnnotations(nsBatch,
		getAnnotations(instrumentation.TypeDotNet, instrumentation.TypeNodeJS),
		getAnnotations(instrumentation.TypeJava, instrumentation.TypePython))
	assert.NoError(t, err) //FAILING: pod is annotated .net & nodejs
	err = helper.ValidateWorkloadAnnotations(Deployment, nsBatch, customServiceDeploymentName,
		getAnnotations(instrumentation.TypeNodeJS, instrumentation.TypeDotNet),
		getAnnotations(instrumentation.TypeJava, instrumentation.TypePython))
	assert.NoError(t, err)
}

// Permutation 22 [LOW]: Enable monitoring AND No AutoRestart with granular configuration including languages, exclusion and custom selector
func TestPermutation22_SelectiveMonitoringWithCustomSelector(t *testing.T) {
	helper := NewTestHelper(t)

	nsSecurity := "high-security"
	nsLegacy := "legacy"
	namespace := helper.Initialize("test-namespace")
	// Create deployments
	err := helper.CreateNamespaceAndApplyResources(namespace, []string{sampleDeploymentYaml, sampleDeploymentServiceYaml, customerServiceYaml}, false)
	assert.NoError(t, err)
	// more deployments to different NSes
	err = helper.CreateNamespaceAndApplyResources(nsSecurity, []string{customerServiceYaml}, false)
	assert.NoError(t, err)
	err = helper.CreateNamespaceAndApplyResources(nsLegacy, []string{customerServiceYaml}, false)
	assert.NoError(t, err)

	// Update operator config
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		Languages:          instrumentation.NewTypeSet(instrumentation.TypeJava, instrumentation.TypePython),
		RestartPods:        true,
		CustomSelector: auto.AnnotationConfig{
			DotNet: auto.AnnotationResources{
				Deployments: []string{nsLegacy + "/" + customServiceDeploymentName},
			},
		},
		Exclude: auto.AnnotationConfig{
			Java: auto.AnnotationResources{
				Namespaces:  []string{nsSecurity},
				Deployments: []string{namespace + "/" + customServiceDeploymentName},
			},
		},
	})

	//postcheck
	// java and python
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName,
		getAnnotations(instrumentation.TypeJava, instrumentation.TypePython),
		getAnnotations(instrumentation.TypeNodeJS, instrumentation.TypeDotNet))
	assert.NoError(t, err)
	// include dotnet by custom selector
	err = helper.ValidateWorkloadAnnotations(Deployment, nsLegacy, customServiceDeploymentName,
		getAnnotations(instrumentation.TypeJava, instrumentation.TypePython, instrumentation.TypeDotNet),
		getAnnotations(instrumentation.TypeNodeJS))
	assert.NoError(t, err)
	// exclude java by ns
	err = helper.ValidateWorkloadAnnotations(Deployment, nsSecurity, customServiceDeploymentName,
		getAnnotations(instrumentation.TypePython),
		getAnnotations(instrumentation.TypeJava, instrumentation.TypeNodeJS, instrumentation.TypeDotNet))
	assert.NoError(t, err)
	// exclude java by workload
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, customServiceDeploymentName,
		getAnnotations(instrumentation.TypePython),
		getAnnotations(instrumentation.TypeJava, instrumentation.TypeNodeJS, instrumentation.TypeDotNet))
	assert.NoError(t, err)

	// Update operator config by dropping languages
	helper.UpdateMonitorConfig(&auto.MonitorConfig{
		MonitorAllServices: true,
		RestartPods:        true,
		CustomSelector: auto.AnnotationConfig{
			DotNet: auto.AnnotationResources{
				Deployments: []string{nsLegacy + "/" + customServiceDeploymentName},
			},
		},
		Exclude: auto.AnnotationConfig{
			Java: auto.AnnotationResources{
				Namespaces:  []string{nsSecurity},
				Deployments: []string{namespace + "/" + customServiceDeploymentName},
			},
		},
	})

	//postcheck
	// all
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, deploymentName, allAnnotations, none)
	assert.NoError(t, err)
	err = helper.ValidateWorkloadAnnotations(Deployment, nsLegacy, customServiceDeploymentName, allAnnotations, none)
	assert.NoError(t, err)
	// exclude java by ns
	err = helper.ValidateWorkloadAnnotations(Deployment, nsSecurity, customServiceDeploymentName,
		getAnnotations(instrumentation.TypePython, instrumentation.TypeNodeJS, instrumentation.TypeDotNet),
		getAnnotations(instrumentation.TypeJava))
	assert.NoError(t, err)
	// exclude java by workload
	err = helper.ValidateWorkloadAnnotations(Deployment, namespace, customServiceDeploymentName,
		getAnnotations(instrumentation.TypePython, instrumentation.TypeNodeJS, instrumentation.TypeDotNet),
		getAnnotations(instrumentation.TypeJava))
	assert.NoError(t, err)
}

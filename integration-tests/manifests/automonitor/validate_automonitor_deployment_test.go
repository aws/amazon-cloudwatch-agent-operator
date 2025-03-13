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

//func TestAllLanguagesDeployment(t *testing.T) {
//
//	clientSet := setupTest(t)
//	randomNumber, err := rand.Int(rand.Reader, big.NewInt(9000))
//	if err != nil {
//		panic(err)
//	}
//	randomNumber.Add(randomNumber, big.NewInt(1000)) //adding a hash to namespace
//	uniqueNamespace := fmt.Sprintf("deployment-namespace-all-languages-%d", randomNumber)
//	annotationConfig := auto.AnnotationConfig{
//		Java: auto.AnnotationResources{
//			Namespaces:   []string{""},
//			DaemonSets:   []string{""},
//			Deployments:  []string{filepath.Join(uniqueNamespace, deploymentName)},
//			StatefulSets: []string{""},
//		},
//		Python: auto.AnnotationResources{
//			Namespaces:   []string{""},
//			DaemonSets:   []string{""},
//			Deployments:  []string{filepath.Join(uniqueNamespace, deploymentName)},
//			StatefulSets: []string{""},
//		},
//		DotNet: auto.AnnotationResources{
//			Namespaces:   []string{""},
//			DaemonSets:   []string{""},
//			Deployments:  []string{filepath.Join(uniqueNamespace, deploymentName)},
//			StatefulSets: []string{""},
//		},
//		NodeJS: auto.AnnotationResources{
//			Namespaces:   []string{""},
//			DaemonSets:   []string{""},
//			Deployments:  []string{filepath.Join(uniqueNamespace, deploymentName)},
//			StatefulSets: []string{""},
//		},
//	}
//	jsonStr, err := json.Marshal(annotationConfig)
//	assert.Nil(t, err)
//
//	startTime := time.Now()
//	updateOperatorConfig(t, clientSet, string(jsonStr))
//
//	if err := checkResourceAnnotations(t, clientSet, "deployment", uniqueNamespace, deploymentName, sampleDeploymentYamlNameRelPath, startTime, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation, injectPythonAnnotation, autoAnnotatePythonAnnotation, injectDotNetAnnotation, autoAnnotateDotNetAnnotation, injectNodeJSAnnotation, autoAnnotateNodeJSAnnotation}, false); err != nil {
//		t.Fatalf("Failed annotation check: %s", err.Error())
//	}
//
//}
//
//func TestJavaOnlyDeployment(t *testing.T) {
//
//	clientSet := setupTest(t)
//	randomNumber, err := rand.Int(rand.Reader, big.NewInt(9000))
//	if err != nil {
//		panic(err)
//	}
//	randomNumber.Add(randomNumber, big.NewInt(1000)) //adding a hash to namespace
//	uniqueNamespace := fmt.Sprintf("deployment-namespace-java-only-%d", randomNumber)
//
//	annotationConfig := auto.AnnotationConfig{
//		Java: auto.AnnotationResources{
//			Namespaces:   []string{""},
//			DaemonSets:   []string{""},
//			Deployments:  []string{filepath.Join(uniqueNamespace, deploymentName)},
//			StatefulSets: []string{""},
//		},
//		Python: auto.AnnotationResources{
//			Namespaces:   []string{""},
//			DaemonSets:   []string{""},
//			Deployments:  []string{""},
//			StatefulSets: []string{""},
//		},
//	}
//	jsonStr, err := json.Marshal(annotationConfig)
//	if err != nil {
//		t.Errorf("Failed to marshal: %v\n", err)
//	}
//	startTime := time.Now()
//	updateOperatorConfig(t, clientSet, string(jsonStr))
//
//	if err := checkResourceAnnotations(t, clientSet, "deployment", uniqueNamespace, deploymentName, sampleDeploymentYamlNameRelPath, startTime, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}, false); err != nil {
//		t.Fatalf("Failed annotation check: %s", err.Error())
//	}
//}
//
//func TestPythonOnlyDeployment(t *testing.T) {
//
//	clientSet := setupTest(t)
//	randomNumber, err := rand.Int(rand.Reader, big.NewInt(9000))
//	if err != nil {
//		panic(err)
//	}
//	randomNumber.Add(randomNumber, big.NewInt(1000)) //adding a hash to namespace
//	uniqueNamespace := fmt.Sprintf("deployment-namespace-python-only-%d", randomNumber)
//
//	annotationConfig := auto.AnnotationConfig{
//		Java: auto.AnnotationResources{
//			Namespaces:   []string{""},
//			DaemonSets:   []string{""},
//			Deployments:  []string{""},
//			StatefulSets: []string{""},
//		},
//		Python: auto.AnnotationResources{
//			Namespaces:   []string{""},
//			DaemonSets:   []string{""},
//			Deployments:  []string{filepath.Join(uniqueNamespace, deploymentName)},
//			StatefulSets: []string{""},
//		},
//	}
//	jsonStr, err := json.Marshal(annotationConfig)
//	if err != nil {
//		t.Error("Error:", err)
//	}
//
//	startTime := time.Now()
//	updateOperatorConfig(t, clientSet, string(jsonStr))
//	if err != nil {
//		t.Errorf("Failed to get deployment app: %s", err.Error())
//	}
//
//	if err := checkResourceAnnotations(t, clientSet, "deployment", uniqueNamespace, deploymentName, sampleDeploymentYamlNameRelPath, startTime, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}, false); err != nil {
//		t.Fatalf("Failed annotation check: %s", err.Error())
//	}
//
//}
//func TestDotNetOnlyDeployment(t *testing.T) {
//
//	clientSet := setupTest(t)
//	randomNumber, err := rand.Int(rand.Reader, big.NewInt(9000))
//	if err != nil {
//		panic(err)
//	}
//	randomNumber.Add(randomNumber, big.NewInt(1000)) //adding a hash to namespace
//	uniqueNamespace := fmt.Sprintf("deployment-namespace-dotnet-only-%d", randomNumber)
//
//	annotationConfig := auto.AnnotationConfig{
//		DotNet: auto.AnnotationResources{
//			Namespaces:   []string{""},
//			DaemonSets:   []string{""},
//			Deployments:  []string{filepath.Join(uniqueNamespace, deploymentName)},
//			StatefulSets: []string{""},
//		},
//	}
//	jsonStr, err := json.Marshal(annotationConfig)
//	if err != nil {
//		t.Error("Error:", err)
//	}
//
//	startTime := time.Now()
//	updateOperatorConfig(t, clientSet, string(jsonStr))
//	if err != nil {
//		t.Errorf("Failed to get deployment app: %s", err.Error())
//	}
//
//	if err := checkResourceAnnotations(t, clientSet, "deployment", uniqueNamespace, deploymentName, sampleDeploymentYamlNameRelPath, startTime, []string{injectDotNetAnnotation, autoAnnotateDotNetAnnotation}, false); err != nil {
//		t.Fatalf("Failed annotation check: %s", err.Error())
//	}
//
//}
//
//func TestNodeJSOnlyDeployment(t *testing.T) {
//
//	clientSet := setupTest(t)
//	randomNumber, err := rand.Int(rand.Reader, big.NewInt(9000))
//	if err != nil {
//		panic(err)
//	}
//	randomNumber.Add(randomNumber, big.NewInt(1000)) //adding a hash to namespace
//	uniqueNamespace := fmt.Sprintf("deployment-namespace-nodejs-only-%d", randomNumber)
//
//	annotationConfig := auto.AnnotationConfig{
//		NodeJS: auto.AnnotationResources{
//			Namespaces:   []string{""},
//			DaemonSets:   []string{""},
//			Deployments:  []string{filepath.Join(uniqueNamespace, deploymentName)},
//			StatefulSets: []string{""},
//		},
//	}
//	jsonStr, err := json.Marshal(annotationConfig)
//	if err != nil {
//		t.Error("Error:", err)
//		t.Error("Error:", err)
//
//	}
//
//	startTime := time.Now()
//	updateOperatorConfig(t, clientSet, string(jsonStr))
//	if err != nil {
//		t.Errorf("Failed to get deployment app: %s", err.Error())
//	}
//
//	if err := checkResourceAnnotations(t, clientSet, "deployment", uniqueNamespace, deploymentName, sampleDeploymentYamlNameRelPath, startTime, []string{injectNodeJSAnnotation, autoAnnotateNodeJSAnnotation}, false); err != nil {
//		t.Fatalf("Failed annotation check: %s", err.Error())
//	}
//
//}

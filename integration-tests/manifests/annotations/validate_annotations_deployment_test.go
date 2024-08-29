// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
package annotations

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/auto"
)

func TestAllLanguagesDeployment(t *testing.T) {

	clientSet := setupTest(t)
	randomNumber, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		panic(err)
	}
	randomNumber.Add(randomNumber, big.NewInt(1000)) //adding a hash to namespace
	uniqueNamespace := fmt.Sprintf("deployment-namespace-all-languages-%d", randomNumber)
	annotationConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{filepath.Join(uniqueNamespace, deploymentName)},
			StatefulSets: []string{""},
		},
		Python: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{filepath.Join(uniqueNamespace, deploymentName)},
			StatefulSets: []string{""},
		},
		DotNet: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{filepath.Join(uniqueNamespace, deploymentName)},
			StatefulSets: []string{""},
		},
	}
	jsonStr, err := json.Marshal(annotationConfig)
	assert.Nil(t, err)

	startTime := time.Now()
	updateTheOperator(t, clientSet, string(jsonStr))

	if err := checkResourceAnnotations(t, clientSet, "deployment", uniqueNamespace, deploymentName, sampleDeploymentYamlNameRelPath, startTime, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation, injectPythonAnnotation, autoAnnotatePythonAnnotation, injectDotNetAnnotation, autoAnnotateDotNetAnnotation}, false); err != nil {
		t.Fatalf("Failed annotation check: %s", err.Error())
	}

}

func TestJavaOnlyDeployment(t *testing.T) {

	clientSet := setupTest(t)
	randomNumber, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		panic(err)
	}
	randomNumber.Add(randomNumber, big.NewInt(1000)) //adding a hash to namespace
	uniqueNamespace := fmt.Sprintf("deployment-namespace-java-only-%d", randomNumber)

	annotationConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{filepath.Join(uniqueNamespace, deploymentName)},
			StatefulSets: []string{""},
		},
		Python: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
	}
	jsonStr, err := json.Marshal(annotationConfig)
	if err != nil {
		t.Errorf("Failed to marshal: %v\n", err)
	}
	startTime := time.Now()
	updateTheOperator(t, clientSet, string(jsonStr))

	if err := checkResourceAnnotations(t, clientSet, "deployment", uniqueNamespace, deploymentName, sampleDeploymentYamlNameRelPath, startTime, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}, false); err != nil {
		t.Fatalf("Failed annotation check: %s", err.Error())
	}
}

func TestPythonOnlyDeployment(t *testing.T) {

	clientSet := setupTest(t)
	randomNumber, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		panic(err)
	}
	randomNumber.Add(randomNumber, big.NewInt(1000)) //adding a hash to namespace
	uniqueNamespace := fmt.Sprintf("deployment-namespace-python-only-%d", randomNumber)

	annotationConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
		Python: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{filepath.Join(uniqueNamespace, deploymentName)},
			StatefulSets: []string{""},
		},
	}
	jsonStr, err := json.Marshal(annotationConfig)
	if err != nil {
		t.Error("Error:", err)
	}

	startTime := time.Now()
	updateTheOperator(t, clientSet, string(jsonStr))
	if err != nil {
		t.Errorf("Failed to get deployment app: %s", err.Error())
	}

	if err := checkResourceAnnotations(t, clientSet, "deployment", uniqueNamespace, deploymentName, sampleDeploymentYamlNameRelPath, startTime, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}, false); err != nil {
		t.Fatalf("Failed annotation check: %s", err.Error())
	}

}
func TestDotNetOnlyDeployment(t *testing.T) {

	clientSet := setupTest(t)
	randomNumber, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		panic(err)
	}
	randomNumber.Add(randomNumber, big.NewInt(1000)) //adding a hash to namespace
	uniqueNamespace := fmt.Sprintf("deployment-namespace-dotnet-only-%d", randomNumber)

	annotationConfig := auto.AnnotationConfig{
		DotNet: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{filepath.Join(uniqueNamespace, deploymentName)},
			StatefulSets: []string{""},
		},
	}
	jsonStr, err := json.Marshal(annotationConfig)
	if err != nil {
		t.Error("Error:", err)
	}

	startTime := time.Now()
	updateTheOperator(t, clientSet, string(jsonStr))
	if err != nil {
		t.Errorf("Failed to get deployment app: %s", err.Error())
	}

	if err := checkResourceAnnotations(t, clientSet, "deployment", uniqueNamespace, deploymentName, sampleDeploymentYamlNameRelPath, startTime, []string{injectDotNetAnnotation, autoAnnotateDotNetAnnotation}, false); err != nil {
		t.Fatalf("Failed annotation check: %s", err.Error())
	}

}

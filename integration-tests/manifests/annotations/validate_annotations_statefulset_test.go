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

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/auto"
)

func TestJavaAndPythonStatefulSet(t *testing.T) {

	clientSet := setupTest(t)
	randomNumber, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		panic(err)
	}
	randomNumber.Add(randomNumber, big.NewInt(1000)) //adding a hash to namespace
	uniqueNamespace := fmt.Sprintf("statefulset-namespace-java-python-%d", randomNumber)
	annotationConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{filepath.Join(uniqueNamespace, statefulSetName)},
		},
		Python: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{filepath.Join(uniqueNamespace, statefulSetName)},
		},
		DotNet: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{filepath.Join(uniqueNamespace, statefulSetName)},
		},
	}
	jsonStr, err := json.Marshal(annotationConfig)
	if err != nil {
		t.Error("Error:", err)
	}
	startTime := time.Now()
	updateTheOperator(t, clientSet, string(jsonStr))
	if err := checkResourceAnnotations(t, clientSet, "statefulset", uniqueNamespace, statefulSetName, sampleStatefulsetYamlNameRelPath, startTime, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation, injectPythonAnnotation, autoAnnotatePythonAnnotation, injectDotNetAnnotation, autoAnnotateDotNetAnnotation}, false); err != nil {
		t.Fatalf("Failed annotation check: %s", err.Error())
	}
}

func TestJavaOnlyStatefulSet(t *testing.T) {

	clientSet := setupTest(t)
	randomNumber, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		panic(err)
	}
	randomNumber.Add(randomNumber, big.NewInt(1000)) //adding a hash to namespace
	uniqueNamespace := fmt.Sprintf("statefulset-java-only-%d", randomNumber)

	annotationConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{filepath.Join(uniqueNamespace, statefulSetName)},
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
		t.Error("Error:", err)
	}
	startTime := time.Now()
	updateTheOperator(t, clientSet, string(jsonStr))
	if err := checkResourceAnnotations(t, clientSet, "statefulset", uniqueNamespace, statefulSetName, sampleStatefulsetYamlNameRelPath, startTime, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}, false); err != nil {
		t.Fatalf("Failed annotation check: %s", err.Error())
	}
}

func TestPythonOnlyStatefulSet(t *testing.T) {

	clientSet := setupTest(t)
	randomNumber, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		panic(err)
	}
	randomNumber.Add(randomNumber, big.NewInt(1000)) //adding a hash to namespace
	uniqueNamespace := fmt.Sprintf("statefulset-namespace-python-only-%d", randomNumber)
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
			Deployments:  []string{""},
			StatefulSets: []string{filepath.Join(uniqueNamespace, statefulSetName)},
		},
	}
	jsonStr, err := json.Marshal(annotationConfig)
	if err != nil {
		t.Error("Error:", err)
	}

	startTime := time.Now()
	updateTheOperator(t, clientSet, string(jsonStr))

	if err := checkResourceAnnotations(t, clientSet, "statefulset", uniqueNamespace, statefulSetName, sampleStatefulsetYamlNameRelPath, startTime, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}, false); err != nil {
		t.Fatalf("Failed annotation check: %s", err.Error())
	}
}

func TestDotNetOnlyStatefulSet(t *testing.T) {

	clientSet := setupTest(t)
	randomNumber, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		panic(err)
	}
	randomNumber.Add(randomNumber, big.NewInt(1000)) //adding a hash to namespace
	uniqueNamespace := fmt.Sprintf("statefulset-namespace-python-only-%d", randomNumber)
	annotationConfig := auto.AnnotationConfig{
		DotNet: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{filepath.Join(uniqueNamespace, statefulSetName)},
		},
	}
	jsonStr, err := json.Marshal(annotationConfig)
	if err != nil {
		t.Error("Error:", err)
	}

	startTime := time.Now()
	updateTheOperator(t, clientSet, string(jsonStr))

	if err := checkResourceAnnotations(t, clientSet, "statefulset", uniqueNamespace, statefulSetName, sampleStatefulsetYamlNameRelPath, startTime, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation, injectDotNetAnnotation, autoAnnotateDotNetAnnotation}, false); err != nil {
		t.Fatalf("Failed annotation check: %s", err.Error())
	}
}

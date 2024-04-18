// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
package annotations

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/auto"
	"math/big"
	"testing"
	"time"
)

func TestJavaAndPythonNamespace(t *testing.T) {

	clientSet := setupTest(t)
	randomNumber, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		panic(err)
	}
	randomNumber.Add(randomNumber, big.NewInt(1000)) //adding a hash to namespace
	uniqueNamespace := fmt.Sprintf("namespace-java-python-%d", randomNumber)

	annotationConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{uniqueNamespace},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
		Python: auto.AnnotationResources{
			Namespaces:   []string{uniqueNamespace},
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
	timeOut := 5 * time.Minute

	updateTheOperator(t, clientSet, string(jsonStr))

	if err := createNamespace(clientSet, uniqueNamespace); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespace(clientSet, uniqueNamespace); err != nil {
			t.Fatalf("Failed to delete namespace: %v", err)
		}
	}()

	for {
		//time.Sleep(5 * time.Second)
		if isNamespaceUpdated(clientSet, uniqueNamespace, startTime) {
			fmt.Printf("Namespace %s has been updated.\n", uniqueNamespace)
			break
		}
		elapsed := time.Since(startTime)
		if elapsed >= timeOut {
			fmt.Printf("Timeout reached while waiting for namespace %s to be updated.\n", uniqueNamespace)
			break
		}

	}

	fmt.Println("Done checking for namespace update.")

	if !checkNameSpaceAnnotations(clientSet, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation, injectPythonAnnotation, autoAnnotatePythonAnnotation}, uniqueNamespace) {
		t.Error("Missing java and python annotations")
	}

}

func TestJavaOnlyNamespace(t *testing.T) {

	clientSet := setupTest(t)
	randomNumber, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		panic(err)
	}
	randomNumber.Add(randomNumber, big.NewInt(1000)) //adding a hash to namespace
	uniqueNamespace := fmt.Sprintf("namespace-java-only-%d", randomNumber)
	annotationConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{uniqueNamespace},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
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
		t.Error("Error:", err)
	}
	startTime := time.Now()
	timeOut := 5 * time.Minute

	updateTheOperator(t, clientSet, string(jsonStr))
	if err := createNamespace(clientSet, uniqueNamespace); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespace(clientSet, uniqueNamespace); err != nil {
			t.Fatalf("Failed to delete namespace: %v", err)
		}
	}()

	for {
		if isNamespaceUpdated(clientSet, uniqueNamespace, startTime) {
			fmt.Printf("Namespace %s has been updated.\n", uniqueNamespace)
			break
		}
		elapsed := time.Since(startTime)
		if elapsed >= timeOut {
			fmt.Printf("Timeout reached while waiting for namespace %s to be updated.\n", uniqueNamespace)
			break
		}

	}

	fmt.Println("Done checking for namespace update.")

	if !checkNameSpaceAnnotations(clientSet, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}, uniqueNamespace) {
		t.Error("Missing Java annotations")
	}

}

func TestPythonOnlyNamespace(t *testing.T) {

	clientSet := setupTest(t)
	randomNumber, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		panic(err)
	}
	randomNumber.Add(randomNumber, big.NewInt(1000)) //adding a hash to namespace
	uniqueNamespace := fmt.Sprintf("namespace-python-only-%d", randomNumber)
	if err := createNamespace(clientSet, uniqueNamespace); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespace(clientSet, uniqueNamespace); err != nil {
			t.Fatalf("Failed to delete namespace: %v", err)
		}
	}()

	annotationConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
		Python: auto.AnnotationResources{
			Namespaces:   []string{uniqueNamespace},
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
	timeOut := 5 * time.Minute

	updateTheOperator(t, clientSet, string(jsonStr))

	for {
		if isNamespaceUpdated(clientSet, uniqueNamespace, startTime) {
			fmt.Printf("Namespace %s has been updated.\n", uniqueNamespace)
			break
		}
		elapsed := time.Since(startTime)
		if elapsed >= timeOut {
			fmt.Printf("Timeout reached while waiting for namespace %s to be updated.\n", uniqueNamespace)
			break
		}
	}
	fmt.Println("Done checking for namespace update.")

	if !checkNameSpaceAnnotations(clientSet, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}, uniqueNamespace) {
		t.Error("Missing Python annotations")
	}
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
package annotations

import (
	"encoding/json"
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/auto"
	"testing"
	"time"
)

// ---------------------------USE CASE 7 (Java and Python on Namespace) ----------------------------------------------
func TestJavaAndPythonNamespace(t *testing.T) {

	clientSet := setupTest(t)
	sampleNamespace := "namespace-java-python"
	if err := createNamespace(clientSet, sampleNamespace); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespace(clientSet, sampleNamespace); err != nil {
			t.Fatalf("Failed to delete namespace: %v", err)
		}
	}()

	annotationConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{sampleNamespace},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
		Python: auto.AnnotationResources{
			Namespaces:   []string{sampleNamespace},
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
		time.Sleep(15 * time.Second)
		if isNamespaceUpdated(clientSet, sampleNamespace, startTime) {
			fmt.Printf("Namespace %s has been updated.\n", sampleNamespace)
			break
		}
		elapsed := time.Since(startTime)
		if elapsed >= timeOut {
			fmt.Printf("Timeout reached while waiting for namespace %s to be updated.\n", sampleNamespace)
			break
		}

	}

	fmt.Println("Done checking for namespace update.")

	if !checkNameSpaceAnnotations(clientSet, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation, injectPythonAnnotation, autoAnnotatePythonAnnotation}, sampleNamespace) {
		t.Error("Missing java and python annotations")
	}

}

// ---------------------------USE CASE 8 (Java on Namespace Python should be removed) ----------------------------------------------
func TestJavaOnlyNamespace(t *testing.T) {

	clientSet := setupTest(t)
	sampleNamespace := "namespace-java-only"
	if err := createNamespace(clientSet, sampleNamespace); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespace(clientSet, sampleNamespace); err != nil {
			t.Fatalf("Failed to delete namespace: %v", err)
		}
	}()
	annotationConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{sampleNamespace},
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

	for {
		time.Sleep(15 * time.Second)

		if isNamespaceUpdated(clientSet, sampleNamespace, startTime) {
			fmt.Printf("Namespace %s has been updated.\n", sampleNamespace)
			break
		}
		elapsed := time.Since(startTime)
		if elapsed >= timeOut {
			fmt.Printf("Timeout reached while waiting for namespace %s to be updated.\n", sampleNamespace)
			break
		}

	}

	fmt.Println("Done checking for namespace update.")

	if !checkNameSpaceAnnotations(clientSet, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}, sampleNamespace) {
		t.Error("Missing Java annotations")
	}
	//------------------------------------USE CASE 8 End ----------------------------------------------

}

// ---------------------------USE CASE 9 (Python on Namespace and Java annotation should not exist) ----------------------------------------------

func TestPythonOnlyNamespace(t *testing.T) {

	clientSet := setupTest(t)
	sampleNamespace := "namespace-python-only"
	if err := createNamespace(clientSet, sampleNamespace); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespace(clientSet, sampleNamespace); err != nil {
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
			Namespaces:   []string{sampleNamespace},
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
		time.Sleep(15 * time.Second)

		if isNamespaceUpdated(clientSet, sampleNamespace, startTime) {
			fmt.Printf("Namespace %s has been updated.\n", sampleNamespace)
			break
		}
		elapsed := time.Since(startTime)
		if elapsed >= timeOut {
			fmt.Printf("Timeout reached while waiting for namespace %s to be updated.\n", sampleNamespace)
			break
		}
	}
	fmt.Println("Done checking for namespace update.")

	if !checkNameSpaceAnnotations(clientSet, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}, sampleNamespace) {
		t.Error("Missing Python annotations")
	}
}

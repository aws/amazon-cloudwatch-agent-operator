// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package annotations

import (
	"context"
	"encoding/json"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/auto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
	"time"
)

// ---------------------------USE CASE 7 (Java and Python on Namespace) ----------------------------------------------
func TestJavaAndPythonNamespace(t *testing.T) {

	t.Parallel()
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

	updateTheOperator(t, clientSet, string(jsonStr))
	time.Sleep(25 * time.Second)

	ns, err := clientSet.CoreV1().Namespaces().Get(context.TODO(), sampleNamespace, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting namespace %s", err.Error())
	}
	if !checkNameSpaceAnnotations(ns, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation, injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		t.Error("Missing java and python annotations")
	}

}

// ---------------------------USE CASE 8 (Java on Namespace Python should be removed) ----------------------------------------------
func TestJavaOnlyNamespace(t *testing.T) {

	t.Parallel()
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

	updateTheOperator(t, clientSet, string(jsonStr))

	//let namspace update
	time.Sleep(25 * time.Second)
	ns, err := clientSet.CoreV1().Namespaces().Get(context.TODO(), sampleNamespace, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting namespace %s", err.Error())
	}

	if !checkNameSpaceAnnotations(ns, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}) {
		t.Error("Missing Java annotations")
	}
	//------------------------------------USE CASE 8 End ----------------------------------------------

}

// ---------------------------USE CASE 9 (Python on Namespace and Java annotation should not exist) ----------------------------------------------
func TestPythonOnlyNamespace(t *testing.T) {

	t.Parallel()
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

	updateTheOperator(t, clientSet, string(jsonStr))
	time.Sleep(25 * time.Second)

	ns, err := clientSet.CoreV1().Namespaces().Get(context.TODO(), sampleNamespace, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting namespace %s", err.Error())
	}
	//java annotations should not exist anymore
	if checkNameSpaceAnnotations(ns, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}) {
		t.Error("Java annotations should not exist")
	}
	if !checkNameSpaceAnnotations(ns, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		t.Error("Missing Python annotations")
	}

}

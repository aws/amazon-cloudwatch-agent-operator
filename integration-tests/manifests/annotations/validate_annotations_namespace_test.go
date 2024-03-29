// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package annotations

import (
	"context"
	"encoding/json"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/auto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

// ---------------------------USE CASE 7 (Java and Python on Namespace) ----------------------------------------------
func TestUseCase7(t *testing.T) {

	t.Parallel()
	clientSet := setupTest(t)
	sampleNamespace := "sample-namespace-7"

	if err := createNamespace(clientSet, sampleNamespace); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespace(clientSet, sampleNamespace); err != nil {
			t.Fatalf("Failed to delete namespace: %v", err)
		}
	}()
	//updating operator deployment
	deployment, err := clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting deployment: %v\n\n", err)
	}

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
	deployment, err = clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting deployment: %v\n\n", err)
		//	}
		updateAnnotationConfig(deployment, string(jsonStr))
		if !updateOperator(t, clientSet, deployment) {
			t.Error("Failed to update Operator")
		}
		ns, err := clientSet.CoreV1().Namespaces().Get(context.TODO(), sampleNamespace, metav1.GetOptions{})
		if err != nil {
			t.Errorf("Error getting namespace %s", err.Error())
		}
		if !checkNameSpaceAnnotations(ns, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation, injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
			t.Error("Incorrect Annotations")
		}

	}
}

// ---------------------------USE CASE 8 (Java on Namespace Python should be removed) ----------------------------------------------
func TestUseCase8(t *testing.T) {

	t.Parallel()
	clientSet := setupTest(t)
	sampleNamespace := "sample-namespace-8"
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
	deployment, err := clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting deployment: %v\n\n", err)
	}

	updateAnnotationConfig(deployment, string(jsonStr))
	if !updateOperator(t, clientSet, deployment) {
		t.Error("Failed to update Operator")
	}
	ns, err := clientSet.CoreV1().Namespaces().Get(context.TODO(), sampleNamespace, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting namespace %s", err.Error())
	}

	//python should not exist
	if checkNameSpaceAnnotations(ns, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		t.Error("Incorrect Annotations")

	}

	if !checkNameSpaceAnnotations(ns, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}) {
		t.Error("Incorrect Annotations")
	}
	//------------------------------------USE CASE 8 End ----------------------------------------------

}

// ---------------------------USE CASE 9 (Python on Namespace and Java annotation should not exist) ----------------------------------------------
func TestUseCase9(t *testing.T) {

	t.Parallel()
	clientSet := setupTest(t)
	sampleNamespace := "sample-namespace-9"
	if err := createNamespace(clientSet, sampleNamespace); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespace(clientSet, sampleNamespace); err != nil {
			t.Fatalf("Failed to delete namespace: %v", err)
		}
	}()
	//updating operator deployment
	deployment, err := clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting deployment: %v\n\n", err)
	}

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
	deployment, err = clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting deployment: %v\n\n", err)
	}
	updateAnnotationConfig(deployment, string(jsonStr))
	if !updateOperator(t, clientSet, deployment) {
		t.Error("Failed to update Operator")
	}
	ns, err := clientSet.CoreV1().Namespaces().Get(context.TODO(), sampleNamespace, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting namespace %s", err.Error())
	}
	//java annotations should not exist anymore
	if checkNameSpaceAnnotations(ns, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}) {
		t.Error("Incorrect Annotations")
	}
	if !checkNameSpaceAnnotations(ns, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		t.Error("Incorrect Annotations")
	}

}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
package annotations

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/auto"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"path/filepath"
	"testing"
	"time"
)

// ---------------------------USE CASE 1 (Java and Python on Deployment) ----------------------------------------------
func TestJavaAndPythonDeployment(t *testing.T) {

	clientSet := setupTest(t)
	uniqueNamespace := "deployment-namespace-java-python"
	if err := createNamespaceAndApplyResources(t, clientSet, uniqueNamespace, []string{"sample-deployment.yaml"}); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespaceAndResources(clientSet, uniqueNamespace, []string{"sample-deployment.yaml"}); err != nil {
			t.Fatalf("Failed to delete namespaces/resources: %v", err)
		}
	}()
	//updating operator deployment

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
	}
	jsonStr, err := json.Marshal(annotationConfig)
	assert.Nil(t, err)

	startTime := time.Now()
	updateTheOperator(t, clientSet, string(jsonStr))
	if err != nil {
		t.Errorf("Failed to get deployment app: %s", err.Error())
	}

	//check if deployment has annotations.
	if err != nil {
		t.Errorf("Failed to get deployment app: %s", err.Error())
	}
	deployment, err := clientSet.AppsV1().Deployments(uniqueNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get deployment: %s", err.Error())
	}

	// List pods belonging to the deployment

	err = waitForNewPodCreation(clientSet, deployment, startTime, 60*time.Second)

	fmt.Println("All pods have completed updating.")
	deploymentPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{})
	fmt.Println("All pods have completed updating.")

	//wait for pods to update
	if !checkIfAnnotationExists(clientSet, deploymentPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation, injectPythonAnnotation, autoAnnotatePythonAnnotation}, 90*time.Second) {
		t.Error("Missing Java and Python Annotations")
	}

}

// ---------------------------USE CASE 2 (Java on Deployment and Python Should be Removed)------------------------------
func TestJavaOnlyDeployment(t *testing.T) {

	clientSet := setupTest(t)
	uniqueNamespace := "deployment-namespace-java-only"
	if err := createNamespaceAndApplyResources(t, clientSet, uniqueNamespace, []string{"sample-deployment.yaml"}); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespaceAndResources(clientSet, uniqueNamespace, []string{"sample-deployment.yaml"}); err != nil {
			t.Fatalf("Failed to delete namespaces/resources: %v", err)
		}
	}()

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

	//finding where index of --auto-annotation-config= is (if it doesn't exist it will be appended)
	updateTheOperator(t, clientSet, string(jsonStr))
	startTime := time.Now()
	updateTheOperator(t, clientSet, string(jsonStr))
	deployment, err := clientSet.AppsV1().Deployments(uniqueNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get deployment: %s", err.Error())
	}

	err = waitForNewPodCreation(clientSet, deployment, startTime, 60*time.Second)

	fmt.Println("All pods have completed updating.")
	deploymentPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{})

	//wait for pods to update
	if !checkIfAnnotationExists(clientSet, deploymentPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}, 60*time.Second) {
		t.Error("Missing Java Annotations")
	}

}

// ---------------------------USE CASE 3 (Python on Deployment and java annotations should be removed) ----------------------------------------------
func TestPythonOnlyDeployment(t *testing.T) {

	clientSet := setupTest(t)
	uniqueNamespace := "deployment-namespace-python-only"
	if err := createNamespaceAndApplyResources(t, clientSet, uniqueNamespace, []string{"sample-deployment.yaml"}); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespaceAndResources(clientSet, uniqueNamespace, []string{"sample-deployment.yaml"}); err != nil {
			t.Fatalf("Failed to delete namespaces/resources: %v", err)
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

	//check if deployment has annotations.
	deployment, err := clientSet.AppsV1().Deployments(uniqueNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get deployment: %s", err.Error())
	}

	err = waitForNewPodCreation(clientSet, deployment, startTime, 60*time.Second)

	deploymentPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{})

	fmt.Println("\n\n\n\nPods:")
	for _, pod := range deploymentPods.Items {
		fmt.Printf("%s\n", pod.GetName())
	}

	if err != nil {
		t.Errorf("Error listing pods for deployment: %s", err.Error())
	}

	//wait for pods to update
	if !checkIfAnnotationExists(clientSet, deploymentPods, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}, 60*time.Second) {
		t.Error("Missing Python Annotations")
	}

}

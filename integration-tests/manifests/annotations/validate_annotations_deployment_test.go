// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package annotations

import (
	"context"
	"encoding/json"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/auto"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"path/filepath"
	"testing"
)

// ---------------------------USE CASE 1 (Java and Python on Deployment) ----------------------------------------------
func TestJavaAndPythonDeployment(t *testing.T) {

	t.Parallel()
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

	updateTheOperator(t, clientSet, string(jsonStr))

	//check if deployment has annotations.
	deployment, err := clientSet.AppsV1().Deployments(uniqueNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get deployment app: %s", err.Error())
	}

	// List pods belonging to the deployment
	set := labels.Set(deployment.Spec.Selector.MatchLabels)
	deploymentPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})
	if err != nil {
		t.Errorf("Error listing pods for deployment: %s", err.Error())
	}

	//wait for pods to update
	if !checkIfAnnotationExists(deploymentPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation, injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		t.Error("Missing Java and Python Annotations")
	}

}

// ---------------------------USE CASE 2 (Java on Deployment and Python Should be Removed)------------------------------
func TestJavaOnlyDeployment(t *testing.T) {

	t.Parallel()
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

	//check if deployment has annotations.
	deployment, err := clientSet.AppsV1().Deployments(uniqueNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		if err != nil {
			t.Errorf("Error listing pods for deployment: %s", err.Error())
		}
	}

	// List pods belonging to the deployment
	set := labels.Set(deployment.Spec.Selector.MatchLabels)
	deploymentPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})
	if err != nil {
		t.Error("Failed to update Operator")

	}

	if checkIfAnnotationExists(deploymentPods, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		t.Error("Python Annotation should not exist")

	}
	//wait for pods to update
	if !checkIfAnnotationExists(deploymentPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}) {
		t.Error("Missing Java Annotations")
	}

}

// ---------------------------USE CASE 3 (Python on Deployment and java annotations should be removed) ----------------------------------------------
func TestPythonOnlyDeployment(t *testing.T) {

	t.Parallel()
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

	updateTheOperator(t, clientSet, string(jsonStr))

	//check if deployment has annotations.
	deployment, err := clientSet.AppsV1().Deployments(uniqueNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get deployment: %s", err.Error())
	}

	// List pods belonging to the deployment
	set := labels.Set(deployment.Spec.Selector.MatchLabels)
	deploymentPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})
	if err != nil {
		t.Errorf("Error listing pods for deployment: %s", err.Error())
	}

	//java shouldn't be annotated in this case
	if checkIfAnnotationExists(deploymentPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}) {
		t.Error("Java Annotations should not exist")

	}
	//wait for pods to update
	if !checkIfAnnotationExists(deploymentPods, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		t.Error("Missing Python Annotations")
	}

}

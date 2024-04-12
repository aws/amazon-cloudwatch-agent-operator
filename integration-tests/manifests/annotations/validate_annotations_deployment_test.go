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
	"k8s.io/apimachinery/pkg/labels"
	"path/filepath"
	"testing"
	"time"
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
	operator, err := clientSet.AppsV1().Deployments("amazon-cloudwatch").Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get deployment app: %s", err.Error())
	}

	//check if deployment has annotations.
	if err != nil {
		t.Errorf("Failed to get deployment app: %s", err.Error())
	}
	fmt.Printf("\n\n\nThis is the operator for the deployemnet name: %v, whose unique namespace is %v, namespace %v, annotation config of operator is %v", operator.Name, uniqueNamespace, operator.Namespace, operator.Spec.Template.Spec.Containers[0].Args)
	deployment, err := clientSet.AppsV1().Deployments(uniqueNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error listing pods for deployment: %s", err.Error())
	}
	set := labels.Set(deployment.Spec.Selector.MatchLabels)

	// List pods belonging to the deployment
	for {
		deploymentPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: set.AsSelector().String(),
		})
		if err != nil {
			panic(err.Error())
		}

		// Check if any pod is in the updating stage
		if !podsInUpdatingStage(deploymentPods.Items) {
			break // Exit loop if no pods are updating
		}

		// Sleep for a short duration before checking again
		time.Sleep(10 * time.Second)
	}
	deploymentPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{})
	fmt.Println("All pods have completed updating.")

	if err != nil {
		t.Errorf("Error listing pods for deployment: %s", err.Error())
	}
	fmt.Println("\n\n\n\nPods:")
	for _, pod := range deploymentPods.Items {
		fmt.Printf("%s\n", pod.GetName())
	}
	//wait for pods to update
	if !checkIfAnnotationExists(clientSet, deploymentPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation, injectPythonAnnotation, autoAnnotatePythonAnnotation}, 60*time.Second) {
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
		t.Errorf("Error listing pods for deployment: %s", err.Error())
	}
	operator, err := clientSet.AppsV1().Deployments("amazon-cloudwatch").Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get deployment app: %s", err.Error())
	}

	fmt.Printf("\n\n\nThis is the operator for the deployemnet name: %v, whose unique namespace is %v, namespace %v, annotation config of operator is %v", deployment.Name, uniqueNamespace, deployment.Namespace, operator.Spec.Template.Spec.Containers[0].Args)
	set := labels.Set(deployment.Spec.Selector.MatchLabels)

	// List pods belonging to the deployment
	for {
		deploymentPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: set.AsSelector().String(),
		})
		if err != nil {
			panic(err.Error())
		}

		// Check if any pod is in the updating stage
		if !podsInUpdatingStage(deploymentPods.Items) {
			break // Exit loop if no pods are updating
		}

		// Sleep for a short duration before checking again
		time.Sleep(10 * time.Second)
	}
	deploymentPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{})

	fmt.Println("All pods have completed updating.")

	fmt.Println("\n\n\n\nPods:")
	for _, pod := range deploymentPods.Items {
		fmt.Printf("%s\n", pod.GetName())
	}
	if err != nil {
		t.Error("Failed to update Operator")

	}

	//wait for pods to update
	if !checkIfAnnotationExists(clientSet, deploymentPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}, 60*time.Second) {
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
	operator, err := clientSet.AppsV1().Deployments("amazon-cloudwatch").Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get deployment app: %s", err.Error())
	}

	//check if deployment has annotations.
	deployment, err := clientSet.AppsV1().Deployments(uniqueNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get deployment: %s", err.Error())
	}
	fmt.Printf("\n\n\nThis is the operator for the deployemnet name: %v, whose unique namespace is %v, namespace %v, annotation config of operator is %v", deployment.Name, uniqueNamespace, deployment.Namespace, operator.Spec.Template.Spec.Containers[0].Args)

	// List pods belonging to the deployment
	set := labels.Set(deployment.Spec.Selector.MatchLabels)

	for {
		deploymentPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: set.AsSelector().String(),
		})
		if err != nil {
			panic(err.Error())
		}

		// Check if any pod is in the updating stage
		if !podsInUpdatingStage(deploymentPods.Items) {
			break // Exit loop if no pods are updating
		}

		// Sleep for a short duration before checking again
		time.Sleep(10 * time.Second)
	}
	deploymentPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{})

	fmt.Println("\n\n\n\nPods:")
	for _, pod := range deploymentPods.Items {
		fmt.Printf("%s\n", pod.GetName())
	}

	if err != nil {
		t.Errorf("Error listing pods for deployment: %s", err.Error())
	}

	//java shouldn't be annotated in this case

	//wait for pods to update
	if !checkIfAnnotationExists(clientSet, deploymentPods, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}, 60*time.Second) {
		t.Error("Missing Python Annotations")
	}

}

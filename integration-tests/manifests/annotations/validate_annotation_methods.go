// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package annotations

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/labels"
	"sync"
	"testing"

	appsV1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const injectJavaAnnotation = "instrumentation.opentelemetry.io/inject-java"
const autoAnnotateJavaAnnotation = "cloudwatch.aws.amazon.com/auto-annotate-java"
const injectPythonAnnotation = "instrumentation.opentelemetry.io/inject-python"
const autoAnnotatePythonAnnotation = "cloudwatch.aws.amazon.com/auto-annotate-python"
const deploymentName = "sample-deployment"
const statefulSetName = "sample-statefulset"
const amazonCloudwatchNamespace = "amazon-cloudwatch"

const daemonSetName = "sample-daemonset"

const amazonControllerManager = "cloudwatch-controller-manager"

var opMutex sync.Mutex

// Function where you lock the mutex
func functionWithLock() {
	opMutex.Lock()
	// Critical section of code where you need the mutex
	// Perform operations while the mutex is locked
}

// Another function where you unlock the mutex
func unlockLock() {
	// Defer the unlock so it is executed when the function exits
	defer opMutex.Unlock()
	// Perform operations before unlocking the mutex
}
func applyYAMLWithKubectl(filename, namespace string) error {
	cmd := exec.Command("kubectl", "apply", "-f", filename, "-n", namespace)
	return cmd.Run()
}

// Usage in your createNamespace function
func createNamespaceAndApplyResources(t *testing.T, clientset *kubernetes.Clientset, name string, resourceFiles []string) error {
	// Create Namespace
	err := createNamespace(clientset, name)
	if err != nil {
		return err
	}

	time.Sleep(15 * time.Second)
	// Apply each YAML file
	for _, file := range resourceFiles {
		err = applyYAMLWithKubectl(filepath.Join("..", file), name)
		if err != nil {
			t.Error("Could not apply resources")
			return err
		}
	}
	time.Sleep(15 * time.Second)

	return nil
}

func deleteYAMLWithKubectl(filename, namespace string) error {
	cmd := exec.Command("kubectl", "delete", "-f", filename, "-n", namespace)
	return cmd.Run()
}

func deleteNamespaceAndResources(clientset *kubernetes.Clientset, name string, resourceFiles []string) error {
	// Delete each YAML file
	for _, file := range resourceFiles {
		err := deleteYAMLWithKubectl(filepath.Join("..", file), name)
		time.Sleep(5 * time.Second)

		if err != nil {
			return err
		}
	}

	// Delete Namespace
	err := deleteNamespace(clientset, name)
	time.Sleep(15 * time.Second)
	unlockLock()
	return err
}
func createNamespace(clientset *kubernetes.Clientset, name string) error {
	// Check if the namespace already exists
	_, err := clientset.CoreV1().Namespaces().Get(context.Background(), name, metav1.GetOptions{})
	if err == nil {
		return nil

	} else if !errors.IsNotFound(err) {
		// Other error while getting namespace
		return err
	}

	// Create the namespace if it doesn't exist
	namespace := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	_, err = clientset.CoreV1().Namespaces().Create(context.Background(), namespace, metav1.CreateOptions{})
	time.Sleep(25 * time.Second)
	return err
}

func deleteNamespace(clientset *kubernetes.Clientset, name string) error {
	err := clientset.CoreV1().Namespaces().Delete(context.Background(), name, metav1.DeleteOptions{})
	time.Sleep(25 * time.Second)
	return err
}

func checkNameSpaceAnnotations(ns *v1.Namespace, expectedAnnotations []string) bool {
	time.Sleep(20 * time.Second)
	for _, annotation := range expectedAnnotations {
		if ns.ObjectMeta.Annotations[annotation] != "true" {
			return false
		}
	}
	fmt.Println("Namespace annotations are correct!")
	return true
}

func areAllPodsUpdated(clientSet *kubernetes.Clientset, deployment *appsV1.Deployment) bool {
	// Get the list of pods for the deployment in all namespaces
	pods, err := clientSet.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
		LabelSelector: labels.Set(deployment.Spec.Selector.MatchLabels).AsSelector().String(),
	})
	if err != nil {
		return false
	}

	// Check if all pods are updated
	for _, pod := range pods.Items {
		if pod.Status.Phase != v1.PodRunning {
			return false
		}
	}
	return true
}

func updateOperator(t *testing.T, clientSet *kubernetes.Clientset, deployment *appsV1.Deployment) bool {
	var err error
	args := deployment.Spec.Template.Spec.Containers[0].Args
	updated := false

	// Attempt to get the deployment by name
	deployment, err = clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	deployment.Spec.Template.Spec.Containers[0].Args = args
	if err != nil {
		t.Errorf("Failed to get deployment: %v\n", err)
		return false
	}

	// Update the deployment and check its status up to 10 attempts
	for attempt := 1; attempt <= 3; attempt++ {
		_, err = clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
		if err != nil {
			t.Errorf("Failed to update deployment: %v\n", err)
			return false
		}

		fmt.Println("Deployment updated successfully!")

		// Wait for deployment to stabilize
		time.Sleep(45 * time.Second)

		// Check if all pods are updated
		if areAllPodsUpdated(clientSet, deployment) {
			updated = true
			break
		}

		// Pods are not all updated, wait and try again
		fmt.Printf("Attempt %d: Pods are not all updated, retrying...\n", attempt)
	}

	if !updated {
		t.Error("Deployment not fully updated after 10 attempts")
		return false
	}

	return true
}
func podsInUpdatingStage(pods []v1.Pod) bool {
	for _, pod := range pods {
		for _, condition := range pod.Status.Conditions {
			if condition.Type == v1.ContainersReady && condition.Status == v1.ConditionFalse {
				return true // Pod is in the updating stage
			}
		}
	}
	return false // No pod is in the updating stage
}
func checkIfAnnotationExists(clientset *kubernetes.Clientset, pods *v1.PodList, expectedAnnotations []string, retryDuration time.Duration) bool {
	startTime := time.Now()
	for {
		if time.Since(startTime) > retryDuration*3 {
			fmt.Println("Timeout reached while waiting for annotations.")
			return false
		}

		foundAllAnnotations := true
		for _, pod := range pods.Items {
			for _, annotation := range expectedAnnotations {
				fmt.Printf("Checking pod %s for annotation %s\n", pod.Name, annotation)
				if pod.Annotations[annotation] != "true" {
					foundAllAnnotations = false
					break
				}
			}
			if !foundAllAnnotations {
				break
			}
		}

		if foundAllAnnotations {
			fmt.Println("Annotations are correct!")
			return true
		}

		fmt.Println("Annotations not found in all pods. Retrying...")
		time.Sleep(5 * time.Second) // Wait before retrying
	}
}
func updateAnnotationConfig(deployment *appsV1.Deployment, jsonStr string) *appsV1.Deployment {

	args := deployment.Spec.Template.Spec.Containers[0].Args
	indexOfAutoAnnotationConfigString := findIndexOfPrefix("--auto-annotation-config=", args)
	//if auto annotation not part of config, we will add it
	if indexOfAutoAnnotationConfigString < 0 {
		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, "--auto-annotation-config="+jsonStr)
		indexOfAutoAnnotationConfigString = len(deployment.Spec.Template.Spec.Containers[0].Args) - 1
	} else {
		deployment.Spec.Template.Spec.Containers[0].Args[indexOfAutoAnnotationConfigString] = "--auto-annotation-config=" + jsonStr
	}
	time.Sleep(5 * time.Second)
	return deployment
}
func findIndexOfPrefix(str string, strs []string) int {
	for i, s := range strs {
		if strings.HasPrefix(s, str) {
			return i
		}
	}
	return -1
}
func setupTest(t *testing.T) *kubernetes.Clientset {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		t.Errorf("error getting user home dir: %v\n\n", err)
	}
	kubeConfigPath := filepath.Join(userHomeDir, ".kube", "config")
	fmt.Printf("Using kubeconfig: %s\n\n", kubeConfigPath)

	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		t.Errorf("Error getting kubernetes config: %v\n\n", err)
	}

	clientSet, err := kubernetes.NewForConfig(kubeConfig)

	if err != nil {
		t.Errorf("error getting kubernetes config: %v\n\n", err)
	}
	return clientSet
}
func updateTheOperator(t *testing.T, clientSet *kubernetes.Clientset, jsonStr string) {
	functionWithLock()
	deployment, err := clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting deployment: %v\n\n", err)
	}
	deployment = updateAnnotationConfig(deployment, jsonStr)
	if !updateOperator(t, clientSet, deployment) {
		t.Error("Failed to update Operator")
	}
	fmt.Println("Ended")

}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package annotations

import (
	"context"
	"fmt"
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

	time.Sleep(25 * time.Second)
	// Apply each YAML file
	for _, file := range resourceFiles {
		err = applyYAMLWithKubectl(filepath.Join("..", file), name)
		if err != nil {
			t.Error("Could not apply resources")
			return err
		}
	}
	time.Sleep(25 * time.Second)

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
		if err != nil {
			return err
		}
	}

	// Delete Namespace
	err := deleteNamespace(clientset, name)
	time.Sleep(25 * time.Second)
	return err
}
func createNamespace(clientset *kubernetes.Clientset, name string) error {
	// Check if the namespace already exists
	_, err := clientset.CoreV1().Namespaces().Get(context.Background(), name, metav1.GetOptions{})
	if err == nil {
		return nil // Use this line if you want to just use the existing namespace

	} else if !errors.IsNotFound(err) {
		// Other error while getting namespace
		return err
	}

	// Create the namespace if it doesn't exist
	namespace := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	_, err = clientset.CoreV1().Namespaces().Create(context.Background(), namespace, metav1.CreateOptions{})
	return err
}

func deleteNamespace(clientset *kubernetes.Clientset, name string) error {
	return clientset.CoreV1().Namespaces().Delete(context.Background(), name, metav1.DeleteOptions{})
}

func checkNameSpaceAnnotations(ns *v1.Namespace, expectedAnnotations []string) bool {
	for _, annotation := range expectedAnnotations {
		if ns.ObjectMeta.Annotations[annotation] != "true" {
			return false
		}
	}
	fmt.Println("Namespace annotations are correct!")
	return true
}
func updateOperator(t *testing.T, clientSet *kubernetes.Clientset, deployment *appsV1.Deployment) bool {
	// Lock the mutex at the beginning
	var err error
	args := deployment.Spec.Template.Spec.Containers[0].Args
	// Attempt to get the deployment by name
	deployment, err = clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	deployment.Spec.Template.Spec.Containers[0].Args = args
	if err != nil {
		t.Errorf("Failed to get deployment: %v\n", err)
		return false
	}

	// Update the deployment
	_, err = clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		t.Errorf("Failed to update deployment: %v\n", err)
		return false
	}

	fmt.Println("Deployment updated successfully!")
	time.Sleep(80 * time.Second)
	return true

}
func checkIfAnnotationExists(deploymentPods *v1.PodList, expectedAnnotations []string) bool {
	for _, pod := range deploymentPods.Items {
		for _, annotation := range expectedAnnotations {
			if pod.ObjectMeta.Annotations[annotation] != "true" {
				return false
			}
		}
	}
	fmt.Println("Annotations are correct!")
	return true
}
func updateAnnotationConfig(deployment *appsV1.Deployment, jsonStr string) {
	args := deployment.Spec.Template.Spec.Containers[0].Args
	indexOfAutoAnnotationConfigString := findIndexOfPrefix("--auto-annotation-config=", args)
	//if auto annotation not part of config, we will add it
	if indexOfAutoAnnotationConfigString < 0 {
		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, "--auto-annotation-config="+jsonStr)
		indexOfAutoAnnotationConfigString = len(deployment.Spec.Template.Spec.Containers[0].Args) - 1
	} else {
		deployment.Spec.Template.Spec.Containers[0].Args[indexOfAutoAnnotationConfigString] = "--auto-annotation-config=" + jsonStr
	}
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
	opMutex.Lock()
	defer opMutex.Unlock()
	deployment, err := clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting deployment: %v\n\n", err)
	}
	updateAnnotationConfig(deployment, jsonStr)
	if !updateOperator(t, clientSet, deployment) {
		t.Error("Failed to update Operator")
	}
}

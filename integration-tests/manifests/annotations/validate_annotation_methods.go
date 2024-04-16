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

// ko
func functionWithLock() {
	opMutex.Lock()
}

func unlockLock() {
	defer opMutex.Unlock()
}
func applyYAMLWithKubectl(filename, namespace string) error {
	cmd := exec.Command("kubectl", "apply", "-f", filename, "-n", namespace)
	return cmd.Run()
}

// Usage in your createNamespace function
func createNamespaceAndApplyResources(t *testing.T, clientset *kubernetes.Clientset, name string, resourceFiles []string) error {
	err := createNamespace(clientset, name)
	if err != nil {
		return err
	}

	time.Sleep(15 * time.Second)
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

func isNamespaceUpdated(clientset *kubernetes.Clientset, namespace string, startTime time.Time) bool {
	for {
		ns, err := clientset.CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{})
		if err != nil {
			fmt.Printf("Failed to get namespace %s: %v\n", namespace, err)
			return false
		}

		if ns.Status.Phase == v1.NamespaceActive {
			break
		}

	}
	//check if the namespace was updated
	ns, err := clientset.CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Failed to get namespace %s: %v\n", namespace, err)
		return false
	}
	if ns.CreationTimestamp.After(startTime) || ns.ResourceVersion != "" {
		return true
	}

	return false
}

func deleteYAMLWithKubectl(filename, namespace string) error {
	cmd := exec.Command("kubectl", "delete", "-f", filename, "-n", namespace)
	return cmd.Run()
}
func deleteNamespaceAndResources(clientset *kubernetes.Clientset, name string, resourceFiles []string) error {
	for _, file := range resourceFiles {
		err := deleteYAMLWithKubectl(filepath.Join("..", file), name)
		time.Sleep(5 * time.Second)

		if err != nil {
			return err
		}
	}

	//delete Namespace
	err := deleteNamespace(clientset, name)
	time.Sleep(15 * time.Second)
	return err
}
func createNamespace(clientset *kubernetes.Clientset, name string) error {
	_, err := clientset.CoreV1().Namespaces().Get(context.Background(), name, metav1.GetOptions{})
	if err == nil {
		return nil

	} else if !errors.IsNotFound(err) {
		return err
	}

	//create the namespace if it doesn't exist
	namespace := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	_, err = clientset.CoreV1().Namespaces().Create(context.Background(), namespace, metav1.CreateOptions{})
	time.Sleep(25 * time.Second)
	return err
}

func deleteNamespace(clientset *kubernetes.Clientset, name string) error {
	err := clientset.CoreV1().Namespaces().Delete(context.Background(), name, metav1.DeleteOptions{})
	return err
}

func checkNameSpaceAnnotations(ns *v1.Namespace, expectedAnnotations []string) bool {

	correct := true
	for i := 0; i < 10; i++ {

		for _, annotation := range expectedAnnotations {
			fmt.Printf("This is the annotation: %v and its status %v, namespace name: %v, ", annotation, ns.Status, ns.Name)
			if ns.ObjectMeta.Annotations[annotation] != "true" {
				time.Sleep(10 * time.Second)
				correct = false
				break
			}
		}
		if correct == true {
			fmt.Println("Namespace annotations are correct!")
			return true
		}
	}
	return false
}

func updateOperator(t *testing.T, clientSet *kubernetes.Clientset, deployment *appsV1.Deployment, startTime time.Time) bool {
	var err error
	args := deployment.Spec.Template.Spec.Containers[0].Args

	// Attempt to get the deployment by name
	deployment, err = clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	deployment.Spec.Template.Spec.Containers[0].Args = args

	// Update the deployment
	_, err = clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		t.Errorf("Failed to update deployment: %v\n", err)
		return false
	}
	err = waitForDeploymentAvailable(clientSet, amazonCloudwatchNamespace, amazonControllerManager, 60*time.Second)
	if err != nil {
		fmt.Println("There was an error trying to wait for deployment available", err)
		return false
	}
	fmt.Println("Deployment updated successfully!")
	// All checks passed, deployment update successful
	fmt.Println("Deployment fully updated.")
	return true
}

func waitForNewPodCreation(clientSet *kubernetes.Clientset, resource interface{}, startTime time.Time, timeout time.Duration) error {
	start := time.Now()

	for {
		if time.Since(start) > timeout {
			return fmt.Errorf("timed out waiting for new pod creation")
		}

		namespace := ""
		labelSelector := ""
		switch r := resource.(type) {
		case *appsV1.Deployment:
			namespace = r.Namespace
			labelSelector = labels.Set(r.Spec.Selector.MatchLabels).AsSelector().String()
		case *appsV1.DaemonSet:
			namespace = r.Namespace
			labelSelector = labels.Set(r.Spec.Selector.MatchLabels).AsSelector().String()
		case *appsV1.StatefulSet:
			namespace = r.Namespace
			labelSelector = labels.Set(r.Spec.Selector.MatchLabels).AsSelector().String()
		default:
			return fmt.Errorf("unsupported resource type")
		}

		newPods, err := clientSet.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			return fmt.Errorf("failed to list pods: %v", err)
		}

		//Check each pod for creation time and phase
		for _, pod := range newPods.Items {
			if pod.CreationTimestamp.Time.After(startTime) && pod.Status.Phase == v1.PodRunning {
				fmt.Printf("Operator pod %s created after start time and is running\n", pod.Name)
				return nil
			} else if pod.CreationTimestamp.Time.After(startTime) {
				fmt.Printf("Operator pod %s created after start time but is not in running stage\n", pod.Name)
			}
		}

		time.Sleep(5 * time.Second)
	}
}

func checkIfAnnotationExists(clientset *kubernetes.Clientset, pods *v1.PodList, expectedAnnotations []string, retryDuration time.Duration) bool {
	startTime := time.Now()

	for {
		if time.Since(startTime) > retryDuration*3 {
			fmt.Println("Timeout reached while waiting for annotations.")
			return false
		}

		// Update the list of pods on each iteration to ensure we have the latest information
		currentPods, err := clientset.CoreV1().Pods(pods.Items[0].Namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			fmt.Printf("Failed to list pods: %v\n", err)
			return false
		}

		// Check if all pods are in the Running phase
		allRunning := true
		for _, pod := range currentPods.Items {
			if pod.Status.Phase != v1.PodRunning {
				allRunning = false
				break
			}
		}
		if !allRunning {
			fmt.Println("Not all pods are in the Running phase. Retrying...")
			time.Sleep(5 * time.Second)
			continue
		}

		// Check annotations for each pod
		foundAllAnnotations := true
		for _, pod := range currentPods.Items {
			for _, annotation := range expectedAnnotations {
				fmt.Printf("Checking pod %s for annotation %s in namespace %s\n", pod.Name, annotation, pod.Namespace)

				// Check if the pod's annotations map contains the expected annotation
				if value, exists := pod.Annotations[annotation]; !exists || value != "true" {
					fmt.Printf("Pod %s does not have annotation %s with value 'true' in namespace %s\n", pod.Name, annotation, pod.Namespace)
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

		fmt.Println("Annotations not found in all pods or some pods are not in Running phase. Retrying...")
		time.Sleep(15 * time.Second) // Wait before retrying
	}
}
func waitForDeploymentAvailable(clientset *kubernetes.Clientset, namespace, deploymentName string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	watcher, err := clientset.AppsV1().Deployments(namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", deploymentName),
	})
	if err != nil {
		return err
	}
	defer watcher.Stop()

	for {
		select {
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("watch channel closed")
			}
			deployment, ok := event.Object.(*appsV1.Deployment)
			if !ok {
				return fmt.Errorf("unexpected object type: %T", event.Object)
			}
			if deployment.Status.AvailableReplicas == deployment.Status.Replicas {
				return nil
			}
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for deployment to be available")
		}
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
	deployment, err := clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting deployment: %v\n\n", err)
	}
	deployment = updateAnnotationConfig(deployment, jsonStr)

	if !updateOperator(t, clientSet, deployment, time.Now().Add(-time.Second)) {
		t.Error("Failed to update Operator", deployment, deployment.Name, deployment.Spec.Template.Spec.Containers[0].Args)
	}
	fmt.Println("Ended")

}

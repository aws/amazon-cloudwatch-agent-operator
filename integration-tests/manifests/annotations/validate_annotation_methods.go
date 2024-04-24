// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package annotations

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/labels"
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

const sampleDaemonsetYamlName = "sample-daemonset.yaml"
const sampleDeploymentYamlName = "sample-deployment.yaml"
const sampleStatefulsetYamlName = "sample-statefulset.yaml"

func applyYAMLWithKubectl(filename, namespace string) error {
	cmd := exec.Command("kubectl", "apply", "-f", filename, "-n", namespace)
	return cmd.Run()
}

func createNamespaceAndApplyResources(t *testing.T, clientset *kubernetes.Clientset, name string, resourceFiles []string) error {
	err := createNamespace(clientset, name)
	if err != nil {
		return err
	}

	for _, file := range resourceFiles {
		err = applyYAMLWithKubectl(filepath.Join("..", file), name)
		if err != nil {
			t.Error("Could not apply resources")
			return err
		}
	}
	return nil
}
func isNamespaceUpdated(clientset *kubernetes.Clientset, namespace string, startTime time.Time) bool {
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
		if err != nil {
			return err
		}
	}

	//delete Namespace
	err := deleteNamespace(clientset, name)
	return err
}

// Check if name space exist and if it does not we create the namespace and wait until it is fully created
func createNamespace(clientSet *kubernetes.Clientset, name string) error {
	_, err := clientSet.CoreV1().Namespaces().Get(context.Background(), name, metav1.GetOptions{})
	if err == nil {
		return nil
	} else if !errors.IsNotFound(err) {
		return err
	}

	namespace := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	_, err = clientSet.CoreV1().Namespaces().Create(context.Background(), namespace, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	timeout := 5 * time.Minute
	startTime := time.Now()
	for {
		if time.Since(startTime) > timeout {
			return fmt.Errorf("timeout reached while waiting for namespace %s to be created", name)
		}

		_, err := clientSet.CoreV1().Namespaces().Get(context.Background(), name, metav1.GetOptions{})
		if err == nil {
			return nil
		} else if !errors.IsNotFound(err) { //if any other error other than not found
			return err
		}

		time.Sleep(5 * time.Second)
	}
}

func deleteNamespace(clientset *kubernetes.Clientset, name string) error {
	err := clientset.CoreV1().Namespaces().Delete(context.Background(), name, metav1.DeleteOptions{})
	return err
}

// This function creates a namespace and checks it annotations and then deletes the namespace after check complete
func checkNameSpaceAnnotations(t *testing.T, clientSet *kubernetes.Clientset, expectedAnnotations []string, uniqueNamespace string, startTime time.Time) bool {

	timeOut := 5 * time.Minute
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

	for i := 0; i < 10; i++ {
		correct := true
		ns, err := clientSet.CoreV1().Namespaces().Get(context.TODO(), uniqueNamespace, metav1.GetOptions{})
		fmt.Printf("This is the loop iteration: %v\n, these are the annotation of ns %v", i, ns)
		if err != nil {
			fmt.Println("There was an error getting namespace, ", err)
			return false
		}
		for _, annotation := range expectedAnnotations {
			fmt.Printf("\n This is the annotation: %v and its status %v, namespace name: %v, \n", annotation, ns.Status, ns.Name)
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

// This function updates the operator and waits until it is ready
func updateOperator(t *testing.T, clientSet *kubernetes.Clientset, deployment *appsV1.Deployment, startTime time.Time) bool {
	var err error

	args := deployment.Spec.Template.Spec.Containers[0].Args

	deployment, err = clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	deployment.Spec.Template.Spec.Containers[0].Args = args

	_, err = clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		t.Errorf("Failed to update deployment: %v\n", err)
		return false
	}
	err = waitForNewPodCreation(clientSet, deployment, startTime, 60*time.Second)
	if err != nil {
		fmt.Println("There was an error trying to wait for deployment available", err)
		return false
	}
	fmt.Println("Deployment updated successfully!")
	return true
}

// Takes in a resource either Deployment, Daemonset, or Stateful set and
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

		newPods, _ := clientSet.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})

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

// check if the given pods have the expected annotations
func checkIfAnnotationExists(clientset *kubernetes.Clientset, pods *v1.PodList, expectedAnnotations []string, retryDuration time.Duration) bool {
	startTime := time.Now()
	for {
		if time.Since(startTime) > retryDuration {
			fmt.Println("Timeout reached while waiting for annotations.")
			return false
		}

		//This exist to check if any pods took too long to delete and we need to list pods again
		currentPods, err := clientset.CoreV1().Pods(pods.Items[0].Namespace).List(context.TODO(), metav1.ListOptions{})
		fmt.Println("Current pods len: ", len(currentPods.Items))
		if err != nil {
			fmt.Printf("Failed to list pods: %v\n", err)
			return false
		}

		//check if all pods are in the Running phase
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

		foundAllAnnotations := true
		for _, pod := range currentPods.Items {
			for _, annotation := range expectedAnnotations {
				fmt.Printf("Checking pod %s for annotation %s in namespace %s\n", pod.Name, annotation, pod.Namespace)

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
		time.Sleep(5 * time.Second)
	}
}

// Finds auto-annotation arg in operator and updates it, if not found it will be added to the end
func updateAnnotationConfig(deployment *appsV1.Deployment, jsonStr string) *appsV1.Deployment {

	args := deployment.Spec.Template.Spec.Containers[0].Args
	indexOfAutoAnnotationConfigString := findIndexOfPrefix("--auto-annotation-config=", args)
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

// kubernetes setup
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
}

func checkResourceAnnotations(t *testing.T, clientSet *kubernetes.Clientset, resourceType, uniqueNamespace, resourceName string, sampleAppYamlName string, startTime time.Time, annotations []string) error {
	if err := createNamespaceAndApplyResources(t, clientSet, uniqueNamespace, []string{sampleAppYamlName}); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
		return err
	}

	defer func() {
		if err := deleteNamespaceAndResources(clientSet, uniqueNamespace, []string{sampleAppYamlName}); err != nil {
			t.Fatalf("Failed to delete namespaces/resources: %v", err)
		}
	}()
	var resource interface{}

	switch resourceType {
	case "deployment":
		// Get deployment
		deployment, err := clientSet.AppsV1().Deployments(uniqueNamespace).Get(context.TODO(), resourceName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get deployment: %s", err.Error())
		}
		resource = deployment
	case "daemonset":
		// Get daemonset
		daemonset, err := clientSet.AppsV1().DaemonSets(uniqueNamespace).Get(context.TODO(), resourceName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get daemonset: %s", err.Error())
		}
		resource = daemonset
	case "statefulset":
		// Get statefulset
		statefulset, err := clientSet.AppsV1().StatefulSets(uniqueNamespace).Get(context.TODO(), resourceName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get statefulset: %s", err.Error())
		}
		resource = statefulset
	default:
		return fmt.Errorf("unsupported resource type: %s", resourceType)
	}

	// Wait for new pod creation
	err := waitForNewPodCreation(clientSet, resource, startTime, 60*time.Second)
	if err != nil {
		return fmt.Errorf("error waiting for pod creation: %s", err.Error())
	}

	// List resource pods
	resourcePods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{})

	if err != nil {
		return fmt.Errorf("failed to list pods: %s", err.Error())
	}

	// Wait for pods to update
	if !checkIfAnnotationExists(clientSet, resourcePods, annotations, 60*time.Second) {
		return fmt.Errorf("missing annotations: %v", annotations)
	}

	return nil
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
package annotations

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/auto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"path/filepath"
	"testing"
	"time"
)

// ---------------------------USE CASE 4 (Python and Java on DaemonSet)------------------------------
func TestJavaAndPythonDaemonSet(t *testing.T) {

	t.Parallel()
	clientSet := setupTest(t)
	uniqueNamespace := "daemonset-namespace-java-python"
	if err := createNamespaceAndApplyResources(t, clientSet, uniqueNamespace, []string{"sample-daemonset.yaml"}); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespaceAndResources(clientSet, uniqueNamespace, []string{"sample-daemonset.yaml"}); err != nil {
			t.Fatalf("Failed to delete namespaces/resources: %v", err)
		}
	}()

	annotationConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{filepath.Join(uniqueNamespace, daemonSetName)},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
		Python: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{filepath.Join(uniqueNamespace, daemonSetName)},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
	}
	jsonStr, err := json.Marshal(annotationConfig)
	if err != nil {
		t.Error("Error:", err)
	}

	updateTheOperator(t, clientSet, string(jsonStr))

	// Get the fluent-bit DaemonSet
	daemonSet, err := clientSet.AppsV1().DaemonSets(uniqueNamespace).Get(context.TODO(), daemonSetName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get fluent-bit daemonset: %s", err.Error())
	}

	// List pods belonging to the fluent-bit DaemonSet
	set := labels.Set(daemonSet.Spec.Selector.MatchLabels)

	for {
		daemonPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: set.AsSelector().String(),
		})
		if err != nil {
			panic(err.Error())
		}

		// Check if any pod is in the updating stage
		if !podsInUpdatingStage(daemonPods.Items) {
			break // Exit loop if no pods are updating
		}

		// Sleep for a short duration before checking again
		time.Sleep(10 * time.Second)
	}
	daemonPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{})

	if err != nil {
		t.Errorf("Error listing pods for fluent-bit daemonset: %s", err.Error())
	}
	if !checkIfAnnotationExists(clientSet, daemonPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation, injectPythonAnnotation, autoAnnotatePythonAnnotation}, 60*time.Second) {
		fmt.Printf("Missing annotation for daemonset name %v, unique namespace %v, actual namespace %v whose annotation config is : %v", daemonSet.Name, uniqueNamespace, daemonSet.Namespace, daemonSet.Spec.Template.Spec.Containers[0].Args)
		t.Error("Missing Java and Python annotations")
	}

}

// ---------------------------USE CASE 5 (Java on DaemonSet and Python should be removed)------------------------------
func TestJavaOnlyDaemonSet(t *testing.T) {

	t.Parallel()
	clientSet := setupTest(t)
	uniqueNamespace := "daemonset-namespace-java-only"
	if err := createNamespaceAndApplyResources(t, clientSet, uniqueNamespace, []string{"sample-daemonset.yaml"}); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespaceAndResources(clientSet, uniqueNamespace, []string{"sample-daemonset.yaml"}); err != nil {
			t.Fatalf("Failed to delete namespaces/resources: %v", err)
		}
	}()

	annotationConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{filepath.Join(uniqueNamespace, daemonSetName)},
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
		t.Error("Error: ", err)
	}

	updateTheOperator(t, clientSet, string(jsonStr))

	// Get the fluent-bit DaemonSet
	daemonSet, err := clientSet.AppsV1().DaemonSets(uniqueNamespace).Get(context.TODO(), daemonSetName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get fluent-bit daemonset: %s", err.Error())
	}

	// List pods belonging to the fluent-bit DaemonSet
	set := labels.Set(daemonSet.Spec.Selector.MatchLabels)
	daemonPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})

	for {
		daemonPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: set.AsSelector().String(),
		})
		if err != nil {
			panic(err.Error())
		}

		// Check if any pod is in the updating stage
		if !podsInUpdatingStage(daemonPods.Items) {
			break // Exit loop if no pods are updating
		}

		// Sleep for a short duration before checking again
		time.Sleep(10 * time.Second)
	}

	if err != nil {
		t.Errorf("Error listing pods for fluent-bit daemonset: %s", err.Error())
	}
	//Python should not exist on pods
	//Python should have been removed

	if !checkIfAnnotationExists(clientSet, daemonPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}, 60*time.Second) {
		fmt.Printf("Missing annotation for daemonset name %v, unique namespace %v, actual namespace %v whose annotation config is : %v", daemonSet.Name, uniqueNamespace, daemonSet.Namespace, daemonSet.Spec.Template.Spec.Containers[0].Args)
		t.Error("Missing Java annotations")
	}

}

// ---------------------------USE CASE 6 (Python on DaemonSet Java annotation should be removed)------------------------------
func TestPythonOnlyDaemonSet(t *testing.T) {

	t.Parallel()
	clientSet := setupTest(t)
	uniqueNamespace := "daemonset-namespace-python-only"
	if err := createNamespaceAndApplyResources(t, clientSet, uniqueNamespace, []string{"sample-daemonset.yaml"}); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespaceAndResources(clientSet, uniqueNamespace, []string{"sample-daemonset.yaml"}); err != nil {
			t.Fatalf("Failed to delete namespaces/resources: %v", err)
		}
	}()
	//updating operator deployment

	annotationConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
		Python: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{filepath.Join(uniqueNamespace, daemonSetName)},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
	}
	jsonStr, err := json.Marshal(annotationConfig)
	if err != nil {
		t.Error("Error:", err)
	}
	updateTheOperator(t, clientSet, string(jsonStr))

	// Get the fluent-bit DaemonSet
	daemonSet, err := clientSet.AppsV1().DaemonSets(uniqueNamespace).Get(context.TODO(), daemonSetName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get fluent-bit daemonset: %s", err.Error())
	}

	// List pods belonging to the fluent-bit DaemonSet
	set := labels.Set(daemonSet.Spec.Selector.MatchLabels)

	for {
		daemonPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: set.AsSelector().String(),
		})
		if err != nil {
			panic(err.Error())
		}

		// Check if any pod is in the updating stage
		if !podsInUpdatingStage(daemonPods.Items) {
			break // Exit loop if no pods are updating
		}

		// Sleep for a short duration before checking again
		time.Sleep(10 * time.Second)
	}

	daemonPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})

	if err != nil {
		t.Errorf("Error listing pods for fluent-bit daemonset: %s", err.Error())
	}
	//java annotations should be removed

	//java shouldn't be annotated in this case

	if !checkIfAnnotationExists(clientSet, daemonPods, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}, 60*time.Second) {
		fmt.Printf("Missing annotation for daemonset name %v, unique namespace %v, actual namespace %v whose annotation config is : %v", daemonSet.Name, uniqueNamespace, daemonSet.Namespace, daemonSet.Spec.Template.Spec.Containers[0].Args)

		t.Error("Missing Python annotations")
	}

}

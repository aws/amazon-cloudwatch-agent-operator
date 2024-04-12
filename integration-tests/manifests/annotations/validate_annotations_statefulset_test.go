// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
package annotations

import (
	"context"
	"encoding/json"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/auto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"path/filepath"
	"testing"
	"time"
)

// ---------------------------USE CASE 10 (Python and Java on Stateful set)------------------------------
func TestJavaAndPythonStatefulSet(t *testing.T) {

	t.Parallel()
	clientSet := setupTest(t)
	uniqueNamespace := "statefulset-namespace-java-python"
	if err := createNamespaceAndApplyResources(t, clientSet, uniqueNamespace, []string{"sample-statefulset.yaml"}); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespaceAndResources(clientSet, uniqueNamespace, []string{"sample-statefulset.yaml"}); err != nil {
			t.Fatalf("Failed to delete namespaces/resources: %v", err)
		}
	}()

	annotationConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{filepath.Join(uniqueNamespace, statefulSetName)},
		},
		Python: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{filepath.Join(uniqueNamespace, statefulSetName)},
		},
	}
	jsonStr, err := json.Marshal(annotationConfig)
	if err != nil {
		t.Error("Error:", err)
	}
	updateTheOperator(t, clientSet, string(jsonStr))
	// Get the StatefulSet
	statefulSet, err := clientSet.AppsV1().StatefulSets(uniqueNamespace).Get(context.TODO(), statefulSetName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get my-statefulset StatefulSet: %s\n", err.Error())
	}

	// List pods belonging to the StatefulSet
	set := labels.Set(statefulSet.Spec.Selector.MatchLabels)

	for {
		statefulSetPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: set.AsSelector().String(),
		})
		if err != nil {
			panic(err.Error())
		}

		// Check if any pod is in the updating stage
		if !podsInUpdatingStage(statefulSetPods.Items) {
			break // Exit loop if no pods are updating
		}

		// Sleep for a short duration before checking again
		time.Sleep(10 * time.Second)
	}
	statefulSetPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})
	if err != nil {
		t.Errorf("Error listing pods for my-statefulset StatefulSet: %s\n", err.Error())
	}
	if !checkIfAnnotationExists(clientSet, statefulSetPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation, injectPythonAnnotation, autoAnnotatePythonAnnotation}, 60*time.Second) {
		t.Error("Missing Java and Python annotations")
	}

}

// ---------------------------USE CASE 11 (Java on Stateful set and Python should be removed)------------------------------
func TestJavaOnlyStatefulSet(t *testing.T) {

	t.Parallel()
	clientSet := setupTest(t)
	uniqueNamespace := "statefulset-java-only"
	if err := createNamespaceAndApplyResources(t, clientSet, uniqueNamespace, []string{"sample-statefulset.yaml"}); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespaceAndResources(clientSet, uniqueNamespace, []string{"sample-statefulset.yaml"}); err != nil {
			t.Fatalf("Failed to delete namespaces/resources: %v", err)
		}
	}()

	annotationConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{filepath.Join(uniqueNamespace, statefulSetName)},
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

	// Get the StatefulSet
	statefulSet, err := clientSet.AppsV1().StatefulSets(uniqueNamespace).Get(context.TODO(), statefulSetName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get my-statefulset StatefulSet: %s\n", err.Error())
	}

	// List pods belonging to the StatefulSet
	set := labels.Set(statefulSet.Spec.Selector.MatchLabels)

	if err != nil {
		t.Errorf("Error listing pods for my-statefulset StatefulSet: %s\n", err.Error())
	}

	for {
		statefulSetPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: set.AsSelector().String(),
		})
		if err != nil {
			panic(err.Error())
		}

		// Check if any pod is in the updating stage
		if !podsInUpdatingStage(statefulSetPods.Items) {
			break // Exit loop if no pods are updating
		}

		// Sleep for a short duration before checking again
		time.Sleep(10 * time.Second)
	}
	statefulSetPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})
	if !checkIfAnnotationExists(clientSet, statefulSetPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}, 60*time.Second) {
		t.Error("Missing Java annotations")
	}
}

// ---------------------------USE CASE 12 (Python on Stateful set and java should be removed)------------------------------
func TestPythonOnlyStatefulSet(t *testing.T) {

	t.Parallel()
	clientSet := setupTest(t)
	uniqueNamespace := "statefulset-namespace-python-only"
	if err := createNamespaceAndApplyResources(t, clientSet, uniqueNamespace, []string{"sample-statefulset.yaml"}); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespaceAndResources(clientSet, uniqueNamespace, []string{"sample-statefulset.yaml"}); err != nil {
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
			Deployments:  []string{""},
			StatefulSets: []string{filepath.Join(uniqueNamespace, statefulSetName)},
		},
	}
	jsonStr, err := json.Marshal(annotationConfig)
	if err != nil {
		t.Error("Error:", err)
	}
	updateTheOperator(t, clientSet, string(jsonStr))

	// Get the StatefulSet
	statefulSet, err := clientSet.AppsV1().StatefulSets(uniqueNamespace).Get(context.TODO(), statefulSetName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get StatefulSet: %s\n", err.Error())
	}

	// List pods belonging to the StatefulSet
	set := labels.Set(statefulSet.Spec.Selector.MatchLabels)

	for {
		statefulSetPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: set.AsSelector().String(),
		})
		if err != nil {
			panic(err.Error())
		}

		// Check if any pod is in the updating stage
		if !podsInUpdatingStage(statefulSetPods.Items) {
			break // Exit loop if no pods are updating
		}

		// Sleep for a short duration before checking again
		time.Sleep(10 * time.Second)
	}
	statefulSetPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})

	if err != nil {
		t.Errorf("Error listing pods for StatefulSet: %s\n", err.Error())
	}

	if !checkIfAnnotationExists(clientSet, statefulSetPods, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}, 60*time.Second) {
		t.Error("Missing Python annotations")
	}
}

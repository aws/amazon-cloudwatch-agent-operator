// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
package annotations

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/auto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math/big"
	"path/filepath"
	"testing"
	"time"
)

func TestJavaAndPythonStatefulSet(t *testing.T) {

	clientSet := setupTest(t)
	randomNumber, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		panic(err)
	}
	randomNumber.Add(randomNumber, big.NewInt(1000)) //adding a hash to namespace
	uniqueNamespace := fmt.Sprintf("statefulset-namespace-java-python-%d", randomNumber)
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
	startTime := time.Now()
	updateTheOperator(t, clientSet, string(jsonStr))
	if err := createNamespaceAndApplyResources(t, clientSet, uniqueNamespace, []string{"sample-statefulset.yaml"}); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}
	defer func() {
		if err := deleteNamespaceAndResources(clientSet, uniqueNamespace, []string{"sample-statefulset.yaml"}); err != nil {
			t.Fatalf("Failed to delete namespaces/resources: %v", err)
		}
	}()

	// Get the fluent-bit DaemonSet
	statefulSet, err := clientSet.AppsV1().StatefulSets(uniqueNamespace).Get(context.TODO(), statefulSetName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get fluent-bit daemonset: %s", err.Error())
	}

	err = waitForNewPodCreation(clientSet, statefulSet, startTime, 60*time.Second)

	fmt.Println("All pods have completed updating.")
	statefulSetPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		t.Errorf("Error listing pods for my-statefulset StatefulSet: %s\n", err.Error())
	}
	if !checkIfAnnotationExists(clientSet, statefulSetPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation, injectPythonAnnotation, autoAnnotatePythonAnnotation}, 60*time.Second) {
		t.Error("Missing Java and Python annotations")
	}

}

func TestJavaOnlyStatefulSet(t *testing.T) {

	clientSet := setupTest(t)
	randomNumber, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		panic(err)
	}
	randomNumber.Add(randomNumber, big.NewInt(1000)) //adding a hash to namespace
	uniqueNamespace := fmt.Sprintf("statefulset-java-only-%d", randomNumber)

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
	startTime := time.Now()
	updateTheOperator(t, clientSet, string(jsonStr))
	if err := createNamespaceAndApplyResources(t, clientSet, uniqueNamespace, []string{"sample-statefulset.yaml"}); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespaceAndResources(clientSet, uniqueNamespace, []string{"sample-statefulset.yaml"}); err != nil {
			t.Fatalf("Failed to delete namespaces/resources: %v", err)
		}
	}()

	// Get the fluent-bit DaemonSet
	statefulSet, err := clientSet.AppsV1().StatefulSets(uniqueNamespace).Get(context.TODO(), statefulSetName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get fluent-bit daemonset: %s", err.Error())
	}

	err = waitForNewPodCreation(clientSet, statefulSet, startTime, 60*time.Second)

	fmt.Println("All pods have completed updating.")
	statefulSetPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{})

	if !checkIfAnnotationExists(clientSet, statefulSetPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}, 60*time.Second) {
		t.Error("Missing Java annotations")
	}
}

func TestPythonOnlyStatefulSet(t *testing.T) {

	clientSet := setupTest(t)
	randomNumber, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		panic(err)
	}
	randomNumber.Add(randomNumber, big.NewInt(1000)) //adding a hash to namespace
	uniqueNamespace := fmt.Sprintf("statefulset-namespace-python-only-%d", randomNumber)
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

	startTime := time.Now()
	updateTheOperator(t, clientSet, string(jsonStr))
	if err := createNamespaceAndApplyResources(t, clientSet, uniqueNamespace, []string{"sample-statefulset.yaml"}); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespaceAndResources(clientSet, uniqueNamespace, []string{"sample-statefulset.yaml"}); err != nil {
			t.Fatalf("Failed to delete namespaces/resources: %v", err)
		}
	}()

	// Get the fluent-bit DaemonSet
	statefulSet, err := clientSet.AppsV1().StatefulSets(uniqueNamespace).Get(context.TODO(), statefulSetName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get fluent-bit daemonset: %s", err.Error())
	}

	// List pods belonging to the fluent-bit DaemonSet
	err = waitForNewPodCreation(clientSet, statefulSet, startTime, 60*time.Second)

	fmt.Println("All pods have completed updating.")
	statefulSetPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{})

	if err != nil {
		t.Errorf("Error listing pods for StatefulSet: %s\n", err.Error())
	}

	if !checkIfAnnotationExists(clientSet, statefulSetPods, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}, 60*time.Second) {
		t.Error("Missing Python annotations")
	}
}

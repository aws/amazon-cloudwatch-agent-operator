// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
package annotations

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent-operator/integration-tests/util"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/auto"
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestJavaAndPythonNamespace(t *testing.T) {

	clientSet := setupTest(t)
	randomNumber, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		panic(err)
	}
	randomNumber.Add(randomNumber, big.NewInt(1000)) //adding a hash to namespace
	uniqueNamespace := fmt.Sprintf("namespace-java-python-%d", randomNumber)

	annotationConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{uniqueNamespace},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
		Python: auto.AnnotationResources{
			Namespaces:   []string{uniqueNamespace},
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
	if !checkNameSpaceAnnotations(t, clientSet, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation, injectPythonAnnotation, autoAnnotatePythonAnnotation}, uniqueNamespace, startTime) {
		t.Error("Missing java and python annotations")
	}
}

func TestJavaOnlyNamespace(t *testing.T) {
	clientSet := setupTest(t)
	randomNumber, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		panic(err)
	}
	randomNumber.Add(randomNumber, big.NewInt(1000)) //adding a hash to namespace
	uniqueNamespace := fmt.Sprintf("namespace-java-only-%d", randomNumber)
	annotationConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{uniqueNamespace},
			DaemonSets:   []string{""},
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
		t.Error("Error:", err)
	}
	startTime := time.Now()
	updateTheOperator(t, clientSet, string(jsonStr))
	if !checkNameSpaceAnnotations(t, clientSet, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}, uniqueNamespace, startTime) {
		t.Error("Missing Java annotations")
	}
}

func TestPythonOnlyNamespace(t *testing.T) {

	clientSet := setupTest(t)
	randomNumber, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		panic(err)
	}
	randomNumber.Add(randomNumber, big.NewInt(1000)) //adding a hash to namespace
	uniqueNamespace := fmt.Sprintf("namespace-python-only-%d", randomNumber)
	if err := createNamespace(clientSet, uniqueNamespace); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespace(clientSet, uniqueNamespace); err != nil {
			t.Fatalf("Failed to delete namespace: %v", err)
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
			Namespaces:   []string{uniqueNamespace},
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

	if !checkNameSpaceAnnotations(t, clientSet, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}, uniqueNamespace, startTime) {
		t.Error("Missing Python annotations")
	}
}

// Multiple resources on the same namespace should all get annotations
func TestAnnotationsOnMultipleResources(t *testing.T) {

	clientSet := setupTest(t)
	newUUID := uuid.New()
	uniqueNamespace := fmt.Sprintf("multiple-resources-%s", uuid.NewString())

	annotationConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			DaemonSets:   []string{filepath.Join(uniqueNamespace, daemonSetName)},
			Deployments:  []string{filepath.Join(uniqueNamespace, deploymentName)},
			StatefulSets: []string{filepath.Join(uniqueNamespace, statefulSetName)},
		},
		Python: auto.AnnotationResources{},
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

	if err := checkResourceAnnotations(t, clientSet, "deployment", uniqueNamespace, deploymentName, sampleDeploymentYamlNameRelPath, startTime, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}, true); err != nil {
		t.Fatalf("Failed annotation check: %s", err.Error())
	}
	if err := checkResourceAnnotations(t, clientSet, "daemonset", uniqueNamespace, daemonSetName, sampleDaemonsetYamlRelPath, startTime, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}, true); err != nil {
		t.Fatalf("Failed annotation check: %s", err.Error())
	}
	if err := checkResourceAnnotations(t, clientSet, "statefulset", uniqueNamespace, statefulSetName, sampleStatefulsetYamlNameRelPath, startTime, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}, false); err != nil {
		t.Fatalf("Failed annotation check: %s", err.Error())
	}

}

// This tests a resource that is auto annotated is manually patched to remove the annotations, our mutator adds back the annotations
func TestAutoAnnotationForManualAnnotationRemoval(t *testing.T) {

	clientSet, uniqueNamespace := setupFunction(t, "manual-annotation-removal", []string{sampleDeploymentYamlNameRelPath})
	t.Cleanup(func() {
		if err := deleteNamespaceAndResources(clientSet, uniqueNamespace, []string{sampleDeploymentYamlNameRelPath}); err != nil {
			t.Fatalf("Failed to delete namespaces/resources: %v", err)
		}
	})

	annotationConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Deployments: []string{filepath.Join(uniqueNamespace, deploymentName)},
		},
		Python: auto.AnnotationResources{},
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

	deployment, err := clientSet.AppsV1().Deployments(uniqueNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Error retrieving deployment: %v\n", err)
		os.Exit(1)
	}

	//Removing all annotations
	deployment.ObjectMeta.Annotations = nil
	deployment, err = clientSet.AppsV1().Deployments(uniqueNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	_, err = clientSet.AppsV1().Deployments(uniqueNamespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		fmt.Printf("Error updating deployment: %v\n", err)
		os.Exit(1)
	}

	err = util.WaitForNewPodCreation(clientSet, deployment, startTime)
	if err != nil {
		fmt.Printf("Error waiting for pod creation: %v\n", err)
		os.Exit(1)
	}

	deploymentPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error listing pods: %v\n", err)
		os.Exit(1)
	}
	//Check if operator has added back the annotations
	checkIfAnnotationExists(clientSet, deploymentPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation})

}

// Creating two apps - First app is annotated
// Second app is not annotated but on the same namespace as the first app
// Annotate the namespace of the apps and make sure only the non annotated app was restarted
// Also tests if a resource is manually annotated and now its namespace is added for auto annotation
// the resource should not be modified and should not be restarted (auto-annotation annotation does not exist)
func TestOnlyNonAnnotatedAppsShouldBeRestarted(t *testing.T) {

	clientSet, uniqueNamespace := setupFunction(t, "non-annotated", []string{sampleDeploymentYamlNameRelPath, sampleNginxAppYamlNameRelPath})
	t.Cleanup(func() {
		if err := deleteNamespaceAndResources(clientSet, uniqueNamespace, []string{sampleDeploymentYamlNameRelPath, sampleNginxAppYamlNameRelPath}); err != nil {
			t.Fatalf("Failed to delete namespaces/resources: %v", err)
		}
	})
	startTime := time.Now()
	annotationConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces: []string{uniqueNamespace},
		},
		Python: auto.AnnotationResources{},
	}
	jsonStr, err := json.Marshal(annotationConfig)
	if err != nil {
		t.Error("Error:", err)
	}

	updateTheOperator(t, clientSet, string(jsonStr))
	if err != nil {
		t.Errorf("Failed to get deployment app: %s", err.Error())
	}
	deployment, err := clientSet.AppsV1().Deployments(uniqueNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Error retrieving deployment: %v\n", err)
		os.Exit(1)
	}
	nginxDeployment, err := clientSet.AppsV1().Deployments(uniqueNamespace).Get(context.TODO(), nginxDeploymentName, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Error retrieving deployment: %v\n", err)
		os.Exit(1)
	}
	err = util.WaitForNewPodCreation(clientSet, deployment, startTime)
	if err != nil {
		t.Fatal("Error waiting for pod creation: ", err)
	}

	if annotationExists(nginxDeployment.Annotations, autoAnnotateJavaAnnotation) {
		t.Fatal("Auto-annotation annotation should not exist")
	}

	numOfRevisions := numberOfRevisions(nginxDeploymentName, uniqueNamespace)
	if numOfRevisions > 1 {
		t.Fatal("Nginx was restarted") //should not be restarted since it already had annotations
	}
	numOfRevisions = numberOfRevisions(deploymentName, uniqueNamespace)
	if numOfRevisions != 2 {
		t.Fatal("Sample-deployment should have been restarted") //should not be restarted since it already had annotations
	}

}

// Test if a resource is auto annotated and now its namespace is added for auto annotation
// the resource should not be restarted
func TestAlreadyAutoAnnotatedResourceShouldNotRestart(t *testing.T) {

	clientSet, uniqueNamespace := setupFunction(t, "already-annotated", []string{sampleDeploymentYamlNameRelPath})
	t.Cleanup(func() {
		if err := deleteNamespaceAndResources(clientSet, uniqueNamespace, []string{sampleDeploymentYamlNameRelPath}); err != nil {
			t.Fatalf("Failed to delete namespaces/resources: %v", err)
		}
	})
	startTime := time.Now()
	annotationConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Deployments: []string{filepath.Join(uniqueNamespace, deploymentName)},
		},
		Python: auto.AnnotationResources{},
	}
	jsonStr, err := json.Marshal(annotationConfig)
	if err != nil {
		t.Error("Error:", err)
	}

	updateTheOperator(t, clientSet, string(jsonStr))
	if err != nil {
		t.Errorf("Failed to get deployment app: %s", err.Error())
	}
	deployment, err := clientSet.AppsV1().Deployments(uniqueNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Error retrieving deployment: %v\n", err)
		os.Exit(1)
	}

	err = util.WaitForNewPodCreation(clientSet, deployment, startTime)
	if err != nil {
		t.Fatalf("Error waiting for pod creation: %v\n", err)
	}
	fmt.Println("Done checking deployment")
	//adding deployment's namespace to get auto annotated
	annotationConfig = auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:  []string{uniqueNamespace},
			Deployments: []string{filepath.Join(uniqueNamespace, deploymentName)},
		},
		Python: auto.AnnotationResources{},
	}
	jsonStr, err = json.Marshal(annotationConfig)
	if err != nil {
		t.Error("Error:", err)
	}

	fmt.Println("Right before update operator", startTime)
	updateTheOperator(t, clientSet, string(jsonStr))
	fmt.Println("Right after update operator", startTime)

	//number of revisions should not be greater than 2
	//first one is for creation second one is for the first operator change and third one should not exist (even with the second operator change)
	numOfRevisions := numberOfRevisions(deploymentName, uniqueNamespace)
	if numOfRevisions > 2 {
		t.Fatal("Sample-deployment should not have been restarted after second operator update") //should not be restarted since it already had annotations
	}

}

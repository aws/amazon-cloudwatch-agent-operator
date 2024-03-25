package annotations

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/auto"
	"github.com/stretchr/testify/assert"
	appsV1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
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

const amazonControllerManager = "amazon-cloudwatch-observability-controller-manager"

var opMutex sync.Mutex

// ---------------------------USE CASE 1 (Java and Python on Deployment) ----------------------------------------------
func TestUseCase1(t *testing.T) {

	t.Parallel()
	clientSet := setupTest(t)
	uniqueNamespace := "sample-namespace-1"
	if err := createNamespaceAndApplyResources(t, clientSet, uniqueNamespace, []string{"sample-deployment.yaml"}); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespaceAndResources(clientSet, uniqueNamespace, []string{"sample-deployment.yaml"}); err != nil {
			t.Fatalf("Failed to delete namespaces/resources: %v", err)
		}
	}()
	//updating operator deployment
	deployment, err := clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting deployment: %v\n\n", err)
	}

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
	deployment, err = clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting deployment: %v\n\n", err)
	}
	updateAnnotationConfig(deployment, string(jsonStr))
	if !updateOperator(t, clientSet, deployment) {
		t.Error("Failed to update Operator")
	}

	//check if deployment has annotations.
	deployment, err = clientSet.AppsV1().Deployments(uniqueNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
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
		t.Error("Incorrect Annotations")
	}

}

// ---------------------------USE CASE 2 (Java on Deployment and Python Should be Removed)------------------------------
func TestUseCase2(t *testing.T) {

	t.Parallel()
	clientSet := setupTest(t)
	uniqueNamespace := "sample-namespace-2"
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

	deployment, err := clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting deployment: %v\n", err)
	}

	//finding where index of --auto-annotation-config= is (if it doesn't exist it will be appended)
	updateAnnotationConfig(deployment, string(jsonStr))
	if !updateOperator(t, clientSet, deployment) {
		t.Error("Failed to update Operator")
	}

	//check if deployment has annotations.
	deployment, err = clientSet.AppsV1().Deployments(uniqueNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
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
		t.Error("Incorrect Annotations")

	}
	//wait for pods to update
	if !checkIfAnnotationExists(deploymentPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}) {
		t.Error("Incorrect Annotations")
	}

}

// ---------------------------USE CASE 3 (Python on Deployment and java annotations should be removed) ----------------------------------------------
func TestUseCase3(t *testing.T) {

	t.Parallel()
	clientSet := setupTest(t)
	uniqueNamespace := "sample-namespace-3"
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
	deployment, err := clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting deployment: %v\n\n", err)
	}
	updateAnnotationConfig(deployment, string(jsonStr))
	if !updateOperator(t, clientSet, deployment) {
		t.Error("Failed to update Operator")
	}

	//check if deployment has annotations.
	deployment, err = clientSet.AppsV1().Deployments(uniqueNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
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
		t.Error("Incorrect Annotations")

	}
	//wait for pods to update
	if !checkIfAnnotationExists(deploymentPods, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		t.Error("Incorrect Annotations")
	}

}

// ---------------------------USE CASE 4 (Python and Java on DaemonSet)------------------------------
func TestUseCase4(t *testing.T) {

	t.Parallel()
	clientSet := setupTest(t)
	uniqueNamespace := "sample-namespace-4"
	if err := createNamespaceAndApplyResources(t, clientSet, uniqueNamespace, []string{"sample-daemonset.yaml"}); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespaceAndResources(clientSet, uniqueNamespace, []string{"sample-daemonset.yaml"}); err != nil {
			t.Fatalf("Failed to delete namespaces/resources: %v", err)
		}
	}()
	//updating operator deployment
	deployment, err := clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting deployment: %v\n\n", err)
	}

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

	deployment, err = clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting deployment: %v\n\n", err)
	}
	updateAnnotationConfig(deployment, string(jsonStr))
	if !updateOperator(t, clientSet, deployment) {
		t.Error("Failed to update Operator")
	}

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

	if err != nil {
		t.Errorf("Error listing pods for fluent-bit daemonset: %s", err.Error())
	}
	if !checkIfAnnotationExists(daemonPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation, injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		t.Error("Incorrect Annotations")
	}

}

// ---------------------------USE CASE 5 (Java on DaemonSet and Python should be removed)------------------------------
func TestUseCase5(t *testing.T) {

	t.Parallel()
	clientSet := setupTest(t)
	uniqueNamespace := "sample-namespace-5"
	if err := createNamespaceAndApplyResources(t, clientSet, uniqueNamespace, []string{"sample-daemonset.yaml"}); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespaceAndResources(clientSet, uniqueNamespace, []string{"sample-daemonset.yaml"}); err != nil {
			t.Fatalf("Failed to delete namespaces/resources: %v", err)
		}
	}()
	//updating operator deployment
	deployment, err := clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting deployment: %v\n", err)
	}

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
		t.Error("Error:", err)
	}
	deployment, err = clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting deployment: %v\n", err)
	}
	updateAnnotationConfig(deployment, string(jsonStr))
	if !updateOperator(t, clientSet, deployment) {
		t.Error("Failed to update Operator")
	}

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
	if err != nil {
		t.Errorf("Error listing pods for fluent-bit daemonset: %s", err.Error())
	}
	//Python should not exist on pods
	//Python should have been removed
	if checkIfAnnotationExists(daemonPods, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		t.Error("Incorrect Annotations")
	}
	if !checkIfAnnotationExists(daemonPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}) {
		t.Error("Incorrect Annotations")
	}

}

// ---------------------------USE CASE 6 (Python on DaemonSet Java annotation should be removed)------------------------------
func TestUseCase6(t *testing.T) {

	t.Parallel()
	clientSet := setupTest(t)
	uniqueNamespace := "sample-namespace-6"
	if err := createNamespaceAndApplyResources(t, clientSet, uniqueNamespace, []string{"sample-daemonset.yaml"}); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespaceAndResources(clientSet, uniqueNamespace, []string{"sample-daemonset.yaml"}); err != nil {
			t.Fatalf("Failed to delete namespaces/resources: %v", err)
		}
	}()
	//updating operator deployment
	deployment, err := clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting deployment: %v\n\n", err)
	}

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
	deployment, err = clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting deployment: %v\n\n", err)
	}
	updateAnnotationConfig(deployment, string(jsonStr))
	if !updateOperator(t, clientSet, deployment) {
		t.Errorf("Failed to update Operator")
	}
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

	if err != nil {
		t.Errorf("Error listing pods for fluent-bit daemonset: %s", err.Error())
	}
	//java annotations should be removed

	//java shouldn't be annotated in this case
	if checkIfAnnotationExists(daemonPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}) {
		t.Error("Incorrect Annotations")
	}
	if !checkIfAnnotationExists(daemonPods, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		t.Error("Incorrect Annotations")
	}

}

// ---------------------------USE CASE 7 (Java and Python on Namespace) ----------------------------------------------
func TestUseCase7(t *testing.T) {

	t.Parallel()
	clientSet := setupTest(t)
	sampleNamespace := "sample-namespace-7"

	if err := createNamespace(clientSet, sampleNamespace); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespace(clientSet, sampleNamespace); err != nil {
			t.Fatalf("Failed to delete namespace: %v", err)
		}
	}()
	//updating operator deployment
	deployment, err := clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting deployment: %v\n\n", err)
	}

	annotationConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{sampleNamespace},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
		Python: auto.AnnotationResources{
			Namespaces:   []string{sampleNamespace},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
	}
	jsonStr, err := json.Marshal(annotationConfig)
	if err != nil {
		t.Error("Error:", err)
	}
	deployment, err = clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting deployment: %v\n\n", err)
		//	}
		updateAnnotationConfig(deployment, string(jsonStr))
		if !updateOperator(t, clientSet, deployment) {
			t.Error("Failed to update Operator")
		}
		ns, err := clientSet.CoreV1().Namespaces().Get(context.TODO(), sampleNamespace, metav1.GetOptions{})
		if err != nil {
			t.Errorf("Error getting namespace %s", err.Error())
		}
		if !checkNameSpaceAnnotations(ns, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation, injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
			t.Error("Incorrect Annotations")
		}

	}
}

// ---------------------------USE CASE 8 (Java on Namespace Python should be removed) ----------------------------------------------
func TestUseCase8(t *testing.T) {

	t.Parallel()
	clientSet := setupTest(t)
	sampleNamespace := "sample-namespace-8"
	if err := createNamespace(clientSet, sampleNamespace); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespace(clientSet, sampleNamespace); err != nil {
			t.Fatalf("Failed to delete namespace: %v", err)
		}
	}()
	annotationConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{sampleNamespace},
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
	deployment, err := clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting deployment: %v\n\n", err)
	}

	updateAnnotationConfig(deployment, string(jsonStr))
	if !updateOperator(t, clientSet, deployment) {
		t.Error("Failed to update Operator")
	}
	ns, err := clientSet.CoreV1().Namespaces().Get(context.TODO(), sampleNamespace, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting namespace %s", err.Error())
	}

	//python should not exist
	if checkNameSpaceAnnotations(ns, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		t.Error("Incorrect Annotations")

	}

	if !checkNameSpaceAnnotations(ns, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}) {
		t.Error("Incorrect Annotations")
	}
	//------------------------------------USE CASE 8 End ----------------------------------------------

}

// ---------------------------USE CASE 9 (Python on Namespace and Java annotation should not exist) ----------------------------------------------
func TestUseCase9(t *testing.T) {

	t.Parallel()
	clientSet := setupTest(t)
	sampleNamespace := "sample-namespace-9"
	if err := createNamespace(clientSet, sampleNamespace); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespace(clientSet, sampleNamespace); err != nil {
			t.Fatalf("Failed to delete namespace: %v", err)
		}
	}()
	//updating operator deployment
	deployment, err := clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting deployment: %v\n\n", err)
	}

	annotationConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
		Python: auto.AnnotationResources{
			Namespaces:   []string{sampleNamespace},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
	}
	jsonStr, err := json.Marshal(annotationConfig)
	if err != nil {
		t.Error("Error:", err)
	}
	deployment, err = clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting deployment: %v\n\n", err)
	}
	updateAnnotationConfig(deployment, string(jsonStr))
	if !updateOperator(t, clientSet, deployment) {
		t.Error("Failed to update Operator")
	}
	ns, err := clientSet.CoreV1().Namespaces().Get(context.TODO(), sampleNamespace, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting namespace %s", err.Error())
	}
	//java annotations should not exist anymore
	if checkNameSpaceAnnotations(ns, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}) {
		t.Error("Incorrect Annotations")
	}
	if !checkNameSpaceAnnotations(ns, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		t.Error("Incorrect Annotations")
	}

}

// ---------------------------USE CASE 10 (Python and Java on Stateful set)------------------------------
func TestUseCase10(t *testing.T) {

	t.Parallel()
	clientSet := setupTest(t)
	uniqueNamespace := "sample-namespace-10"
	if err := createNamespaceAndApplyResources(t, clientSet, uniqueNamespace, []string{"sample-statefulset.yaml"}); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespaceAndResources(clientSet, uniqueNamespace, []string{"sample-statefulset.yaml"}); err != nil {
			t.Fatalf("Failed to delete namespaces/resources: %v", err)
		}
	}()
	//updating operator deployment
	deployment, err := clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting deployment: %v\n\n", err)
	}
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
	updateAnnotationConfig(deployment, string(jsonStr))
	if !updateOperator(t, clientSet, deployment) {
		t.Error("Failed to update Operator")
	}

	// Get the StatefulSet
	statefulSet, err := clientSet.AppsV1().StatefulSets(uniqueNamespace).Get(context.TODO(), "my-statefulset", metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get my-statefulset StatefulSet: %s\n", err.Error())
	}

	// List pods belonging to the StatefulSet
	set := labels.Set(statefulSet.Spec.Selector.MatchLabels)
	statefulSetPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})
	if err != nil {
		t.Errorf("Error listing pods for my-statefulset StatefulSet: %s\n", err.Error())
	}
	if !checkIfAnnotationExists(statefulSetPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation, injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		t.Error("Incorrect Annotations")
	}

}

// ---------------------------USE CASE 11 (Java on Stateful set and Python should be removed)------------------------------
func TestUseCase11(t *testing.T) {

	t.Parallel()
	clientSet := setupTest(t)
	uniqueNamespace := "sample-namespace-11"
	if err := createNamespaceAndApplyResources(t, clientSet, uniqueNamespace, []string{"sample-statefulset.yaml"}); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespaceAndResources(clientSet, uniqueNamespace, []string{"sample-statefulset.yaml"}); err != nil {
			t.Fatalf("Failed to delete namespaces/resources: %v", err)
		}
	}()
	//updating operator deployment
	deployment, err := clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting deployment: %v\n\n", err)
	}

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
	updateAnnotationConfig(deployment, string(jsonStr))
	if !updateOperator(t, clientSet, deployment) {
		t.Error("Failed to update Operator")
	}

	// Get the StatefulSet
	statefulSet, err := clientSet.AppsV1().StatefulSets(uniqueNamespace).Get(context.TODO(), "my-statefulset", metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get my-statefulset StatefulSet: %s\n", err.Error())
	}

	// List pods belonging to the StatefulSet
	set := labels.Set(statefulSet.Spec.Selector.MatchLabels)
	statefulSetPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})
	if err != nil {
		t.Errorf("Error listing pods for my-statefulset StatefulSet: %s\n", err.Error())
	}
	//Python should have been removed
	if checkIfAnnotationExists(statefulSetPods, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		t.Error("Incorrect Annotations")

	}
	if !checkIfAnnotationExists(statefulSetPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}) {
		t.Error("Incorrect Annotations")
	}
}

// ---------------------------USE CASE 12 (Python on Stateful set and java should be removed)------------------------------
func TestUseCase12(t *testing.T) {

	t.Parallel()
	clientSet := setupTest(t)
	uniqueNamespace := "sample-namespace-12"
	if err := createNamespaceAndApplyResources(t, clientSet, uniqueNamespace, []string{"sample-statefulset.yaml"}); err != nil {
		t.Fatalf("Failed to create/apply resoures on namespace: %v", err)
	}

	defer func() {
		if err := deleteNamespaceAndResources(clientSet, uniqueNamespace, []string{"sample-statefulset.yaml"}); err != nil {
			t.Fatalf("Failed to delete namespaces/resources: %v", err)
		}
	}()
	//updating operator deployment
	deployment, err := clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error getting deployment: %v\n\n", err)
	}

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
	updateAnnotationConfig(deployment, string(jsonStr))
	if !updateOperator(t, clientSet, deployment) {
		t.Error("Failed to update Operator")
	}

	// Get the StatefulSet
	statefulSet, err := clientSet.AppsV1().StatefulSets(uniqueNamespace).Get(context.TODO(), "my-statefulset", metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get my-statefulset StatefulSet: %s\n", err.Error())
	}

	// List pods belonging to the StatefulSet
	set := labels.Set(statefulSet.Spec.Selector.MatchLabels)
	statefulSetPods, err := clientSet.CoreV1().Pods(uniqueNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})
	if err != nil {
		t.Errorf("Error listing pods for my-statefulset StatefulSet: %s\n", err.Error())
	}

	//java shouldn't be annotated in this case
	if checkIfAnnotationExists(statefulSetPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}) {
		t.Error("Incorrect Annotations")
	}
	if !checkIfAnnotationExists(statefulSetPods, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		t.Error("Incorrect Annotations")
	}
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
		// Namespace exists - you can choose to return nil or delete and recreate it
		return nil // Use this line if you want to just use the existing namespace
		// Uncomment below lines if you want to delete and recreate the namespace
		// err = deleteNamespace(clientset, name)
		// if err != nil {
		// return err
		// }
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
	opMutex.Lock() // Lock the mutex at the beginning
	defer opMutex.Unlock()
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
	time.Sleep(70 * time.Second)
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
	if indexOfAutoAnnotationConfigString < 0 || indexOfAutoAnnotationConfigString >= len(deployment.Spec.Template.Spec.Containers[0].Args) {
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

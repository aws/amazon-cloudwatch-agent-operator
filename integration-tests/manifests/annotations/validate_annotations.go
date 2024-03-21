package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/auto"
	appsV1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {

	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("error getting user home dir: %v\n\n", err)
	}
	kubeConfigPath := filepath.Join(userHomeDir, ".kube", "config")
	fmt.Printf("Using kubeconfig: %s\n\n", kubeConfigPath)

	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		fmt.Printf("Error getting kubernetes config: %v\n\n", err)
	}

	clientSet, err := kubernetes.NewForConfig(kubeConfig)

	if err != nil {
		fmt.Printf("error getting kubernetes config: %v\n\n", err)
	}
	success := verifyAutoAnnotation(clientSet)
	if !success {
		fmt.Println("Instrumentation Annotation Injection Test: FAIL")
		os.Exit(1)
	} else {
		fmt.Println("Instrumentation Annotation Injection Test: PASS")
	}
}

const injectJavaAnnotation = "instrumentation.opentelemetry.io/inject-java"
const autoAnnotateJavaAnnotation = "cloudwatch.aws.amazon.com/auto-annotate-java"
const injectPythonAnnotation = "instrumentation.opentelemetry.io/inject-python"
const autoAnnotatePythonAnnotation = "cloudwatch.aws.amazon.com/auto-annotate-python"
const defaultNamespace = "default"
const deploymentName = "nginx"
const amazonCloudwatchNamespace = "amazon-cloudwatch"

const amazonControllerManager = "cloudwatch-controller-manager"

func verifyAutoAnnotation(clientSet *kubernetes.Clientset) bool {

	//updating operator deployment
	deployment, err := clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Error getting deployment: %v\n\n", err)
		os.Exit(1)
	}

	//---------------------------USE CASE 1 (Java and Python on Deployment) ----------------------------------------------
	annotationConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{"default/nginx"},
			StatefulSets: []string{""},
		},
		Python: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{"default/nginx"},
			StatefulSets: []string{""},
		},
	}
	jsonStr, err := json.Marshal(annotationConfig)
	if err != nil {
		fmt.Println("Error:", err)
		return false
	}
	deployment, err = clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Error getting deployment: %v\n\n", err)
		os.Exit(1)
	}
	updateAnnotationConfig(deployment, string(jsonStr))
	if !updateOperator(clientSet, deployment) {
		return false
	}

	//check if deployment has annotations.
	deployment, err = clientSet.AppsV1().Deployments(defaultNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Failed to get nginx deployment: %s", err.Error())
		return false
	}

	// List pods belonging to the nginx deployment
	set := labels.Set(deployment.Spec.Selector.MatchLabels)
	deploymentPods, err := clientSet.CoreV1().Pods(defaultNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})
	if err != nil {
		fmt.Printf("Error listing pods for nginx deployment: %s", err.Error())
		return false
	}

	//wait for pods to update
	if !checkIfAnnotationExists(deploymentPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation, injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		return false
	}
	//---------------------------------------------------USE CASE 1 End ---------------------------------------------------------

	//---------------------------USE CASE 2 (Java on Deployment and Python Should be Removed)------------------------------

	annotationConfig = auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{"default/nginx"},
			StatefulSets: []string{""},
		},
		Python: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
	}
	jsonStr, err = json.Marshal(annotationConfig)
	if err != nil {
		fmt.Println("Error:", err)
		return false
	}

	deployment, err = clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Error getting deployment: %v\n\n", err)
		os.Exit(1)
	}

	//finding where index of --auto-annotation-config= is (if it doesn't exist it will be appended)
	updateAnnotationConfig(deployment, string(jsonStr))
	if !updateOperator(clientSet, deployment) {
		return false
	}

	//check if deployment has annotations.
	deployment, err = clientSet.AppsV1().Deployments(defaultNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Failed to get nginx deployment: %s", err.Error())
		return false
	}

	// List pods belonging to the nginx deployment
	set = labels.Set(deployment.Spec.Selector.MatchLabels)
	deploymentPods, err = clientSet.CoreV1().Pods(deployment.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})
	if err != nil {
		fmt.Printf("Error listing pods for nginx deployment: %s", err.Error())
		return false
	}

	//Python should have been removed
	if checkIfAnnotationExists(deploymentPods, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		return false
	}
	//wait for pods to update
	if !checkIfAnnotationExists(deploymentPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}) {
		return false
	}

	//---------------------------USE CASE 2 End ----------------------------------------------

	//---------------------------USE CASE 3 (Python on Deployment and java annotations should be removed) ----------------------------------------------

	annotationConfig = auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
		Python: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{"default/nginx"},
			StatefulSets: []string{""},
		},
	}
	jsonStr, err = json.Marshal(annotationConfig)
	if err != nil {
		fmt.Println("Error:", err)
		return false
	}
	deployment, err = clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Error getting deployment: %v\n\n", err)
		os.Exit(1)
	}
	//finding where index of --auto-annotation-config= is (if it doesn't exist it will be appended)

	if !updateOperator(clientSet, deployment) {
		return false
	}

	//check if deployment has annotations.
	deployment, err = clientSet.AppsV1().Deployments(defaultNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Failed to get nginx deployment: %s", err.Error())
		return false
	}

	// List pods belonging to the nginx deployment
	set = labels.Set(deployment.Spec.Selector.MatchLabels)
	deploymentPods, err = clientSet.CoreV1().Pods(defaultNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})
	if err != nil {
		fmt.Printf("Error listing pods for nginx deployment: %s", err.Error())
		return false
	}

	//java shouldn't be annotated in this case
	if checkIfAnnotationExists(deploymentPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}) {
		return false
	}
	//wait for pods to update
	if !checkIfAnnotationExists(deploymentPods, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		return false
	}

	//---------------------------USE CASE 3 End ----------------------------------------------

	//---------------------------USE CASE 4 (Python and Java on DaemonSet)------------------------------

	annotationConfig = auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{"amazon-cloudwatch/fluent-bit"},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
		Python: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{"amazon-cloudwatch/fluent-bit"},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
	}
	jsonStr, err = json.Marshal(annotationConfig)
	if err != nil {
		fmt.Println("Error:", err)
		return false
	}

	deployment, err = clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Error getting deployment: %v\n\n", err)
		os.Exit(1)
	}
	if !updateOperator(clientSet, deployment) {
		return false
	}

	// Get the fluent-bit DaemonSet
	daemonSet, err := clientSet.AppsV1().DaemonSets(amazonCloudwatchNamespace).Get(context.TODO(), "fluent-bit", metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Failed to get fluent-bit daemonset: %s", err.Error())
	}

	// List pods belonging to the fluent-bit DaemonSet
	set = labels.Set(daemonSet.Spec.Selector.MatchLabels)
	daemonPods, err := clientSet.CoreV1().Pods(amazonCloudwatchNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})

	if err != nil {
		fmt.Printf("Error listing pods for fluent-bit daemonset: %s", err.Error())
	}
	if !checkIfAnnotationExists(daemonPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation, injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		return false
	}

	//---------------------------Use Case 4 End-------------------------------------

	//---------------------------USE CASE 5 (Java on DaemonSet and Python should be removed)------------------------------

	annotationConfig = auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{"amazon-cloudwatch/fluent-bit"},
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
	jsonStr, err = json.Marshal(annotationConfig)
	if err != nil {
		fmt.Println("Error:", err)
		return false
	}
	deployment, err = clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Error getting deployment: %v\n\n", err)
		os.Exit(1)
	}
	if !updateOperator(clientSet, deployment) {
		return false
	}

	// Get the fluent-bit DaemonSet
	daemonSet, err = clientSet.AppsV1().DaemonSets(amazonCloudwatchNamespace).Get(context.TODO(), "fluent-bit", metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Failed to get fluent-bit daemonset: %s", err.Error())
	}

	// List pods belonging to the fluent-bit DaemonSet
	set = labels.Set(daemonSet.Spec.Selector.MatchLabels)
	daemonPods, err = clientSet.CoreV1().Pods(amazonCloudwatchNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})
	if err != nil {
		fmt.Printf("Error listing pods for fluent-bit daemonset: %s", err.Error())
	}
	//Python should not exist on pods
	//Python should have been removed
	if checkIfAnnotationExists(daemonPods, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		return false
	}
	if !checkIfAnnotationExists(daemonPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}) {
		return false
	}

	//---------------------------Use Case 5 End-------------------------------------

	//---------------------------USE CASE 6 (Python on DaemonSet Java annotation should be removed)------------------------------

	annotationConfig = auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
		Python: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{"amazon-cloudwatch/fluent-bit"},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
	}
	jsonStr, err = json.Marshal(annotationConfig)
	if err != nil {
		fmt.Println("Error:", err)
		return false
	}
	deployment, err = clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Error getting deployment: %v\n\n", err)
		os.Exit(1)
	}
	if !updateOperator(clientSet, deployment) {
		return false
	}
	// Get the fluent-bit DaemonSet
	daemonSet, err = clientSet.AppsV1().DaemonSets(amazonCloudwatchNamespace).Get(context.TODO(), "fluent-bit", metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Failed to get fluent-bit daemonset: %s", err.Error())
	}

	// List pods belonging to the fluent-bit DaemonSet
	set = labels.Set(daemonSet.Spec.Selector.MatchLabels)
	daemonPods, err = clientSet.CoreV1().Pods(amazonCloudwatchNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})

	if err != nil {
		fmt.Printf("Error listing pods for fluent-bit daemonset: %s", err.Error())
	}
	//java annotations should be removed

	//java shouldn't be annotated in this case
	if checkIfAnnotationExists(daemonPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}) {
		return false
	}
	if !checkIfAnnotationExists(daemonPods, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		return false
	}

	//---------------------------Use Case 6 End-------------------------------------

	//---------------------------USE CASE 7 (Java and Python on Namespace) ----------------------------------------------

	annotationConfig = auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{defaultNamespace},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
		Python: auto.AnnotationResources{
			Namespaces:   []string{defaultNamespace},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
	}
	jsonStr, err = json.Marshal(annotationConfig)
	if err != nil {
		fmt.Println("Error:", err)
		return false
	}
	deployment, err = clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Error getting deployment: %v\n\n", err)
		os.Exit(1)
	}
	if !updateOperator(clientSet, deployment) {
		return false
	}

	ns, err := clientSet.CoreV1().Namespaces().Get(context.TODO(), defaultNamespace, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Error getting namespace %s", err.Error())
		return false
	}
	if !checkNameSpaceAnnotations(ns, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation, injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		return false
	}

	//------------------------------------USE CASE 7 End ----------------------------------------------

	//---------------------------USE CASE 8 (Java on Namespace Python should be removed) ----------------------------------------------

	annotationConfig = auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{defaultNamespace},
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
	jsonStr, err = json.Marshal(annotationConfig)
	if err != nil {
		fmt.Println("Error:", err)
		return false
	}
	deployment, err = clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Error getting deployment: %v\n\n", err)
		os.Exit(1)
	}
	if !updateOperator(clientSet, deployment) {
		return false
	}

	ns, err = clientSet.CoreV1().Namespaces().Get(context.TODO(), defaultNamespace, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Error getting namespace %s", err.Error())
		return false
	}

	//python should not exist
	if checkNameSpaceAnnotations(ns, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		return false
	}

	if !checkNameSpaceAnnotations(ns, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}) {
		return false
	}
	//------------------------------------USE CASE 8 End ----------------------------------------------

	//---------------------------USE CASE 9 (Python on Namespace and Java annotation should not exist) ----------------------------------------------

	annotationConfig = auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
		Python: auto.AnnotationResources{
			Namespaces:   []string{defaultNamespace},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
	}
	jsonStr, err = json.Marshal(annotationConfig)
	if err != nil {
		fmt.Println("Error:", err)
		return false
	}
	deployment, err = clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Error getting deployment: %v\n\n", err)
		os.Exit(1)
	}
	if !updateOperator(clientSet, deployment) {
		return false
	}

	ns, err = clientSet.CoreV1().Namespaces().Get(context.TODO(), defaultNamespace, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Error getting namespace %s", err.Error())
		return false
	}
	//java annotations should not exist anymore
	if checkNameSpaceAnnotations(ns, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}) {
		return false
	}
	if !checkNameSpaceAnnotations(ns, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		return false
	}
	//------------------------------------USE CASE 9 End ----------------------------------------------

	//---------------------------USE CASE 10 (Python and Java on Stateful set)------------------------------

	annotationConfig = auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{"default/my-statefulset"},
		},
		Python: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{"default/my-statefulset"},
		},
	}
	jsonStr, err = json.Marshal(annotationConfig)
	if err != nil {
		fmt.Println("Error:", err)
		return false
	}

	if !updateOperator(clientSet, deployment) {
		return false
	}

	// Get the StatefulSet
	statefulSet, err := clientSet.AppsV1().StatefulSets(defaultNamespace).Get(context.TODO(), "my-statefulset", metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Failed to get my-statefulset StatefulSet: %s\n", err.Error())
	}

	// List pods belonging to the StatefulSet
	set = labels.Set(statefulSet.Spec.Selector.MatchLabels)
	statefulSetPods, err := clientSet.CoreV1().Pods(defaultNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})
	if err != nil {
		fmt.Printf("Error listing pods for my-statefulset StatefulSet: %s\n", err.Error())
	}
	if !checkIfAnnotationExists(statefulSetPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation, injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		return false
	}
	//---------------------------Use Case 10 End-------------------------------------

	//---------------------------USE CASE 11 (Java on Stateful set and Python should be removed)------------------------------

	annotationConfig = auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{"default/my-statefulset"},
		},
		Python: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
	}
	jsonStr, err = json.Marshal(annotationConfig)
	if err != nil {
		fmt.Println("Error:", err)
		return false
	}

	if !updateOperator(clientSet, deployment) {
		return false
	}

	// Get the StatefulSet
	statefulSet, err = clientSet.AppsV1().StatefulSets(defaultNamespace).Get(context.TODO(), "my-statefulset", metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Failed to get my-statefulset StatefulSet: %s\n", err.Error())
	}

	// List pods belonging to the StatefulSet
	set = labels.Set(statefulSet.Spec.Selector.MatchLabels)
	statefulSetPods, err = clientSet.CoreV1().Pods(defaultNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})
	if err != nil {
		fmt.Printf("Error listing pods for my-statefulset StatefulSet: %s\n", err.Error())
	}
	//Python should have been removed
	if checkIfAnnotationExists(statefulSetPods, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		return false
	}
	if !checkIfAnnotationExists(statefulSetPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}) {
		return false
	}
	//---------------------------Use Case 11 End-------------------------------------

	//---------------------------USE CASE 12 (Python on Stateful set and java should be removed)------------------------------

	annotationConfig = auto.AnnotationConfig{
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
			StatefulSets: []string{"default/my-statefulset"},
		},
	}
	jsonStr, err = json.Marshal(annotationConfig)
	if err != nil {
		fmt.Println("Error:", err)
		return false
	}

	if !updateOperator(clientSet, deployment) {
		return false
	}

	// Get the StatefulSet
	statefulSet, err = clientSet.AppsV1().StatefulSets(defaultNamespace).Get(context.TODO(), "my-statefulset", metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Failed to get my-statefulset StatefulSet: %s\n", err.Error())
	}

	// List pods belonging to the StatefulSet
	set = labels.Set(statefulSet.Spec.Selector.MatchLabels)
	statefulSetPods, err = clientSet.CoreV1().Pods(defaultNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})
	if err != nil {
		fmt.Printf("Error listing pods for my-statefulset StatefulSet: %s\n", err.Error())
	}

	//java shouldn't be annotated in this case
	if checkIfAnnotationExists(statefulSetPods, []string{injectJavaAnnotation, autoAnnotateJavaAnnotation}) {
		return false
	}
	if !checkIfAnnotationExists(statefulSetPods, []string{injectPythonAnnotation, autoAnnotatePythonAnnotation}) {
		return false
	}

	//---------------------------Use Case 12 End-------------------------------------

	return true

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
func updateOperator(clientSet *kubernetes.Clientset, deployment *appsV1.Deployment) bool {
	var err error
	args := deployment.Spec.Template.Spec.Containers[0].Args
	// Attempt to get the deployment by name
	deployment, err = clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), amazonControllerManager, metav1.GetOptions{})
	deployment.Spec.Template.Spec.Containers[0].Args = args
	if err != nil {
		fmt.Printf("Failed to get deployment: %v\n", err)
		return false
	}

	// Update the deployment
	_, err = clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		fmt.Printf("Failed to update deployment: %v\n", err)
		return false
	}

	fmt.Println("Deployment updated successfully!")
	time.Sleep(30 * time.Second)
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

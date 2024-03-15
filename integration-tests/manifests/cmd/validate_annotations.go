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

	args := os.Args
	namespace := args[1]

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
	deployments, err := ListDeployments(namespace, clientSet)

	success := verifyAutoAnnotation(deployments, clientSet)
	if !success {
		fmt.Println("Instrumentation Annotation Injection Test: FAIL")
		os.Exit(1)
	} else {
		fmt.Println("Instrumentation Annotation Injection Test: PASS")
	}
}

func verifyAutoAnnotation(deployments *appsV1.DeploymentList, clientSet *kubernetes.Clientset) bool {

	//updating operator deployment
	args := deployments.Items[0].Spec.Template.Spec.Containers[0].Args
	fmt.Println("These are the args: ", args)
	indexOfAutoAnnotationConfigString := findMatchingPrefix("--auto-annotation-config=", args)

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
	deployments.Items[0].Spec.Template.Spec.Containers[0].Args[indexOfAutoAnnotationConfigString] = "--auto-annotation-config=" + string(jsonStr)

	//finding where index of --auto-annotation-config= is (if it doesn't exist it will be appended)
	fmt.Println(indexOfAutoAnnotationConfigString, string(jsonStr))

	if !updateOperator(clientSet, deployments.Items[0].Spec.Template.Spec.Containers[0].Args) {
		return false
	}
	time.Sleep(15 * time.Second)

	//check if deployment has annotations.
	deployment, err := clientSet.AppsV1().Deployments("default").Get(context.TODO(), "nginx", metav1.GetOptions{})
	if err != nil {
		fmt.Println("Failed to get nginx deployment: %s", err.Error())
		return false
	}

	// List pods belonging to the nginx deployment
	set := labels.Set(deployment.Spec.Selector.MatchLabels)
	deploymentPods, err := clientSet.CoreV1().Pods("default").List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})
	if err != nil {
		fmt.Println("Error listing pods for nginx deployment: %s", err.Error())
		return false
	}
	//wait for pods to update
	if !checkIfAnnotationsExistJava(deploymentPods) {
		return false
	}
	//wait for pods to update
	if !checkIfAnnotationsExistPython(deploymentPods) {
		return false
	}

	//---------------------------------------------------USE CASE 1 End ---------------------------------------------------------

	//---------------------------USE CASE 2 (Java on Deployment and Python Should be Remove Python)------------------------------

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

	//finding where index of --auto-annotation-config= is (if it doesn't exist it will be appended)
	indexOfAutoAnnotationConfigString = updateAnnotationConfig(indexOfAutoAnnotationConfigString, deployments, string(jsonStr))
	fmt.Println("This is the index of annotation: ", indexOfAutoAnnotationConfigString)
	if !updateOperator(clientSet, deployments.Items[0].Spec.Template.Spec.Containers[0].Args) {
		return false
	}
	time.Sleep(15 * time.Second)

	//check if deployment has annotations.
	deployment, err = clientSet.AppsV1().Deployments("default").Get(context.TODO(), "nginx", metav1.GetOptions{})
	if err != nil {
		fmt.Println("Failed to get nginx deployment: %s", err.Error())
		return false
	}

	// List pods belonging to the nginx deployment
	set = labels.Set(deployment.Spec.Selector.MatchLabels)
	deploymentPods, err = clientSet.CoreV1().Pods(deployment.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})
	if err != nil {
		fmt.Println("Error listing pods for nginx deployment: %s", err.Error())
		return false
	}

	//Python should have been removed
	if checkIfAnnotationsExistPython(deploymentPods) {
		return false
	}
	//wait for pods to update
	if !checkIfAnnotationsExistJava(deploymentPods) {
		return false
	}

	//---------------------------USE CASE 1 End ----------------------------------------------

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
	deployments.Items[0].Spec.Template.Spec.Containers[0].Args[indexOfAutoAnnotationConfigString] = "--auto-annotation-config=" + string(jsonStr)

	//finding where index of --auto-annotation-config= is (if it doesn't exist it will be appended)
	fmt.Println(indexOfAutoAnnotationConfigString, string(jsonStr))

	if !updateOperator(clientSet, deployments.Items[0].Spec.Template.Spec.Containers[0].Args) {
		return false
	}
	time.Sleep(15 * time.Second)

	//check if deployment has annotations.
	deployment, err = clientSet.AppsV1().Deployments("default").Get(context.TODO(), "nginx", metav1.GetOptions{})
	if err != nil {
		fmt.Println("Failed to get nginx deployment: %s", err.Error())
		return false
	}

	// List pods belonging to the nginx deployment
	set = labels.Set(deployment.Spec.Selector.MatchLabels)
	deploymentPods, err = clientSet.CoreV1().Pods("default").List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})
	if err != nil {
		fmt.Println("Error listing pods for nginx deployment: %s", err.Error())
		return false
	}

	//java shouldn't be annotated in this case
	if checkIfAnnotationsExistJava(deploymentPods) {
		return false
	}

	//wait for pods to update
	if !checkIfAnnotationsExistPython(deploymentPods) {
		return false
	}

	//---------------------------USE CASE 3 End ----------------------------------------------

	//---------------------------USE CASE 4 (Python and Java on DaemonSet)------------------------------

	annotationConfig = auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{"default/fluent-bit"},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
		Python: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{"default/fluent-bit"},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
	}
	jsonStr, err = json.Marshal(annotationConfig)
	if err != nil {
		fmt.Println("Error:", err)
		return false
	}

	deployments.Items[0].Spec.Template.Spec.Containers[0].Args[indexOfAutoAnnotationConfigString] = "--auto-annotation-config=" + string(jsonStr)

	fmt.Println(indexOfAutoAnnotationConfigString, string(jsonStr))
	if !updateOperator(clientSet, deployments.Items[0].Spec.Template.Spec.Containers[0].Args) {
		return false
	}
	fmt.Println(indexOfAutoAnnotationConfigString, string(jsonStr))
	time.Sleep(15 * time.Second)

	// Get the fluent-bit DaemonSet
	daemonSet, err := clientSet.AppsV1().DaemonSets("default").Get(context.TODO(), "fluent-bit", metav1.GetOptions{})
	if err != nil {
		fmt.Println("Failed to get fluent-bit daemonset: %s", err.Error())
	}

	// List pods belonging to the fluent-bit DaemonSet
	set = labels.Set(daemonSet.Spec.Selector.MatchLabels)
	daemonPods, err := clientSet.CoreV1().Pods("default").List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})

	if err != nil {
		fmt.Println("Error listing pods for fluent-bit daemonset: %s", err.Error())
	}
	if !checkIfAnnotationsExistJava(daemonPods) {
		return false
	}
	if !checkIfAnnotationsExistPython(daemonPods) {
		return false
	}
	fmt.Printf("All fluent-bit pods have the correct annotations\n")
	//---------------------------Use Case 4 End-------------------------------------

	//---------------------------USE CASE 5 (Java on DaemonSet and Python should be removed)------------------------------

	annotationConfig = auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{""},
			DaemonSets:   []string{"default/fluent-bit"},
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

	deployments.Items[0].Spec.Template.Spec.Containers[0].Args[indexOfAutoAnnotationConfigString] = "--auto-annotation-config=" + string(jsonStr)

	fmt.Println(indexOfAutoAnnotationConfigString, string(jsonStr))
	if !updateOperator(clientSet, deployments.Items[0].Spec.Template.Spec.Containers[0].Args) {
		return false
	}
	time.Sleep(15 * time.Second)

	fmt.Println(indexOfAutoAnnotationConfigString, string(jsonStr))

	// Get the fluent-bit DaemonSet
	daemonSet, err = clientSet.AppsV1().DaemonSets("default").Get(context.TODO(), "fluent-bit", metav1.GetOptions{})
	if err != nil {
		fmt.Println("Failed to get fluent-bit daemonset: %s", err.Error())
	}

	// List pods belonging to the fluent-bit DaemonSet
	set = labels.Set(daemonSet.Spec.Selector.MatchLabels)
	daemonPods, err = clientSet.CoreV1().Pods("default").List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})
	if err != nil {
		fmt.Println("Error listing pods for fluent-bit daemonset: %s", err.Error())
	}
	//Python should not exist on pods
	if checkIfAnnotationsExistPython(daemonPods) {
		return false
	}
	if !checkIfAnnotationsExistJava(daemonPods) {
		return false
	}
	fmt.Printf("All fluent-bit pods have the correct annotations\n")
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
			DaemonSets:   []string{"default/fluent-bit"},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
	}
	jsonStr, err = json.Marshal(annotationConfig)
	if err != nil {
		fmt.Println("Error:", err)
		return false
	}

	deployments.Items[0].Spec.Template.Spec.Containers[0].Args[indexOfAutoAnnotationConfigString] = "--auto-annotation-config=" + string(jsonStr)

	fmt.Println(indexOfAutoAnnotationConfigString, string(jsonStr))
	if !updateOperator(clientSet, deployments.Items[0].Spec.Template.Spec.Containers[0].Args) {
		return false
	}
	fmt.Println(indexOfAutoAnnotationConfigString, string(jsonStr))
	time.Sleep(15 * time.Second)

	// Get the fluent-bit DaemonSet
	daemonSet, err = clientSet.AppsV1().DaemonSets("default").Get(context.TODO(), "fluent-bit", metav1.GetOptions{})
	if err != nil {
		fmt.Println("Failed to get fluent-bit daemonset: %s", err.Error())
	}

	// List pods belonging to the fluent-bit DaemonSet
	set = labels.Set(daemonSet.Spec.Selector.MatchLabels)
	daemonPods, err = clientSet.CoreV1().Pods("default").List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})

	if err != nil {
		fmt.Println("Error listing pods for fluent-bit daemonset: %s", err.Error())
	}
	//java annotations should be removed
	if checkIfAnnotationsExistJava(daemonPods) {
		return false
	}
	if !checkIfAnnotationsExistPython(daemonPods) {
		return false
	}
	fmt.Printf("All fluent-bit pods have the correct annotations\n")
	//---------------------------Use Case 6 End-------------------------------------

	//---------------------------USE CASE 7 (Java and Python on Namespace) ----------------------------------------------

	annotationConfig = auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{"default"},
			DaemonSets:   []string{""},
			Deployments:  []string{""},
			StatefulSets: []string{""},
		},
		Python: auto.AnnotationResources{
			Namespaces:   []string{"default"},
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
	deployments.Items[0].Spec.Template.Spec.Containers[0].Args[indexOfAutoAnnotationConfigString] = "--auto-annotation-config=" + string(jsonStr)

	//finding where index of --auto-annotation-config= is (if it doesn't exist it will be appended)
	fmt.Println(indexOfAutoAnnotationConfigString, string(jsonStr))

	if !updateOperator(clientSet, deployments.Items[0].Spec.Template.Spec.Containers[0].Args) {
		return false
	}
	time.Sleep(15 * time.Second)

	ns, err := clientSet.CoreV1().Namespaces().Get(context.TODO(), "default", metav1.GetOptions{})
	if err != nil {
		fmt.Println("Error getting namespace %s", err.Error())
		return false
	}
	if !checkNameSpaceAnnotationsJava(ns) {
		return false
	}
	//wait for pods to update
	if !checkNameSpaceAnnotationsPython(ns) {
		return false
	}
	//------------------------------------USE CASE 7 End ----------------------------------------------

	//---------------------------USE CASE 8 (Java on Namespace Python should be removed) ----------------------------------------------

	annotationConfig = auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces:   []string{"default"},
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
	deployments.Items[0].Spec.Template.Spec.Containers[0].Args[indexOfAutoAnnotationConfigString] = "--auto-annotation-config=" + string(jsonStr)

	//finding where index of --auto-annotation-config= is (if it doesn't exist it will be appended)
	fmt.Println(indexOfAutoAnnotationConfigString, string(jsonStr))

	if !updateOperator(clientSet, deployments.Items[0].Spec.Template.Spec.Containers[0].Args) {
		return false
	}
	time.Sleep(15 * time.Second)

	ns, err = clientSet.CoreV1().Namespaces().Get(context.TODO(), "default", metav1.GetOptions{})
	if err != nil {
		fmt.Println("Error getting namespace %s", err.Error())
		return false
	}

	if checkNameSpaceAnnotationsPython(ns) {
		return false
	}
	//wait for pods to update
	if !checkNameSpaceAnnotationsJava(ns) {
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
			Namespaces:   []string{"default"},
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
	deployments.Items[0].Spec.Template.Spec.Containers[0].Args[indexOfAutoAnnotationConfigString] = "--auto-annotation-config=" + string(jsonStr)

	//finding where index of --auto-annotation-config= is (if it doesn't exist it will be appended)
	fmt.Println(indexOfAutoAnnotationConfigString, string(jsonStr))

	if !updateOperator(clientSet, deployments.Items[0].Spec.Template.Spec.Containers[0].Args) {
		return false
	}
	time.Sleep(15 * time.Second)

	ns, err = clientSet.CoreV1().Namespaces().Get(context.TODO(), "default", metav1.GetOptions{})
	if err != nil {
		fmt.Println("Error getting namespace %s", err.Error())
		return false
	}
	//java annotations should not exist anymore
	if checkNameSpaceAnnotationsJava(ns) {
		return false
	}
	//wait for pods to update
	if !checkNameSpaceAnnotationsPython(ns) {
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

	deployments.Items[0].Spec.Template.Spec.Containers[0].Args[indexOfAutoAnnotationConfigString] = "--auto-annotation-config=" + string(jsonStr)

	fmt.Println(indexOfAutoAnnotationConfigString, string(jsonStr))
	if !updateOperator(clientSet, deployments.Items[0].Spec.Template.Spec.Containers[0].Args) {
		return false
	}
	time.Sleep(15 * time.Second)

	fmt.Println(indexOfAutoAnnotationConfigString, string(jsonStr))

	// Get the StatefulSet
	statefulSet, err := clientSet.AppsV1().StatefulSets("default").Get(context.TODO(), "my-statefulset", metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Failed to get my-statefulset StatefulSet: %s\n", err.Error())
	}

	// List pods belonging to the StatefulSet
	set = labels.Set(statefulSet.Spec.Selector.MatchLabels)
	statefulSetPods, err := clientSet.CoreV1().Pods("default").List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})
	if err != nil {
		fmt.Printf("Error listing pods for my-statefulset StatefulSet: %s\n", err.Error())
	}
	if !checkIfAnnotationsExistJava(statefulSetPods) {
		return false
	}
	if !checkIfAnnotationsExistPython(statefulSetPods) {
		return false
	}
	fmt.Printf("All my-statefulset pods have the correct annotations\n")
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

	deployments.Items[0].Spec.Template.Spec.Containers[0].Args[indexOfAutoAnnotationConfigString] = "--auto-annotation-config=" + string(jsonStr)

	fmt.Println(indexOfAutoAnnotationConfigString, string(jsonStr))
	if !updateOperator(clientSet, deployments.Items[0].Spec.Template.Spec.Containers[0].Args) {
		return false
	}
	time.Sleep(15 * time.Second)

	fmt.Println(indexOfAutoAnnotationConfigString, string(jsonStr))

	// Get the StatefulSet
	statefulSet, err = clientSet.AppsV1().StatefulSets("default").Get(context.TODO(), "my-statefulset", metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Failed to get my-statefulset StatefulSet: %s\n", err.Error())
	}

	// List pods belonging to the StatefulSet
	set = labels.Set(statefulSet.Spec.Selector.MatchLabels)
	statefulSetPods, err = clientSet.CoreV1().Pods("default").List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})
	if err != nil {
		fmt.Printf("Error listing pods for my-statefulset StatefulSet: %s\n", err.Error())
	}
	if checkIfAnnotationsExistPython(statefulSetPods) {
		return false
	}
	if !checkIfAnnotationsExistJava(statefulSetPods) {
		return false
	}
	fmt.Printf("All my-statefulset pods have the correct annotations\n")
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

	deployments.Items[0].Spec.Template.Spec.Containers[0].Args[indexOfAutoAnnotationConfigString] = "--auto-annotation-config=" + string(jsonStr)

	fmt.Println(indexOfAutoAnnotationConfigString, string(jsonStr))
	if !updateOperator(clientSet, deployments.Items[0].Spec.Template.Spec.Containers[0].Args) {
		return false
	}
	time.Sleep(15 * time.Second)

	fmt.Println(indexOfAutoAnnotationConfigString, string(jsonStr))

	// Get the StatefulSet
	statefulSet, err = clientSet.AppsV1().StatefulSets("default").Get(context.TODO(), "my-statefulset", metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Failed to get my-statefulset StatefulSet: %s\n", err.Error())
	}

	// List pods belonging to the StatefulSet
	set = labels.Set(statefulSet.Spec.Selector.MatchLabels)
	statefulSetPods, err = clientSet.CoreV1().Pods("default").List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})
	if err != nil {
		fmt.Printf("Error listing pods for my-statefulset StatefulSet: %s\n", err.Error())
	}
	if checkIfAnnotationsExistJava(statefulSetPods) {
		return false
	}
	if !checkIfAnnotationsExistPython(statefulSetPods) {
		return false
	}
	fmt.Printf("All my-statefulset pods have the correct annotations\n")
	//---------------------------Use Case 12 End-------------------------------------

	return true

}

func checkNameSpaceAnnotationsJava(ns *v1.Namespace) bool {
	if ns.ObjectMeta.Annotations["instrumentation.opentelemetry.io/inject-java"] != "true" {
		return false
	}
	if ns.ObjectMeta.Annotations["cloudwatch.aws.amazon.com/auto-annotate-java"] != "true" {
		return false
	}
	return true
}
func checkNameSpaceAnnotationsPython(ns *v1.Namespace) bool {
	if ns.ObjectMeta.Annotations["instrumentation.opentelemetry.io/inject-python"] != "true" {
		return false
	}
	if ns.ObjectMeta.Annotations["cloudwatch.aws.amazon.com/auto-annotate-python"] != "true" {
		return false
	}
	return true
}
func updateOperator(clientSet *kubernetes.Clientset, Args []string) bool {
	var err error

	// Attempt to get the deployment by name
	deployment, err := clientSet.AppsV1().Deployments("amazon-cloudwatch").Get(context.TODO(), "amazon-cloudwatch-observability-controller-manager", metav1.GetOptions{})
	//fmt.Println("This is the deployment args: ", deployment.Spec.Template.Spec.Containers[0].Args)
	deployment.Spec.Template.Spec.Containers[0].Args = Args
	//fmt.Println("This is the deployment args: ", deployment.Spec.Template.Spec.Containers[0].Args)
	if err != nil {
		fmt.Printf("Failed to get deployment: %v\n", err)
		return false
	}

	// Update the deployment
	_, err = clientSet.AppsV1().Deployments("amazon-cloudwatch").Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		fmt.Printf("Failed to update deployment: %v\n", err)
		return false
	}

	fmt.Println("Deployment updated successfully!")
	time.Sleep(5 * time.Second)
	return true

}

func checkIfAnnotationsExistJava(deploymentPods *v1.PodList) bool {
	for _, pod := range deploymentPods.Items {

		fmt.Printf("This is the key: %v, this is value: %v\n", "instrumentation.opentelemetry.io/inject-java", pod.ObjectMeta.Annotations["instrumentation.opentelemetry.io/inject-java"])
		fmt.Println("pod name: ", pod.Name)
		if pod.ObjectMeta.Annotations["instrumentation.opentelemetry.io/inject-java"] != "true" {
			return false
		}
		if pod.ObjectMeta.Annotations["cloudwatch.aws.amazon.com/auto-annotate-java"] != "true" {
			return false
		}

	}

	fmt.Printf("All pods have the correct annotations\n")
	return true
}
func checkIfAnnotationsExistPython(deploymentPods *v1.PodList) bool {
	for _, pod := range deploymentPods.Items {

		fmt.Printf("This is the key: %v, this is value: %v\n", "instrumentation.opentelemetry.io/inject-python", pod.ObjectMeta.Annotations["instrumentation.opentelemetry.io/inject-python"])
		fmt.Println("pod name: ", pod.Name)

		if pod.ObjectMeta.Annotations["instrumentation.opentelemetry.io/inject-python"] != "true" {
			return false
		}
		if pod.ObjectMeta.Annotations["cloudwatch.aws.amazon.com/auto-annotate-python"] != "true" {
			return false
		}

	}

	fmt.Printf("All pods have the correct annotations\n")
	return true
}

func updateAnnotationConfig(indexOfAutoAnnotationConfigString int, deployments *appsV1.DeploymentList, jsonStr string) int {
	//fmt.Printf("Index of annotation %v and this is length of deployment args %v \n", indexOfAutoAnnotationConfigString, len(deployments.Items[0].Spec.Template.Spec.Containers[0].Args))
	//if auto annotation not part of config, we will add it
	if indexOfAutoAnnotationConfigString < 0 || indexOfAutoAnnotationConfigString >= len(deployments.Items[0].Spec.Template.Spec.Containers[0].Args) {
		fmt.Println("We are in the if statement")
		deployments.Items[0].Spec.Template.Spec.Containers[0].Args = append(deployments.Items[0].Spec.Template.Spec.Containers[0].Args, "--auto-annotation-config="+jsonStr)
		indexOfAutoAnnotationConfigString = len(deployments.Items[0].Spec.Template.Spec.Containers[0].Args) - 1
		fmt.Println("AutoAnnotationConfiguration: " + deployments.Items[0].Spec.Template.Spec.Containers[0].Args[indexOfAutoAnnotationConfigString])
		//fmt.Println("This is the updated index of annotation: ", indexOfAutoAnnotationConfigString)
		//fmt.Println("These are the args: ", deployments.Items[0].Spec.Template.Spec.Containers[0].Args)
	} else {
		//fmt.Println("We are in the else statement")
		deployments.Items[0].Spec.Template.Spec.Containers[0].Args[indexOfAutoAnnotationConfigString] = "--auto-annotation-config=" + jsonStr
		//fmt.Println("AutoAnnotationConfiguration: " + deployments.Items[0].Spec.Template.Spec.Containers[0].Args[indexOfAutoAnnotationConfigString])
	}
	return indexOfAutoAnnotationConfigString
}
func ListDeployments(namespace string, client kubernetes.Interface) (*appsV1.DeploymentList, error) {
	deployments, err := client.AppsV1().Deployments(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		err = fmt.Errorf("error getting Deploymets: %v\n", err)
		return nil, err
	}
	return deployments, nil
}

func findMatchingPrefix(str string, strs []string) int {
	for i, s := range strs {
		if strings.HasPrefix(s, str) {
			return i
		}
	}
	return -1 // Return -1 if no matching prefix is found
}

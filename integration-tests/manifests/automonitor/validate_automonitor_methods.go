// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
package annotations

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/auto"

	"github.com/google/uuid"
	appsV1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/aws/amazon-cloudwatch-agent-operator/integration-tests/util"
)

const (
	injectJavaAnnotation         = "instrumentation.opentelemetry.io/inject-java"
	autoAnnotateJavaAnnotation   = "cloudwatch.aws.amazon.com/auto-annotate-java"
	injectPythonAnnotation       = "instrumentation.opentelemetry.io/inject-python"
	autoAnnotatePythonAnnotation = "cloudwatch.aws.amazon.com/auto-annotate-python"
	injectDotNetAnnotation       = "instrumentation.opentelemetry.io/inject-dotnet"
	autoAnnotateDotNetAnnotation = "cloudwatch.aws.amazon.com/auto-annotate-dotnet"
	injectNodeJSAnnotation       = "instrumentation.opentelemetry.io/inject-nodejs"
	autoAnnotateNodeJSAnnotation = "cloudwatch.aws.amazon.com/auto-annotate-nodejs"

	deploymentName            = "sample-deployment"
	nginxDeploymentName       = "nginx"
	statefulSetName           = "sample-statefulset"
	amazonCloudwatchNamespace = "amazon-cloudwatch"
	daemonSetName             = "sample-daemonset"

	sampleDaemonsetYamlRelPath       = "../sample-daemonset.yaml"
	sampleDeploymentYaml             = "../sample-deployment.yaml"
	sampleNginxAppYamlNameRelPath    = "../../java/sample-deployment-java.yaml"
	sampleStatefulsetYamlNameRelPath = "../sample-statefulset.yaml"

	timoutDuration     = 2 * time.Minute
	numberOfRetries    = 10
	timeBetweenRetries = 5 * time.Second
)

var (
	amazonControllerManager = flag.String("controllerManagerName", "amazon-cloudwatch-observability-controller-manager", "short")
)

type TestHelper struct {
	clientSet  *kubernetes.Clientset
	t          *testing.T
	startTime  time.Time
	skipDelete bool
	logger     logr.Logger
}

func NewTestHelper(t *testing.T, skipDelete bool) *TestHelper {
	logger := testr.New(t)
	return &TestHelper{
		clientSet:  setupTest(t, logger),
		t:          t,
		skipDelete: skipDelete,
		logger:     logger,
	}
}

func setupTest(t *testing.T, logger logr.Logger) *kubernetes.Clientset {
	userHomeDir, err := os.UserHomeDir()

	if err != nil {
		t.Errorf("error getting user home dir: %v\n\n", err)
	}
	kubeConfigPath := filepath.Join(userHomeDir, ".kube", "config")
	logger.Info(fmt.Sprintf("Using kubeconfig: %s\n\n", kubeConfigPath))

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

func (h *TestHelper) ApplyYAMLWithKubectl(filename, namespace string) error {
	cmd := exec.Command("kubectl", "apply", "-f", filename, "-n", namespace)
	h.logger.Info(fmt.Sprintf("Applying YAML with kubectl %s\n", cmd))
	return cmd.Run()
}

func (h *TestHelper) CreateNamespaceAndApplyResources(namespace string, resourceFiles []string) error {
	h.logger.Info(fmt.Sprintf("Creating namespace %s\n", namespace))
	err := h.CreateNamespace(namespace)
	if err != nil {
		return err
	}

	for _, file := range resourceFiles {
		err = h.ApplyYAMLWithKubectl(file, namespace)
		if err != nil {
			h.t.Errorf("Could not apply resources %s/%s\n", namespace, file)
			return err
		}
	}

	for _, file := range resourceFiles {
		err = h.WaitYamlWithKubectl(file, namespace)
		if err != nil {
			h.t.Errorf("Could not wait resources %s/%s\n", namespace, file)
			return err
		}
		// ignore error because it could run on resource which doesn't rollout (aka a service)
		_ = h.RolloutWaitYamlWithKubectl(file, namespace)
	}
	return nil
}

func (h *TestHelper) IsNamespaceUpdated(namespace string) bool {
	ns, err := h.clientSet.CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{})
	if err != nil {
		h.logger.Info(fmt.Sprintf("Failed to get namespace %s: %v\n", namespace, err))
		return false
	}
	return ns.CreationTimestamp.After(h.startTime) || ns.ResourceVersion != ""
}

func (h *TestHelper) DeleteYAMLWithKubectl(filename, namespace string) error {
	cmd := exec.Command("kubectl", "delete", "-f", filename, "-n", namespace)
	return cmd.Run()
}

func (h *TestHelper) DeleteNamespaceAndResources(name string, resourceFiles []string) error {
	for _, file := range resourceFiles {
		if err := h.DeleteYAMLWithKubectl(file, name); err != nil {
			return err
		}
	}
	return h.DeleteNamespace(name)
}

// CreateNamespace if not already created
func (h *TestHelper) CreateNamespace(name string) error {
	_, err := h.clientSet.CoreV1().Namespaces().Get(context.Background(), name, metav1.GetOptions{})
	if err == nil {
		return nil
	} else if !errors.IsNotFound(err) {
		return err
	}

	namespace := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	_, err = h.clientSet.CoreV1().Namespaces().Create(context.Background(), namespace, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	startTime := time.Now()
	for {
		if time.Since(startTime) > timoutDuration {
			return fmt.Errorf("timeout reached while waiting for namespace %s to be created", name)
		}

		_, err := h.clientSet.CoreV1().Namespaces().Get(context.Background(), name, metav1.GetOptions{})
		if err == nil {
			return nil
		} else if !errors.IsNotFound(err) {
			return err
		}

		time.Sleep(timeBetweenRetries)
	}
}

func (h *TestHelper) DeleteNamespace(name string) error {
	return h.clientSet.CoreV1().Namespaces().Delete(context.Background(), name, metav1.DeleteOptions{})
}

func (h *TestHelper) CheckNameSpaceAnnotations(expectedAnnotations []string, uniqueNamespace string) bool {
	if err := h.CreateNamespace(uniqueNamespace); err != nil {
		h.t.Fatalf("Failed to create/apply resources on namespace: %v", err)
	}

	defer func() {
		if err := h.DeleteNamespace(uniqueNamespace); err != nil {
			h.t.Fatalf("Failed to delete namespace: %v", err)
		}
	}()

	for {
		if h.IsNamespaceUpdated(uniqueNamespace) {
			h.logger.Info(fmt.Sprintf("Namespace %s has been updated.\n", uniqueNamespace))
			break
		}
		elapsed := time.Since(h.startTime)
		if elapsed >= timoutDuration {
			h.logger.Info(fmt.Sprintf("Timeout reached while waiting for namespace %s to be updated.\n", uniqueNamespace))
			break
		}
	}

	for i := 0; i < numberOfRetries; i++ {
		correct := true
		ns, err := h.clientSet.CoreV1().Namespaces().Get(context.TODO(), uniqueNamespace, metav1.GetOptions{})
		if err != nil {
			h.logger.Error(err, "There was an error getting namespace, ")
			return false
		}

		for _, annotation := range expectedAnnotations {
			if ns.ObjectMeta.Annotations[annotation] != "true" {
				time.Sleep(timeBetweenRetries)
				correct = false
				break
			}
		}

		if correct {
			h.logger.Info("Namespace annotations are correct!")
			return true
		}
	}
	return false
}

func (h *TestHelper) UpdateOperator(deployment *appsV1.Deployment) bool {
	args := deployment.Spec.Template.Spec.Containers[0].Args
	now := time.Now()

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Get the latest version of the deployment
		currentDeployment, err := h.clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), *amazonControllerManager, metav1.GetOptions{})
		if err != nil {
			return err
		}

		// Apply your changes to the latest version
		currentDeployment.Spec.Template.Spec.Containers[0].Args = args
		forceRestart(currentDeployment)

		// Try to update
		_, updateErr := h.clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Update(context.TODO(), currentDeployment, metav1.UpdateOptions{})
		return updateErr
	})

	if retryErr != nil {
		h.t.Errorf("Failed to update deployment after retries: %v\n", retryErr)
		return false
	}

	err := util.WaitForNewPodCreation(h.clientSet, deployment, now)
	if err != nil {
		h.logger.Error(err, "There was an error trying to wait for deployment available")
		return false
	}

	h.logger.Info("Operator updated successfully!", "args", args)
	return true
}

func forceRestart(deployment *appsV1.Deployment) {
	annotations := deployment.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations["test-restart"] = time.Now().String()
	deployment.Spec.Template.SetAnnotations(annotations)
}

func (h *TestHelper) PodsAnnotationsValid(namespace string, shouldExistAnnotations []string, shouldNotExistAnnotations []string) bool {
	currentPods, err := h.clientSet.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		h.logger.Info(fmt.Sprintf("Failed to list pods: %v\n", err))
		return false
	}
	for _, pod := range currentPods.Items {
		h.logger.Info(fmt.Sprintf("Pod %s is in phase %s\n", pod.Name, pod.Status.Phase))
		if pod.Status.Phase != v1.PodRunning {
			continue
		}
		for _, annotation := range shouldExistAnnotations {
			if value, exists := pod.Annotations[annotation]; !exists || value != "true" {
				h.logger.Info(fmt.Sprintf("%s/%s should have annotation %s", pod.Namespace, pod.Name, annotation))
				return false
			}
		}
		for _, annotation := range shouldNotExistAnnotations {
			if _, exists := pod.Annotations[annotation]; exists {
				h.logger.Info(fmt.Sprintf("%s/%s should not have annotation %s", pod.Namespace, pod.Name, annotation))
				return false
			}
		}
	}

	return true
}

func (h *TestHelper) findIndexOfPrefix(str string, strs []string) int {
	for i, s := range strs {
		if strings.HasPrefix(s, str) {
			return i
		}
	}
	return -1
}

func (h *TestHelper) UpdateMonitorConfig(config *auto.MonitorConfig) {
	jsonStr, err := json.Marshal(config)
	assert.Nil(h.t, err)

	h.logger.Info("Setting monitor config to:")
	util.PrettyPrint(config)
	h.updateOperatorConfig(string(jsonStr), "--auto-monitor-config=")
}

func (h *TestHelper) UpdateAnnotationConfig(config *auto.AnnotationConfig) {
	var jsonStr = ""
	if marshalledConfig, err := json.Marshal(config); config != nil && assert.Nil(h.t, err) {
		jsonStr = string(marshalledConfig)
	}
	h.logger.Info("Setting annotation config to ", "jsonStr", jsonStr)
	h.updateOperatorConfig(jsonStr, "--auto-annotation-config=")
}

func (h *TestHelper) updateOperatorConfig(jsonStr string, flag string) {
	deployment, err := h.clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), *amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		h.t.Errorf("Error getting deployment: %v\n\n", err)
		return
	}
	args := deployment.Spec.Template.Spec.Containers[0].Args
	indexOfAutoAnnotationConfigString := h.findIndexOfPrefix(flag, args)
	shouldDelete := len(jsonStr) == 0
	if indexOfAutoAnnotationConfigString < 0 {
		if !shouldDelete {
			deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, flag+jsonStr)
		}
	} else {
		if shouldDelete {
			deployment.Spec.Template.Spec.Containers[0].Args = slices.Delete(deployment.Spec.Template.Spec.Containers[0].Args, indexOfAutoAnnotationConfigString, indexOfAutoAnnotationConfigString+1)
		} else {
			deployment.Spec.Template.Spec.Containers[0].Args[indexOfAutoAnnotationConfigString] = flag + jsonStr
		}
	}

	if !h.UpdateOperator(deployment) {
		h.t.Error("Failed to update Operator", deployment, deployment.Name, deployment.Spec.Template.Spec.Containers[0].Args)
	}
	time.Sleep(5 * time.Second)
}

func (h *TestHelper) ValidateWorkloadAnnotations(resourceType, uniqueNamespace, resourceName string, shouldExist []string, shouldNotExist []string) error {
	return retry.OnError(retry.DefaultBackoff, func(err error) bool {
		return err != nil
	}, func() error {
		return h.ValidateWorkloadAnnotationsA(resourceType, uniqueNamespace, resourceName, shouldExist, shouldNotExist)
	})
}

func (h *TestHelper) ValidateWorkloadAnnotationsA(resourceType, uniqueNamespace, resourceName string, shouldExist []string, shouldNotExist []string) error {
	var annotations map[string]string
	switch resourceType {
	case "deployment":
		resource, err := h.clientSet.AppsV1().Deployments(uniqueNamespace).Get(context.TODO(), resourceName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		annotations = resource.Spec.Template.Annotations
	case "daemonset":
		resource, err := h.clientSet.AppsV1().DaemonSets(uniqueNamespace).Get(context.TODO(), resourceName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		annotations = resource.Spec.Template.Annotations
	case "statefulset":
		resource, err := h.clientSet.AppsV1().StatefulSets(uniqueNamespace).Get(context.TODO(), resourceName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		annotations = resource.Spec.Template.Annotations
	default:
		return fmt.Errorf("unsupported resource type: %s", resourceType)
	}
	for _, shouldExistAnnotation := range shouldExist {
		if _, ok := annotations[shouldExistAnnotation]; !ok {
			return fmt.Errorf("annotation should be present: %s", shouldExistAnnotation)
		}
	}
	for _, shouldNotExistAnnotation := range shouldNotExist {
		if _, ok := annotations[shouldNotExistAnnotation]; ok {
			return fmt.Errorf("annotation should not be present: %s", shouldNotExistAnnotation)
		}
	}
	return nil
	//if err := util.WaitForNewPodCreation(h.clientSet, resource, h.startTime); err != nil {
	//	return fmt.Errorf("error waiting for pod creation: %s", err.Error())
	//}
	//
	//if h.PodsAnnotationsValid(uniqueNamespace, shouldExist, shouldNotExist) {
	//	return nil
	//} else {
	//	return fmt.Errorf("A pod has invalid annotations")
	//}
}

func (h *TestHelper) CreateResource(uniqueNamespace string, sampleAppYamlPath string, skipDelete bool) error {
	if err := h.CreateNamespaceAndApplyResources(uniqueNamespace, []string{sampleAppYamlPath}); err != nil {
		return fmt.Errorf("failed to create/apply resources on namespace: %v", err)
	}

	if !skipDelete {
		h.t.Cleanup(func() {
			if err := h.DeleteNamespaceAndResources(uniqueNamespace, []string{sampleAppYamlPath}); err != nil {
				h.t.Fatalf("Failed to delete namespaces/resources: %v", err)
			}
		})
	}
	return nil
}

func (h *TestHelper) Initialize(namespace string, apps []string) string {
	newUUID := uuid.New()
	uniqueNamespace := fmt.Sprintf("%s-%s", namespace, newUUID.String())

	h.UpdateMonitorConfig(&auto.MonitorConfig{MonitorAllServices: false})
	h.UpdateAnnotationConfig(nil)
	h.startTime = time.Now()
	if err := h.CreateNamespaceAndApplyResources(uniqueNamespace, apps); err != nil {
		h.t.Fatalf("Failed to create/apply resources on namespace: %v", err)
	}

	if !h.skipDelete {
		h.t.Cleanup(func() {
			h.logger.Info(fmt.Sprintf("Deleting namespace %s and resources %s", uniqueNamespace, apps))
			if err := h.DeleteNamespaceAndResources(uniqueNamespace, apps); err != nil {
				h.t.Fatalf("Failed to delete namespaces/resources: %v", err)
			}
		})
	}

	return uniqueNamespace
}

func (h *TestHelper) NumberOfRevisions(deploymentName string, namespace string) int {
	numOfRevisions := 0
	i := 0
	for {
		cmd := exec.Command("kubectl", "rollout", "history", "deployment", deploymentName, "-n", namespace, "--revision", strconv.Itoa(i))
		if err := cmd.Run(); err != nil {
			break
		}
		numOfRevisions++
		i++
	}
	return numOfRevisions - 1
}

func (h *TestHelper) WaitYamlWithKubectl(filename string, namespace string) error {
	cmd := exec.Command("kubectl", "wait", "--for=create", "-f", filename, "-n", namespace)
	h.logger.Info(fmt.Sprintf("Waiting YAML with kubectl %s\n", cmd))
	return cmd.Run()
}

func (h *TestHelper) RolloutWaitYamlWithKubectl(filename string, namespace string) error {
	cmd := exec.Command("kubectl", "rollout", "status", "-f", filename, "-n", namespace)
	h.logger.Info(fmt.Sprintf("Waiting YAML with kubectl %s\n", cmd))
	return cmd.Run()
}

func (h *TestHelper) RestartDeployment(namespace string, deploymentName string) error {
	h.logger.Info(fmt.Sprintf("Restarting %s/%s...", namespace, deploymentName))
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Get the latest version of the deployment
		deployment, err := h.clientSet.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get deployment: %v", err)
		}

		// Add or update restart annotation
		if deployment.Spec.Template.Annotations == nil {
			deployment.Spec.Template.Annotations = make(map[string]string)
		}
		deployment.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

		// Update the deployment
		_, updateErr := h.clientSet.AppsV1().Deployments(namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
		if updateErr != nil {
			return fmt.Errorf("failed to update deployment: %v", updateErr)
		}

		// wait
		cmd := exec.Command("kubectl", "rollout", "status", "deployment/"+deploymentName, "-n", namespace)
		h.logger.Info(fmt.Sprintf("Waiting YAML with kubectl %s\n", cmd))
		return cmd.Run()
	})

	if retryErr != nil {
		return fmt.Errorf("failed to restart deployment %s: %v", deploymentName, retryErr)
	}

	// Wait for rollout to complete
	err := h.WaitForDeploymentRollout(namespace, deploymentName)
	if err != nil {
		return fmt.Errorf("failed to wait for deployment rollout: %v", err)
	}

	h.logger.Info(fmt.Sprintf("Successfully restarted deployment %s in namespace %s\n", deploymentName, namespace))
	return nil
}

// WaitForDeploymentRollout waits for the deployment to complete its rollout
func (h *TestHelper) WaitForDeploymentRollout(namespace string, deploymentName string) error {
	return wait.PollUntilContextTimeout(
		context.TODO(), // parent context
		time.Second*2,  // interval between polls
		time.Minute*5,  // timeout
		false,          // immediate (set to false to match PollImmediate behavior)
		func(ctx context.Context) (bool, error) {
			deployment, err := h.clientSet.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
			if err != nil {
				return false, err
			}

			// Check if the rollout is complete
			if deployment.Generation <= deployment.Status.ObservedGeneration &&
				deployment.Status.UpdatedReplicas == *deployment.Spec.Replicas &&
				deployment.Status.Replicas == *deployment.Spec.Replicas &&
				deployment.Status.AvailableReplicas == *deployment.Spec.Replicas {
				return true, nil
			}

			return false, nil
		})
}

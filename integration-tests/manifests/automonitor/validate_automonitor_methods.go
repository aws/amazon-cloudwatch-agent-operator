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
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/auto"

	appsV1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/aws/amazon-cloudwatch-agent-operator/integration-tests/util"
)

type workloadType string

const (
	Deployment  workloadType = "deployment"
	DaemonSet   workloadType = "daemonset"
	StatefulSet workloadType = "statefulset"

	injectJavaAnnotation         = "instrumentation.opentelemetry.io/inject-java"
	autoAnnotateJavaAnnotation   = "cloudwatch.aws.amazon.com/auto-annotate-java"
	injectPythonAnnotation       = "instrumentation.opentelemetry.io/inject-python"
	autoAnnotatePythonAnnotation = "cloudwatch.aws.amazon.com/auto-annotate-python"
	injectDotNetAnnotation       = "instrumentation.opentelemetry.io/inject-dotnet"
	autoAnnotateDotNetAnnotation = "cloudwatch.aws.amazon.com/auto-annotate-dotnet"
	injectNodeJSAnnotation       = "instrumentation.opentelemetry.io/inject-nodejs"
	autoAnnotateNodeJSAnnotation = "cloudwatch.aws.amazon.com/auto-annotate-nodejs"

	kubeSystemNamespace       = "kube-system"
	amazonCloudwatchNamespace = "amazon-cloudwatch"

	timoutDuration     = 2 * time.Minute
	timeBetweenRetries = 5 * time.Second
)

var (
	amazonControllerManager = flag.String("controllerManagerName", "cloudwatch-controller-manager", "short")
)

type TestHelper struct {
	clientSet *kubernetes.Clientset
	t         *testing.T
	startTime time.Time
	logger    logr.Logger
}

func NewTestHelper(t *testing.T) *TestHelper {
	logger := testr.New(t)
	return &TestHelper{
		clientSet: setupTest(t, logger),
		t:         t,
		logger:    logger,
	}
}

func (h *TestHelper) Initialize(namespace string) string {
	newUUID := uuid.New()
	uniqueNamespace := fmt.Sprintf("%s-%s", namespace, newUUID.String())

	h.UpdateMonitorAndAnnotationConfig(&auto.MonitorConfig{MonitorAllServices: false, RestartPods: false}, nil)
	h.startTime = time.Now()

	return uniqueNamespace
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
	cmd := exec.Command("kubectl", "apply", "--wait=true", "-f", filename, "-n", namespace)
	h.logger.Info(fmt.Sprintf("Applying YAML with kubectl %s\n", cmd))
	return cmd.Run()
}

func (h *TestHelper) CreateNamespaceAndApplyResources(namespace string, resourceFiles []string, skipDelete bool) error {
	h.logger.Info(fmt.Sprintf("Creating namespace %s\n", namespace))
	err := h.CreateNamespace(namespace, skipDelete)
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

	if !skipDelete {
		h.t.Cleanup(func() {
			h.logger.Info(fmt.Sprintf("Deleting resources %s in namespace %s", namespace, resourceFiles))
			if err := h.DeleteResources(namespace, resourceFiles); err != nil {
				h.t.Logf("Failed to delete resources: %v", err)
			}
		})
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

func (h *TestHelper) DeleteResources(name string, resourceFiles []string) error {
	for _, file := range resourceFiles {
		if err := h.DeleteYAMLWithKubectl(file, name); err != nil {
			return err
		}
	}
	return nil
}

// CreateNamespace if not already created
func (h *TestHelper) CreateNamespace(name string, skipDelete bool) error {
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
			break
		} else if !errors.IsNotFound(err) {
			return err
		}

		time.Sleep(timeBetweenRetries)
	}

	if !skipDelete {
		h.t.Cleanup(func() {
			h.logger.Info(fmt.Sprintf("Deleting namespace %v", namespace))
			if err := h.DeleteNamespace(name); err != nil {
				h.t.Fatalf("Failed to delete namespaces: %v", err)
			}
		})
	}
	return nil
}

func (h *TestHelper) DeleteNamespace(name string) error {
	return h.clientSet.CoreV1().Namespaces().Delete(context.Background(), name, metav1.DeleteOptions{})
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

func (h *TestHelper) findIndexOfPrefix(str string, strs []string) int {
	for i, s := range strs {
		if strings.HasPrefix(s, str) {
			return i
		}
	}
	return -1
}

func (h *TestHelper) UpdateMonitorAndAnnotationConfig(monitorConfig *auto.MonitorConfig, annotationConfig *auto.AnnotationConfig) {
	monitorStr, err := json.Marshal(monitorConfig)
	assert.Nil(h.t, err)

	h.logger.Info("Setting monitor config to:", "monitorStr", string(monitorStr))

	var annotationStr = ""
	if marshalledConfig, err := json.Marshal(annotationConfig); annotationConfig != nil && assert.Nil(h.t, err) {
		annotationStr = string(marshalledConfig)
	}
	h.logger.Info("Setting annotation config to ", "annotationStr", annotationStr)
	h.updateOperatorConfig(deploymentArg{string(monitorStr), "--auto-monitor-config="}, deploymentArg{annotationStr, "--auto-annotation-config="})

}

func (h *TestHelper) UpdateMonitorConfig(config *auto.MonitorConfig) {
	jsonStr, err := json.Marshal(config)
	assert.Nil(h.t, err)

	h.logger.Info("Setting monitor config to:")
	h.updateOperatorConfig(deploymentArg{string(jsonStr), "--auto-monitor-config="})
}

func (h *TestHelper) UpdateAnnotationConfig(config *auto.AnnotationConfig) {
	var jsonStr = ""
	if marshalledConfig, err := json.Marshal(config); config != nil && assert.Nil(h.t, err) {
		jsonStr = string(marshalledConfig)
	}
	h.logger.Info("Setting annotation config to ", "jsonStr", jsonStr)
	h.updateOperatorConfig(deploymentArg{jsonStr, "--auto-annotation-config="})
}

func (h *TestHelper) updateOperatorConfig(argList ...deploymentArg) {
	deployment, err := h.clientSet.AppsV1().Deployments(amazonCloudwatchNamespace).Get(context.TODO(), *amazonControllerManager, metav1.GetOptions{})
	if err != nil {
		h.t.Errorf("Error getting deployment: %v\n\n", err)
		return
	}
	args := deployment.Spec.Template.Spec.Containers[0].Args
	for _, x := range argList {
		jsonStr := x.jsonStr
		flag := x.flag
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
	}

	if !h.UpdateOperator(deployment) {
		h.t.Error("Failed to update Operator", deployment, deployment.Name, deployment.Spec.Template.Spec.Containers[0].Args)
	}
	time.Sleep(5 * time.Second)
}

type deploymentArg struct {
	jsonStr string
	flag    string
}

func (h *TestHelper) ValidateNamespaceAnnotations(namespace string, shouldExist []string, shouldNotExist []string) error {
	for {
		if h.IsNamespaceUpdated(namespace) {
			h.logger.Info(fmt.Sprintf("Namespace %s has been updated.\n", namespace))
			break
		}
		elapsed := time.Since(h.startTime)
		if elapsed >= timoutDuration {
			h.logger.Info(fmt.Sprintf("Timeout reached while waiting for namespace %s to be updated.\n", namespace))
			break
		}
	}

	maxRetries := 3
	var annotations map[string]string
	var err error
	var ns *v1.Namespace
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			h.logger.Info(fmt.Sprintf("Attempt %d/%d: Waiting 5 seconds before retrying...", attempt+1, maxRetries))
			time.Sleep(timeBetweenRetries)
		}

		ns, err = h.clientSet.CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{})
		if err != nil {
			h.logger.Error(err, "There was an error getting namespace")
			return err
		}

		annotations = ns.ObjectMeta.Annotations
		if annotations != nil && len(annotations) > 0 {
			break
		}

		h.logger.Info(fmt.Sprintf("Namespace annotations are empty or nil on attempt %d/%d", attempt+1, maxRetries))

		if attempt == maxRetries-1 {
			annotations = map[string]string{}
		}
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
}

func (h *TestHelper) ValidateWorkloadAnnotations(workloadType workloadType, namespace, resourceName string, shouldExist []string, shouldNotExist []string) error {
	return retry.OnError(retry.DefaultBackoff, func(err error) bool {
		return err != nil
	}, func() error {
		var annotations map[string]string
		switch workloadType {
		case Deployment:
			resource, err := h.clientSet.AppsV1().Deployments(namespace).Get(context.TODO(), resourceName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			annotations = resource.Spec.Template.Annotations
		case DaemonSet:
			resource, err := h.clientSet.AppsV1().DaemonSets(namespace).Get(context.TODO(), resourceName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			annotations = resource.Spec.Template.Annotations
		case StatefulSet:
			resource, err := h.clientSet.AppsV1().StatefulSets(namespace).Get(context.TODO(), resourceName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			annotations = resource.Spec.Template.Annotations
		default:
			return fmt.Errorf("unsupported resource type: %s", workloadType)
		}

		// resource level annotation validation
		if len(annotations) > 0 {
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
		}
		return nil
	})
}

func (h *TestHelper) ValidatePodsAnnotations(namespace string, shouldExist []string, shouldNotExist []string) error {
	currentPods, err := h.clientSet.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		h.logger.Info(fmt.Sprintf("Failed to list pods: %v\n", err))
		return err
	}
	for _, pod := range currentPods.Items {
		h.logger.Info(fmt.Sprintf("Pod %s is in phase %s\n", pod.Name, pod.Status.Phase))
		if pod.Status.Phase != v1.PodRunning {
			continue
		}
		annotations := pod.Annotations
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
	}

	return nil
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

func (h *TestHelper) RestartWorkload(wlType workloadType, namespace, name string) error {
	h.logger.Info(fmt.Sprintf("Restarting %s/%s...", namespace, name))

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var updateErr error

		switch wlType {
		case Deployment:
			deployment, err := h.clientSet.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get deployment: %v", err)
			}

			if deployment.Spec.Template.Annotations == nil {
				deployment.Spec.Template.Annotations = make(map[string]string)
			}
			deployment.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

			_, updateErr = h.clientSet.AppsV1().Deployments(namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})

		case DaemonSet:
			ds, err := h.clientSet.AppsV1().DaemonSets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get daemonset: %v", err)
			}

			if ds.Spec.Template.Annotations == nil {
				ds.Spec.Template.Annotations = make(map[string]string)
			}
			ds.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

			_, updateErr = h.clientSet.AppsV1().DaemonSets(namespace).Update(context.TODO(), ds, metav1.UpdateOptions{})

		case StatefulSet:
			ss, err := h.clientSet.AppsV1().StatefulSets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get statefulset: %v", err)
			}

			if ss.Spec.Template.Annotations == nil {
				ss.Spec.Template.Annotations = make(map[string]string)
			}
			ss.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

			_, updateErr = h.clientSet.AppsV1().StatefulSets(namespace).Update(context.TODO(), ss, metav1.UpdateOptions{})

		}

		if updateErr != nil {
			return fmt.Errorf("failed to update %s: %v", wlType, updateErr)
		}

		cmd := exec.Command("kubectl", "rollout", "status", fmt.Sprintf("%s/%s", wlType, name), "-n", namespace)
		h.logger.Info(fmt.Sprintf("Waiting YAML with kubectl %s\n", cmd))
		return cmd.Run()
	})

	if retryErr != nil {
		return fmt.Errorf("failed to restart %s %s: %v", wlType, name, retryErr)
	}

	err := h.waitForWorkloadRollout(wlType, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to wait for %s rollout: %v", wlType, err)
	}

	h.logger.Info(fmt.Sprintf("Successfully restarted %s %s in namespace %s\n", wlType, name, namespace))
	return nil
}

func (h *TestHelper) waitForWorkloadRollout(wlType workloadType, namespace, name string) error {
	return wait.PollUntilContextTimeout(
		context.TODO(), // parent context
		time.Second*2,  // interval between polls
		time.Minute*5,  // timeout
		false,          // immediate (set to false to match PollImmediate behavior)
		func(ctx context.Context) (bool, error) {
			switch wlType {
			case Deployment:
				deployment, err := h.clientSet.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				return deployment.Generation <= deployment.Status.ObservedGeneration &&
					deployment.Status.UpdatedReplicas == *deployment.Spec.Replicas &&
					deployment.Status.Replicas == *deployment.Spec.Replicas &&
					deployment.Status.AvailableReplicas == *deployment.Spec.Replicas, nil

			case DaemonSet:
				daemonset, err := h.clientSet.AppsV1().DaemonSets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				return daemonset.Generation <= daemonset.Status.ObservedGeneration &&
					daemonset.Status.UpdatedNumberScheduled == daemonset.Status.DesiredNumberScheduled &&
					daemonset.Status.NumberReady == daemonset.Status.DesiredNumberScheduled, nil
			case StatefulSet:
				statefulset, err := h.clientSet.AppsV1().StatefulSets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				return statefulset.Generation <= statefulset.Status.ObservedGeneration &&
					statefulset.Status.UpdatedReplicas == *statefulset.Spec.Replicas &&
					statefulset.Status.ReadyReplicas == *statefulset.Spec.Replicas &&
					statefulset.Status.CurrentReplicas == *statefulset.Spec.Replicas, nil
			}
			return false, fmt.Errorf("unknown workload type: %s", wlType)
		})
}

package operator

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"regexp"

	arv1 "k8s.io/api/admissionregistration/v1"
	appsV1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacV1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"testing"
)

const nameSpace = "amazon-cloudwatch"

func TestK8s(t *testing.T) {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("error getting user home dir: %v", err)
	}
	kubeConfigPath := filepath.Join(userHomeDir, ".kube", "config")
	t.Logf("Using kubeconfig: %s\n", kubeConfigPath)

	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		t.Fatalf("error getting kubernetes config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		t.Fatalf("error getting kubernetes config: %v", err)
	}

	// Validating the "amazon-cloudwatch" namespace creation
	namespace, err := GetNameSpace(nameSpace, clientset)
	assert.NoError(t, err)
	assert.Equal(t, nameSpace, namespace.Name)

	//Validating the number of pods and status
	pods, err := ListPods(nameSpace, clientset)
	assert.NoError(t, err)
	assert.Len(t, pods.Items, 2)
	assert.Equal(t, v1.PodRunning, pods.Items[0].Status.Phase)
	assert.Equal(t, v1.PodRunning, pods.Items[1].Status.Phase)

	if validateAgentPodRegexMatch(pods.Items[0].Name) {
		assert.True(t, validateOperatorRegexMatch(pods.Items[1].Name))
	} else if validateOperatorRegexMatch(pods.Items[0].Name) {
		assert.True(t, validateAgentPodRegexMatch(pods.Items[1].Name))
	} else {
		assert.Fail(t, "failed to validate pod names")
	}

	//Validating the services
	services, err := ListServices(nameSpace, clientset)
	assert.NoError(t, err)
	assert.Len(t, services.Items, 4)
	assert.Equal(t, "cloudwatch-agent", services.Items[0].Name)
	assert.Equal(t, "cloudwatch-agent-headless", services.Items[1].Name)
	assert.Equal(t, "cloudwatch-agent-monitoring", services.Items[2].Name)
	assert.Equal(t, "cloudwatch-webhook-service", services.Items[3].Name)

	//Validating the Deployment
	deployments, err := ListDeployments(nameSpace, clientset)
	assert.NoError(t, err)
	assert.Len(t, deployments.Items, 1)
	assert.Equal(t, "cloudwatch-controller-manager", deployments.Items[0].Name)
	assert.Equal(t, appsV1.DeploymentAvailable, deployments.Items[0].Status.Conditions[0].Type)

	//Validating the Daemon Sets
	daemonSets, err := ListDaemonSets(nameSpace, clientset)
	assert.NoError(t, err)
	assert.Len(t, daemonSets.Items, 1)
	assert.Equal(t, "cloudwatch-agent", daemonSets.Items[0].Name)

	// Validating Service Accounts
	serviceAccounts, err := ListServiceAccounts(nameSpace, clientset)
	assert.NoError(t, err)
	assert.True(t, validateServiceAccount(serviceAccounts, "cloudwatch-controller-manager"))
	assert.True(t, validateServiceAccount(serviceAccounts, "cloudwatch-agent"))

	//Validating ClusterRoles
	clusterRoles, err := ListClusterRoles(clientset)
	assert.NoError(t, err)
	assert.True(t, validateClusterRoles(clusterRoles, "cloudwatch-agent-role"))
	assert.True(t, validateClusterRoles(clusterRoles, "cloudwatch-manager-role"))

	//Validating ClusterRoleBinding
	clusterRoleBindings, err := ListClusterRoleBindings(clientset)
	assert.NoError(t, err)
	assert.True(t, validateClusterRoleBindings(clusterRoleBindings, "cloudwatch-agent-role-binding"))
	assert.True(t, validateClusterRoleBindings(clusterRoleBindings, "cloudwatch-manager-rolebinding"))

	//Validating MutatingWebhookConfiguration
	mutatingWebhookConfigurations, err := ListMutatingWebhookConfigurations(clientset)
	assert.NoError(t, err)
	assert.True(t, validateMutatingWebhookConfiguration(mutatingWebhookConfigurations, "cloudwatch-mutating-webhook-configuration"))

	//Validating ValidatingWebhookConfiguration
	validatingWebhookConfigurations, err := ListValidatingWebhookConfigurations(clientset)
	assert.NoError(t, err)
	assert.True(t, validateValidatingWebhookConfiguration(validatingWebhookConfigurations, "cloudwatch-validating-webhook-configuration"))
}

func validateAgentPodRegexMatch(podName string) bool {
	agentPodMatch, _ := regexp.MatchString("cloudwatch-agent-*", podName)
	return agentPodMatch
}

func validateOperatorRegexMatch(podName string) bool {
	operatorPodMatch, _ := regexp.MatchString("cloudwatch-controller-manager-*", podName)
	return operatorPodMatch
}
func validateServiceAccount(serviceAccounts *v1.ServiceAccountList, serviceAccountName string) bool {
	for _, serviceAccount := range serviceAccounts.Items {
		if serviceAccount.Name == serviceAccountName {
			return true
		}
	}
	return false
}

func validateClusterRoles(clusterRoles *rbacV1.ClusterRoleList, clusterRoleName string) bool {
	for _, clusterRole := range clusterRoles.Items {
		if clusterRole.Name == clusterRoleName {
			return true
		}
	}
	return false
}

func validateClusterRoleBindings(clusterRoleBindings *rbacV1.ClusterRoleBindingList, clusterRoleBindingName string) bool {
	for _, clusterRoleBinding := range clusterRoleBindings.Items {
		if clusterRoleBinding.Name == clusterRoleBindingName {
			return true
		}
	}
	return false
}

func validateMutatingWebhookConfiguration(mutatingWebhookConfigurations *arv1.MutatingWebhookConfigurationList, mutatingWebhookConfigName string) bool {
	for _, mutatingWebhookConfiguration := range mutatingWebhookConfigurations.Items {
		if mutatingWebhookConfiguration.Name == mutatingWebhookConfigName {
			return true
		}
	}
	return false
}

func validateValidatingWebhookConfiguration(validatingWebhookConfigurations *arv1.ValidatingWebhookConfigurationList, validatingWebhookConfigurationName string) bool {
	for _, validatingWebhookConfiguration := range validatingWebhookConfigurations.Items {
		if validatingWebhookConfiguration.Name == validatingWebhookConfigurationName {
			return true
		}
	}
	return false
}

func ListPods(namespace string, client kubernetes.Interface) (*v1.PodList, error) {
	pods, err := client.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		err = fmt.Errorf("error getting pods: %v\n", err)
		return nil, err
	}
	return pods, nil
}

func GetNameSpace(namespace string, client kubernetes.Interface) (*v1.Namespace, error) {
	ns, err := client.CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})
	if err != nil {
		err = fmt.Errorf("error getting namespace: %v\n", err)
		return nil, err
	}
	return ns, nil
}

func ListServices(namespace string, client kubernetes.Interface) (*v1.ServiceList, error) {
	namespaces, err := client.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		err = fmt.Errorf("error getting Services: %v\n", err)
		return nil, err
	}
	return namespaces, nil
}

func ListDeployments(namespace string, client kubernetes.Interface) (*appsV1.DeploymentList, error) {
	deployments, err := client.AppsV1().Deployments(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		err = fmt.Errorf("error getting Deploymets: %v\n", err)
		return nil, err
	}
	return deployments, nil
}

func ListDaemonSets(namespace string, client kubernetes.Interface) (*appsV1.DaemonSetList, error) {
	daemonSets, err := client.AppsV1().DaemonSets(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		err = fmt.Errorf("error getting DaemonSets: %v\n", err)
		return nil, err
	}
	return daemonSets, nil
}

func ListServiceAccounts(namespace string, client kubernetes.Interface) (*v1.ServiceAccountList, error) {
	serviceAccounts, err := client.CoreV1().ServiceAccounts(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		err = fmt.Errorf("error getting ServiceAccounts: %v\n", err)
		return nil, err
	}
	return serviceAccounts, nil
}

func ListClusterRoles(client kubernetes.Interface) (*rbacV1.ClusterRoleList, error) {
	clusterRoles, err := client.RbacV1().ClusterRoles().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		err = fmt.Errorf("error getting ClusterRoles: %v\n", err)
		return nil, err
	}
	return clusterRoles, nil
}

func ListClusterRoleBindings(client kubernetes.Interface) (*rbacV1.ClusterRoleBindingList, error) {
	clusterRoleBindings, err := client.RbacV1().ClusterRoleBindings().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		err = fmt.Errorf("error getting ClusterRoleBindings: %v\n", err)
		return nil, err
	}
	return clusterRoleBindings, nil
}

func ListMutatingWebhookConfigurations(client kubernetes.Interface) (*arv1.MutatingWebhookConfigurationList, error) {
	mutatingWebhookConfigurations, err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		err = fmt.Errorf("error getting MutatingWebhookConfigurations: %v\n", err)
		return nil, err
	}
	return mutatingWebhookConfigurations, nil
}

func ListValidatingWebhookConfigurations(client kubernetes.Interface) (*arv1.ValidatingWebhookConfigurationList, error) {
	validatingWebhookConfigurations, err := client.AdmissionregistrationV1().ValidatingWebhookConfigurations().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		err = fmt.Errorf("error getting ValidatingWebhookConfigurations: %v\n", err)
		return nil, err
	}
	return validatingWebhookConfigurations, nil
}

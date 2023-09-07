package eks_addon

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"

	arv1 "k8s.io/api/admissionregistration/v1"
	appsV1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacV1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"testing"
)

const NAMESPACE = "amazon-cloudwatch"

func TestK8s(t *testing.T) {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("error getting user home dir: %v\n", err)
		os.Exit(1)
	}
	kubeConfigPath := filepath.Join(userHomeDir, ".kube", "config")
	fmt.Printf("Using kubeconfig: %s\n", kubeConfigPath)

	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		fmt.Printf("Error getting kubernetes config: %v\n", err)
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(kubeConfig)

	if err != nil {
		fmt.Printf("error getting kubernetes config: %v\n", err)
		os.Exit(1)
	}

	// Validating the "amazon-cloudwatch" namespace creation as part of EKS addon
	namespace, err := GetNameSpace(NAMESPACE, clientset)
	assert.NoError(t, err)
	assert.Equal(t, NAMESPACE, namespace.Name)

	//Validating the number of pods and status
	pods, err := ListPods(NAMESPACE, clientset)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(pods.Items))
	/*
		null_resource.validator (local-exec): Total Pods: 2
		null_resource.validator (local-exec): name: amazon-cloudwatch-agent-gzhs8 namespace:amazon-cloudwatch
		null_resource.validator (local-exec): name: amazon-cloudwatch-agent-operator-controller-manager-b7ccb9vdngt namespace:amazon-cloudwatch
	*/
	assert.Equal(t, v1.PodRunning, pods.Items[0].Status.Phase)
	assert.Equal(t, v1.PodRunning, pods.Items[1].Status.Phase)

	//Validating the services
	services, err := ListServices(NAMESPACE, clientset)
	assert.NoError(t, err)
	assert.Equal(t, 4, len(services.Items))
	/*
		null_resource.validator (local-exec): Total Services: 4
		null_resource.validator (local-exec): name: amazon-cloudwatch-agent namespace:amazon-cloudwatch
		null_resource.validator (local-exec): name: amazon-cloudwatch-agent-headless namespace:amazon-cloudwatch
		null_resource.validator (local-exec): name: amazon-cloudwatch-agent-monitoring namespace:amazon-cloudwatch
		null_resource.validator (local-exec): name: amazon-cloudwatch-agent-operator-webhook-service namespace:amazon-cloudwatch
	*/
	assert.Equal(t, "amazon-cloudwatch-agent", services.Items[0].Name)
	assert.Equal(t, "amazon-cloudwatch-agent-headless", services.Items[1].Name)
	assert.Equal(t, "amazon-cloudwatch-agent-monitoring", services.Items[2].Name)
	assert.Equal(t, "amazon-cloudwatch-agent-operator-webhook-service", services.Items[3].Name)

	//Validating the Deployment
	deployments, err := ListDeployments(NAMESPACE, clientset)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(deployments.Items))
	assert.Equal(t, "amazon-cloudwatch-agent-operator-controller-manager", deployments.Items[0].Name)
	assert.Equal(t, appsV1.DeploymentAvailable, deployments.Items[0].Status.Conditions[0].Type)

	//Validating the Daemon Sets
	daemonSets, err := ListDaemonSets(NAMESPACE, clientset)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(daemonSets.Items))
	assert.Equal(t, "amazon-cloudwatch-agent", daemonSets.Items[0].Name)

	// Validating Service Accounts
	serviceAccounts, err := ListServiceAccounts(NAMESPACE, clientset)
	assert.NoError(t, err)
	/*
		null_resource.validator (local-exec): name: amazon-cloudwatch-agent-operator-agent namespace:amazon-cloudwatch
		null_resource.validator (local-exec): name: amazon-cloudwatch-agent-operator-controller-manager namespace:amazon-cloudwatch
		null_resource.validator (local-exec): name: default namespace:amazon-cloudwatch
	*/
	assert.True(t, validateServiceAccount(serviceAccounts, "amazon-cloudwatch-agent-operator-controller-manager"))
	assert.True(t, validateServiceAccount(serviceAccounts, "amazon-cloudwatch-agent-operator-agent"))

	//Validating ClusterRoles
	clusterRoles, err := ListClusterRoles(clientset)
	assert.NoError(t, err)
	assert.True(t, validateClusterRoles(clusterRoles, "amazon-cloudwatch-agent-operator-manager-role"))
	assert.True(t, validateClusterRoles(clusterRoles, "amazon-cloudwatch-agent-operator-agent-role"))

	//Validating ClusterRoleBinding
	clusterRoleBindings, err := ListClusterRoleBindings(clientset)
	assert.NoError(t, err)
	assert.True(t, validateClusterRoleBindings(clusterRoleBindings, "amazon-cloudwatch-agent-operator-manager-rolebinding"))
	assert.True(t, validateClusterRoleBindings(clusterRoleBindings, "amazon-cloudwatch-agent-operator-agent-role-binding"))

	//Validating MutatingWebhookConfiguration
	mutatingWebhookConfigurations, err := ListMutatingWebhookConfigurations(clientset)
	assert.NoError(t, err)
	/*
		null_resource.validator (local-exec): name: amazon-cloudwatch-agent-operator-mutating-webhook-configuration namespace:
		null_resource.validator (local-exec): name: pod-identity-webhook namespace:
		null_resource.validator (local-exec): name: vpc-resource-mutating-webhook namespace:
	*/
	assert.Equal(t, "amazon-cloudwatch-agent-operator-mutating-webhook-configuration", mutatingWebhookConfigurations.Items[0].Name)
	assert.Equal(t, 3, len(mutatingWebhookConfigurations.Items[0].Webhooks))

	//Validating ValidatingWebhookConfiguration
	validatingWebhookConfigurations, err := ListValidatingWebhookConfigurations(clientset)
	assert.NoError(t, err)
	/*
		null_resource.validator (local-exec): name: amazon-cloudwatch-agent-operator-validating-webhook-configuration namespace:
		null_resource.validator (local-exec): name: eks-aws-auth-configmap-validation-webhook namespace:
		null_resource.validator (local-exec): name: vpc-resource-validating-webhook namespace:
	*/
	assert.Equal(t, "amazon-cloudwatch-agent-operator-validating-webhook-configuration", validatingWebhookConfigurations.Items[0].Name)
	assert.Equal(t, 4, len(validatingWebhookConfigurations.Items[0].Webhooks))
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
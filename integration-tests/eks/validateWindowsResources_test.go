// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build windows_tests
// +build windows_tests

package eks_addon

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	agentNameWindows    = "cloudwatch-agent-windows"
	podNameWindowsRegex = "(" + agentName + "|" + addOnName + "|fluent-bit-windows)-*"
)

func TestAddonOnEKsWindows(t *testing.T) {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("error getting user home dir: %v\n", err)
	}
	kubeConfigPath := filepath.Join(userHomeDir, ".kube", "config")
	t.Logf("Using kubeconfig: %s\n", kubeConfigPath)

	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		t.Fatalf("Error getting kubernetes config: %v\n", err)
	}

	clientSet, err := kubernetes.NewForConfig(kubeConfig)

	if err != nil {
		t.Fatalf("error getting kubernetes config: %v\n", err)
	}

	// Validating the "amazon-cloudwatch" namespace creation as part of EKS addon
	namespace, err := GetNameSpace(nameSpace, clientSet)
	assert.NoError(t, err)
	assert.Equal(t, nameSpace, namespace.Name)

	//Validating the number of pods and status
	pods, err := ListPods(nameSpace, clientSet)
	assert.NoError(t, err)
	assert.Len(t, pods.Items, 3)
	for _, pod := range pods.Items {
		fmt.Println("pod name: " + pod.Name + " namespace:" + pod.Namespace)
		assert.Equal(t, v1.PodRunning, pod.Status.Phase)
		// matches
		// - cloudwatch-agent-windows-*
		// - fluent-bit-windows-*
		if match, _ := regexp.MatchString(podNameWindowsRegex, pod.Name); !match {
			assert.Fail(t, "Cluster Pods are not created correctly")
		}
	}

	//Validating the Daemon Sets
	daemonSets, err := ListDaemonSets(nameSpace, clientSet)
	assert.NoError(t, err)
	assert.Len(t, daemonSets.Items, 2)
	for _, daemonSet := range daemonSets.Items {
		fmt.Println("daemonSet name: " + daemonSet.Name + " namespace:" + daemonSet.Namespace)
		// matches
		// - cloudwatch-agent
		// - fluent-bit
		if match, _ := regexp.MatchString(agentNameWindows+"|fluent-bit-windows", daemonSet.Name); !match {
			assert.Fail(t, "DaemonSet is created correctly")
		}
	}
}

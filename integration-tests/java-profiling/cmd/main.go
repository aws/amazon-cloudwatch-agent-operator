// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Command validates the Java profiling injection on the running pods of a cluster.
// Usage: go run ./integration-tests/java-profiling/cmd <namespace> <expectations.json>
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/aws/amazon-cloudwatch-agent-operator/integration-tests/java-profiling/validate"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("usage: <namespace> <expectations.json>")
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2]); err != nil {
		fmt.Println(err)
		fmt.Println("Java Profiling Injection Test: FAIL")
		os.Exit(1)
	}
	fmt.Println("Java Profiling Injection Test: PASS")
}

func run(namespace, expectationsPath string) error {
	exp, err := validate.Load(expectationsPath)
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error getting user home dir: %w", err)
	}
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", filepath.Join(home, ".kube", "config"))
	if err != nil {
		return fmt.Errorf("error getting kubernetes config: %w", err)
	}
	clientSet, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("error creating kubernetes client: %w", err)
	}
	pods, err := clientSet.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: exp.LabelSelector})
	if err != nil {
		return fmt.Errorf("error listing pods: %w", err)
	}
	checked := 0
	var failed []string
	for _, pod := range pods.Items {
		if pod.DeletionTimestamp != nil || pod.Status.Phase != corev1.PodRunning {
			continue
		}
		checked++
		problems := validate.Pod(pod, exp)
		for _, p := range problems {
			fmt.Printf("Pod %s: %s\n", pod.Name, p)
		}
		if len(problems) > 0 {
			failed = append(failed, pod.Name)
		} else {
			fmt.Printf("Pod %s: matches %s\n", pod.Name, expectationsPath)
		}
	}
	if checked == 0 {
		return fmt.Errorf("no running pods in %s match %q", namespace, exp.LabelSelector)
	}
	if len(failed) > 0 {
		return fmt.Errorf("%d of %d pods do not match: %v", len(failed), checked, failed)
	}
	return nil
}

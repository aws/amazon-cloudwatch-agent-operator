// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"context"
	"fmt"
	"time"

	appsV1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

const TimoutDuration = 3 * time.Minute
const TimeBetweenRetries = 5 * time.Second

// WaitForNewPodCreation takes in a resource either Deployment, DaemonSet, or StatefulSet wait until it is in running stage
func WaitForNewPodCreation(clientSet *kubernetes.Clientset, resource interface{}, startTime time.Time) error {
	start := time.Now()
	for {
		if time.Since(start) > TimoutDuration {
			return fmt.Errorf("timed out waiting for new pod creation")
		}
		namespace := ""
		labelSelector := ""
		switch r := resource.(type) {
		case *appsV1.Deployment:
			namespace = r.Namespace
			labelSelector = labels.Set(r.Spec.Selector.MatchLabels).AsSelector().String()
		case *appsV1.DaemonSet:
			namespace = r.Namespace
			labelSelector = labels.Set(r.Spec.Selector.MatchLabels).AsSelector().String()
		case *appsV1.StatefulSet:
			namespace = r.Namespace
			labelSelector = labels.Set(r.Spec.Selector.MatchLabels).AsSelector().String()
		default:
			return fmt.Errorf("unsupported resource type")
		}

		newPods, _ := clientSet.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})

		for _, pod := range newPods.Items {
			if pod.CreationTimestamp.Time.After(startTime) && pod.Status.Phase == v1.PodRunning {
				fmt.Printf("Operator pod %s created after start time and is running\n", pod.Name)
				return nil
			} else if pod.CreationTimestamp.Time.After(startTime) {
				fmt.Printf("Operator pod %s created after start time but is not in running stage\n", pod.Name)
			}
		}

		time.Sleep(TimeBetweenRetries)
	}
}

func CheckIfPodsAreRunning(pods *v1.PodList) bool {
	allRunning := true
	for _, pod := range pods.Items {
		if pod.Status.Phase != v1.PodRunning {
			allRunning = false
			break
		}
	}
	if !allRunning {
		fmt.Println("Not all pods are in the Running phase")
	}
	fmt.Println("All pods are in the Running phase")
	return true
}

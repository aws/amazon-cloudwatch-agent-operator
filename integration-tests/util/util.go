// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"context"
	"encoding/json"
	"fmt"
	"k8s.io/apimachinery/pkg/util/wait"
	"time"

	appsV1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

const TimoutDuration = 1 * time.Minute
const TimeBetweenRetries = 2 * time.Second

func WaitForNewPodCreation(clientSet *kubernetes.Clientset, resource interface{}, startTime time.Time) error {
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
	return wait.PollUntilContextTimeout(context.TODO(), TimeBetweenRetries, TimoutDuration, true, func(ctx context.Context) (bool, error) {
		newPods, err := clientSet.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			return false, fmt.Errorf("failed to list pods: %v", err)
		}
		for _, pod := range newPods.Items {
			if pod.CreationTimestamp.Time.After(startTime.Add(-time.Second)) {
				if pod.Status.Phase == v1.PodRunning {
					isReady := isPodReady(&pod)
					if isReady {
						return true, nil
					}
				}
			}
		}
		return false, nil
	})
}

// Helper function to check if pod is ready
func isPodReady(pod *v1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == v1.PodReady {
			return condition.Status == v1.ConditionTrue
		}
	}
	return false
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

func PrettyPrint(data interface{}) {
	b, _ := json.MarshalIndent(data, "", "  ")
	fmt.Println(string(b))
}

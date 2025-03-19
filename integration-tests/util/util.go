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

const TimoutDuration = 2 * time.Minute
const TimeBetweenRetries = 2 * time.Second

func WaitForNewPodCreation(clientSet *kubernetes.Clientset, resource interface{}, startTime time.Time) error {
	// 1. Use wait.PollImmediate instead of manual polling
	// 2. Move type switch outside the polling loop
	fmt.Println("start time: ", startTime)
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
	return wait.PollImmediate(TimeBetweenRetries, TimoutDuration, func() (bool, error) {
		// 3. Handle list error
		newPods, err := clientSet.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		// get list of pod names
		//podNames := make([]string, 0)
		//for _, pod := range newPods.Items {
		//	podNames = append(podNames, pod.Name)
		//}
		//
		//fmt.Println(podNames)
		if err != nil {
			return false, fmt.Errorf("failed to list pods: %v", err)
		}

		// 4. Check for pod readiness, not just running
		for _, pod := range newPods.Items {
			//fmt.Println("Pod Name: ", pod.Name, ", pod.Creation: ", pod.CreationTimestamp)
			if pod.CreationTimestamp.Time.After(startTime.Add(-time.Second)) {
				if pod.Status.Phase == v1.PodRunning {
					// 5. Check if pod is ready
					isReady := isPodReady(&pod)
					if isReady {
						fmt.Printf("pod %s created after start time and is ready\n", pod.Name)
						return true, nil
					}
					//fmt.Printf("pod %s is running but not ready\n", pod.Name)
				} else {
					//fmt.Printf("pod %s created after start time but is in %s state\n",
					//	pod.Name, pod.Status.Phase)
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

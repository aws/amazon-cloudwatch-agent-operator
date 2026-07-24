// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/amazon-cloudwatch-agent-operator/cmd/amazon-cloudwatch-agent-target-allocator/allocation"
)

var logger = logf.Log.WithName("collector-unit-tests")

func getTestClient() (Client, watch.Interface) {
	kubeClient := Client{
		k8sClient: fake.NewSimpleClientset(),
		close:     make(chan struct{}),
		log:       logger,
	}

	labelMap := map[string]string{
		"app.kubernetes.io/instance":   "default.test",
		"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
	}

	opts := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labelMap).String(),
	}
	watcher, err := kubeClient.k8sClient.CoreV1().Pods("test-ns").Watch(context.Background(), opts)
	if err != nil {
		fmt.Printf("failed to setup a Collector Pod watcher: %v", err)
		os.Exit(1)
	}
	return kubeClient, watcher
}

func pod(name string) *v1.Pod {
	labelSet := make(map[string]string)
	labelSet["app.kubernetes.io/instance"] = "default.test"
	labelSet["app.kubernetes.io/managed-by"] = "amazon-cloudwatch-agent-operator"

	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "test-ns",
			Labels:    labelSet,
		},
		Spec: v1.PodSpec{
			NodeName: name + "-node",
		},
	}
}

func Test_runWatch(t *testing.T) {
	type args struct {
		kubeFn       func(t *testing.T, client Client, group *sync.WaitGroup)
		collectorMap map[string]*allocation.Collector
	}
	tests := []struct {
		name string
		args args
		want map[string]*allocation.Collector
	}{
		{
			name: "pod add",
			args: args{
				kubeFn: func(t *testing.T, client Client, group *sync.WaitGroup) {
					for _, k := range []string{"test-pod1", "test-pod2", "test-pod3"} {
						p := pod(k)
						group.Add(1)
						_, err := client.k8sClient.CoreV1().Pods("test-ns").Create(context.Background(), p, metav1.CreateOptions{})
						assert.NoError(t, err)
					}
				},
				collectorMap: map[string]*allocation.Collector{},
			},
			want: map[string]*allocation.Collector{
				"test-pod1": {
					Name:     "test-pod1",
					NodeName: "test-pod1-node",
				},
				"test-pod2": {
					Name:     "test-pod2",
					NodeName: "test-pod2-node",
				},
				"test-pod3": {
					Name:     "test-pod3",
					NodeName: "test-pod3-node",
				},
			},
		},
		{
			name: "pod delete",
			args: args{
				kubeFn: func(t *testing.T, client Client, group *sync.WaitGroup) {
					for _, k := range []string{"test-pod2", "test-pod3"} {
						group.Add(1)
						err := client.k8sClient.CoreV1().Pods("test-ns").Delete(context.Background(), k, metav1.DeleteOptions{})
						assert.NoError(t, err)
					}
				},
				collectorMap: map[string]*allocation.Collector{
					"test-pod1": {
						Name: "test-pod1",
					},
					"test-pod2": {
						Name: "test-pod2",
					},
					"test-pod3": {
						Name: "test-pod3",
					},
				},
			},
			want: map[string]*allocation.Collector{
				"test-pod1": {
					Name:     "test-pod1",
					NodeName: "test-pod1-node",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kubeClient, watcher := getTestClient()
			defer func() {
				close(kubeClient.close)
				watcher.Stop()
			}()
			var wg sync.WaitGroup
			actual := make(map[string]*allocation.Collector)
			for _, k := range tt.args.collectorMap {
				p := pod(k.Name)
				_, err := kubeClient.k8sClient.CoreV1().Pods("test-ns").Create(context.Background(), p, metav1.CreateOptions{})
				wg.Add(1)
				assert.NoError(t, err)
			}
			go runWatch(context.Background(), &kubeClient, watcher.ResultChan(), map[string]*allocation.Collector{}, func(colMap map[string]*allocation.Collector) {
				actual = colMap
				wg.Done()
			})

			tt.args.kubeFn(t, kubeClient, &wg)
			wg.Wait()

			assert.Len(t, actual, len(tt.want))
			assert.Equal(t, actual, tt.want)
		})
	}
}

// this tests runWatch in the case of watcher channel closing and watcher timing out.
func Test_closeChannel(t *testing.T) {
	tests := []struct {
		description    string
		isCloseChannel bool
		timeout        time.Duration
	}{
		{
			// event is triggered by channel closing.
			description:    "close_channel",
			isCloseChannel: true,
			// channel should be closed before this timeout occurs
			timeout: 10 * time.Second,
		},
		{
			// event triggered by timeout.
			description:    "watcher_timeout",
			isCloseChannel: false,
			timeout:        0 * time.Second,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			kubeClient, watcher := getTestClient()

			defer func() {
				close(kubeClient.close)
				watcher.Stop()
			}()
			var wg sync.WaitGroup
			wg.Add(1)
			terminated := false

			go func(watcher watch.Interface) {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), tc.timeout)
				defer cancel()
				if msg := runWatch(ctx, &kubeClient, watcher.ResultChan(), map[string]*allocation.Collector{}, func(colMap map[string]*allocation.Collector) {}); msg != "" {
					terminated = true
					return
				}
			}(watcher)

			if tc.isCloseChannel {
				// stop pod watcher to trigger event.
				watcher.Stop()
			}
			wg.Wait()
			assert.False(t, terminated)
		})
	}
}


// Test_runWatch_UnscheduledThenScheduled verifies an unscheduled collector pod
// (empty NodeName) is skipped when Added, then registered with its node once a
// Modified event reports the assignment. This is the DaemonSet-rollout fix: the
// per-node strategy must pick up a collector's node without a TA restart.
func Test_runWatch_UnscheduledThenScheduled(t *testing.T) {
	kubeClient, watcher := getTestClient()
	defer func() {
		close(kubeClient.close)
		watcher.Stop()
	}()

	var wg sync.WaitGroup
	actual := make(map[string]*allocation.Collector)
	go runWatch(context.Background(), &kubeClient, watcher.ResultChan(), map[string]*allocation.Collector{}, func(colMap map[string]*allocation.Collector) {
		actual = colMap
		wg.Done()
	})

	// Added while unscheduled (no NodeName): must be skipped.
	wg.Add(1)
	p := pod("test-pod1")
	p.Spec.NodeName = ""
	created, err := kubeClient.k8sClient.CoreV1().Pods("test-ns").Create(context.Background(), p, metav1.CreateOptions{})
	assert.NoError(t, err)
	wg.Wait()
	assert.Empty(t, actual, "unscheduled pod (no NodeName) must not be registered")

	// Scheduled later: a Modified event carrying the node must register it.
	wg.Add(1)
	created.Spec.NodeName = "test-pod1-node"
	_, err = kubeClient.k8sClient.CoreV1().Pods("test-ns").Update(context.Background(), created, metav1.UpdateOptions{})
	assert.NoError(t, err)
	wg.Wait()

	assert.Equal(t, map[string]*allocation.Collector{
		"test-pod1": {Name: "test-pod1", NodeName: "test-pod1-node"},
	}, actual)
}


// Test_runWatch_NonPodEventRestarts verifies runWatch restarts (returns) when an
// event carries an object that is not a Pod, rather than panicking on the type
// assertion.
func Test_runWatch_NonPodEventRestarts(t *testing.T) {
	kubeClient, watcher := getTestClient()
	defer func() {
		close(kubeClient.close)
		watcher.Stop()
	}()

	events := make(chan watch.Event, 1)
	events <- watch.Event{Type: watch.Added, Object: &v1.ConfigMap{}}
	msg := runWatch(context.Background(), &kubeClient, events, map[string]*allocation.Collector{},
		func(map[string]*allocation.Collector) {})
	assert.Equal(t, "", msg)
}

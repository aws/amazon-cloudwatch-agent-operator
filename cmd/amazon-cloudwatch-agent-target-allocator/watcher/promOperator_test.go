// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package watcher

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	fakemonitoringclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/prometheus-operator/prometheus-operator/pkg/informers"
	"github.com/prometheus-operator/prometheus-operator/pkg/prometheus"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	kubeDiscovery "github.com/prometheus/prometheus/discovery/kubernetes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name           string
		serviceMonitor *monitoringv1.ServiceMonitor
		podMonitor     *monitoringv1.PodMonitor
		want           *promconfig.Config
		wantErr        bool
	}{
		{
			name: "simple test",
			serviceMonitor: &monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "simple",
					Namespace: "test",
				},
				Spec: monitoringv1.ServiceMonitorSpec{
					JobLabel: "test",
					Endpoints: []monitoringv1.Endpoint{
						{
							Port: "web",
						},
					},
				},
			},
			podMonitor: &monitoringv1.PodMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "simple",
					Namespace: "test",
				},
				Spec: monitoringv1.PodMonitorSpec{
					JobLabel: "test",
					PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{
						{
							Port: ptr.To("web"),
						},
					},
				},
			},
			want: &promconfig.Config{
				ScrapeConfigs: []*promconfig.ScrapeConfig{
					{
						JobName:         "serviceMonitor/test/simple/0",
						ScrapeInterval:  model.Duration(30 * time.Second),
						ScrapeTimeout:   model.Duration(10 * time.Second),
						HonorTimestamps: true,
						HonorLabels:     false,
						Scheme:          "http",
						MetricsPath:     "/metrics",
						ServiceDiscoveryConfigs: []discovery.Config{
							&kubeDiscovery.SDConfig{
								Role: "endpoints",
								NamespaceDiscovery: kubeDiscovery.NamespaceDiscovery{
									Names:               []string{"test"},
									IncludeOwnNamespace: false,
								},
								HTTPClientConfig: config.DefaultHTTPClientConfig,
							},
						},
						HTTPClientConfig: config.DefaultHTTPClientConfig,
					},
					{
						JobName:         "podMonitor/test/simple/0",
						ScrapeInterval:  model.Duration(30 * time.Second),
						ScrapeTimeout:   model.Duration(10 * time.Second),
						HonorTimestamps: true,
						HonorLabels:     false,
						Scheme:          "http",
						MetricsPath:     "/metrics",
						ServiceDiscoveryConfigs: []discovery.Config{
							&kubeDiscovery.SDConfig{
								Role: "pod",
								NamespaceDiscovery: kubeDiscovery.NamespaceDiscovery{
									Names:               []string{"test"},
									IncludeOwnNamespace: false,
								},
								HTTPClientConfig: config.DefaultHTTPClientConfig,
							},
						},
						HTTPClientConfig: config.DefaultHTTPClientConfig,
					},
				},
			},
		},
		{
			name: "basic auth (serviceMonitor)",
			serviceMonitor: &monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "auth",
					Namespace: "test",
				},
				Spec: monitoringv1.ServiceMonitorSpec{
					JobLabel: "auth",
					Endpoints: []monitoringv1.Endpoint{
						{
							Port: "web",
							HTTPConfigWithProxyAndTLSFiles: monitoringv1.HTTPConfigWithProxyAndTLSFiles{
								HTTPConfigWithTLSFiles: monitoringv1.HTTPConfigWithTLSFiles{
									HTTPConfigWithoutTLS: monitoringv1.HTTPConfigWithoutTLS{
										BasicAuth: &monitoringv1.BasicAuth{
											Username: v1.SecretKeySelector{
												LocalObjectReference: v1.LocalObjectReference{
													Name: "basic-auth",
												},
												Key: "username",
											},
											Password: v1.SecretKeySelector{
												LocalObjectReference: v1.LocalObjectReference{
													Name: "basic-auth",
												},
												Key: "password",
											},
										},
									},
								},
							},
						},
					},
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "auth",
						},
					},
				},
			},
			want: &promconfig.Config{
				GlobalConfig: promconfig.GlobalConfig{},
				ScrapeConfigs: []*promconfig.ScrapeConfig{
					{
						JobName:         "serviceMonitor/test/auth/0",
						ScrapeInterval:  model.Duration(30 * time.Second),
						ScrapeTimeout:   model.Duration(10 * time.Second),
						HonorTimestamps: true,
						HonorLabels:     false,
						Scheme:          "http",
						MetricsPath:     "/metrics",
						ServiceDiscoveryConfigs: []discovery.Config{
							&kubeDiscovery.SDConfig{
								Role: "endpoints",
								NamespaceDiscovery: kubeDiscovery.NamespaceDiscovery{
									Names:               []string{"test"},
									IncludeOwnNamespace: false,
								},
								HTTPClientConfig: config.DefaultHTTPClientConfig,
							},
						},
						HTTPClientConfig: config.HTTPClientConfig{
							FollowRedirects: true,
							EnableHTTP2:     true,
							BasicAuth: &config.BasicAuth{
								Username: "admin",
								Password: "password",
							},
						},
					},
				},
			},
		},
		{
			name: "bearer token (podMonitor)",
			podMonitor: &monitoringv1.PodMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bearer",
					Namespace: "test",
				},
				Spec: monitoringv1.PodMonitorSpec{
					JobLabel: "bearer",
					PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{
						{
							Port: ptr.To("web"),
							HTTPConfigWithProxy: monitoringv1.HTTPConfigWithProxy{
								HTTPConfig: monitoringv1.HTTPConfig{
									HTTPConfigWithoutTLS: monitoringv1.HTTPConfigWithoutTLS{
										Authorization: &monitoringv1.SafeAuthorization{
											Type: "Bearer",
											Credentials: &v1.SecretKeySelector{
												LocalObjectReference: v1.LocalObjectReference{
													Name: "bearer",
												},
												Key: "token",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: &promconfig.Config{
				GlobalConfig: promconfig.GlobalConfig{},
				ScrapeConfigs: []*promconfig.ScrapeConfig{
					{
						JobName:         "podMonitor/test/bearer/0",
						ScrapeInterval:  model.Duration(30 * time.Second),
						ScrapeTimeout:   model.Duration(10 * time.Second),
						HonorTimestamps: true,
						HonorLabels:     false,
						Scheme:          "http",
						MetricsPath:     "/metrics",
						ServiceDiscoveryConfigs: []discovery.Config{
							&kubeDiscovery.SDConfig{
								Role: "pod",
								NamespaceDiscovery: kubeDiscovery.NamespaceDiscovery{
									Names:               []string{"test"},
									IncludeOwnNamespace: false,
								},
								HTTPClientConfig: config.DefaultHTTPClientConfig,
							},
						},
						HTTPClientConfig: config.HTTPClientConfig{
							FollowRedirects: true,
							EnableHTTP2:     true,
							Authorization: &config.Authorization{
								Type:        "Bearer",
								Credentials: "bearer-token",
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := getTestPrometheusCRWatcher(t, tt.serviceMonitor, tt.podMonitor)
			defer func() { _ = w.Close() }()

			// Start both informers via the per-type lifecycle (both CRDs present).
			notifyEvents := make(chan struct{}, 1)
			require.NoError(t, w.startMonitorInformer(monitoringv1.ServiceMonitorName, notifyEvents))
			require.NoError(t, w.startMonitorInformer(monitoringv1.PodMonitorName, notifyEvents))

			got, err := w.LoadConfig(context.Background())
			require.NoError(t, err)

			sanitizeScrapeConfigsForTest(got.ScrapeConfigs)
			assert.Equal(t, tt.want.ScrapeConfigs, got.ScrapeConfigs)
		})
	}
}

func TestRateLimit(t *testing.T) {
	var err error
	serviceMonitor := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "simple",
			Namespace: "test",
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			JobLabel: "test",
			Endpoints: []monitoringv1.Endpoint{
				{
					Port: "web",
				},
			},
		},
	}
	events := make(chan Event, 1)
	eventInterval := 5 * time.Millisecond

	w := getTestPrometheusCRWatcher(t, nil, nil)
	defer func() { _ = w.Close() }()
	w.eventInterval = eventInterval

	go func() {
		watchErr := w.Watch(events, make(chan error))
		require.NoError(t, watchErr)
	}()
	// we don't have a simple way to wait for the watch to actually add event handlers to the informer,
	// instead, we just update a ServiceMonitor periodically and wait until we get a notification
	_, err = w.kubeMonitoringClient.MonitoringV1().ServiceMonitors("test").Create(context.Background(), serviceMonitor, metav1.CreateOptions{})
	require.NoError(t, err)

	// wait for the watcher to start the ServiceMonitor informer and finish its
	// startup loop (both CRDs present => both informers running) so the
	// rate-limited event sender is active before we measure event timing.
	require.Eventually(t, func() bool {
		w.informersMtx.RLock()
		defer w.informersMtx.RUnlock()
		sm, ok := w.informers[monitoringv1.ServiceMonitorName]
		return ok && sm.HasSynced() && len(w.informers) == 2
	}, 5*time.Second, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		_, createErr := w.kubeMonitoringClient.MonitoringV1().ServiceMonitors("test").Update(context.Background(), serviceMonitor, metav1.UpdateOptions{})
		if createErr != nil {
			return false
		}
		select {
		case <-events:
			return true
		default:
			return false
		}
	}, eventInterval*2, time.Millisecond)

	// it's difficult to measure the rate precisely
	// what we do, is send two updates, and then assert that the elapsed time is between eventInterval and 3*eventInterval
	startTime := time.Now()
	_, err = w.kubeMonitoringClient.MonitoringV1().ServiceMonitors("test").Update(context.Background(), serviceMonitor, metav1.UpdateOptions{})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		select {
		case <-events:
			return true
		default:
			return false
		}
	}, eventInterval*2, time.Millisecond)
	_, err = w.kubeMonitoringClient.MonitoringV1().ServiceMonitors("test").Update(context.Background(), serviceMonitor, metav1.UpdateOptions{})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		select {
		case <-events:
			return true
		default:
			return false
		}
	}, eventInterval*2, time.Millisecond)
	elapsedTime := time.Since(startTime)
	assert.Less(t, eventInterval, elapsedTime)
	assert.GreaterOrEqual(t, eventInterval*3, elapsedTime)

}

// getTestPrometheusCRWatcher creates a test instance of PrometheusCRWatcher with
// fake clients and test secrets. Both the ServiceMonitor and PodMonitor CRDs are
// registered in the fake apiextensions client, so the watcher behaves as if both
// CRDs are present. Use getTestPrometheusCRWatcherWithCRDs to control CRD presence.
func getTestPrometheusCRWatcher(t *testing.T, sm *monitoringv1.ServiceMonitor, pm *monitoringv1.PodMonitor) *PrometheusCRWatcher {
	return getTestPrometheusCRWatcherWithCRDs(t, sm, pm, true, true)
}

// crdFor builds a minimal CustomResourceDefinition object for the given
// monitoring resource (e.g. "servicemonitors"), matching the names the watcher
// keys on.
func crdFor(resourceName string) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: resourceName + "." + monitoringv1.SchemeGroupVersion.Group,
		},
	}
}

func getTestPrometheusCRWatcherWithCRDs(t *testing.T, sm *monitoringv1.ServiceMonitor, pm *monitoringv1.PodMonitor, smCRD, pmCRD bool) *PrometheusCRWatcher {
	mClient := fakemonitoringclient.NewSimpleClientset() //nolint:staticcheck // NewClientset causes structured merge diff schema errors in tests
	if sm != nil {
		_, err := mClient.MonitoringV1().ServiceMonitors("test").Create(context.Background(), sm, metav1.CreateOptions{})
		if err != nil {
			t.Fatal(t, err)
		}
	}
	if pm != nil {
		_, err := mClient.MonitoringV1().PodMonitors("test").Create(context.Background(), pm, metav1.CreateOptions{})
		if err != nil {
			t.Fatal(t, err)
		}
	}

	var crdObjects []runtime.Object
	if smCRD {
		crdObjects = append(crdObjects, crdFor(monitoringv1.ServiceMonitorName))
	}
	if pmCRD {
		crdObjects = append(crdObjects, crdFor(monitoringv1.PodMonitorName))
	}
	crdClient := apiextensionsfake.NewSimpleClientset(crdObjects...)

	k8sClient := fake.NewSimpleClientset()
	_, err := k8sClient.CoreV1().Secrets("test").Create(context.Background(), &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "basic-auth",
			Namespace: "test",
		},
		Data: map[string][]byte{"username": []byte("admin"), "password": []byte("password")},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(t, err)
	}
	_, err = k8sClient.CoreV1().Secrets("test").Create(context.Background(), &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bearer",
			Namespace: "test",
		},
		Data: map[string][]byte{"token": []byte("bearer-token")},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(t, err)
	}

	prom := &monitoringv1.Prometheus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: monitoringv1.PrometheusSpec{
			CommonPrometheusFields: monitoringv1.CommonPrometheusFields{
				ScrapeInterval: monitoringv1.Duration("30s"),
			},
			EvaluationInterval: monitoringv1.Duration("30s"),
		},
	}

	generator, err := prometheus.NewConfigGenerator(slog.Default(), prom, prometheus.WithEndpointSliceSupport())
	if err != nil {
		t.Fatal(t, err)
	}

	return &PrometheusCRWatcher{
		logger:                 logr.Discard(),
		kubeMonitoringClient:   mClient,
		k8sClient:              k8sClient,
		crdClient:              crdClient,
		informers:              map[string]*informers.ForResource{},
		informerStopChannels:   map[string]chan struct{}{},
		configGenerator:        generator,
		prom:                   prom,
		serviceMonitorSelector: getSelector(nil),
		podMonitorSelector:     getSelector(nil),
		stopChannel:            make(chan struct{}),
	}
}

// Remove relable configs fields from scrape configs for testing,
// since these are mutated and tested down the line with the hook(s).
// Also normalizes library-default fields that change across prometheus versions.
func sanitizeScrapeConfigsForTest(scs []*promconfig.ScrapeConfig) {
	for _, sc := range scs {
		sc.RelabelConfigs = nil
		sc.MetricRelabelConfigs = nil
		sc.ScrapeProtocols = nil
		sc.ScrapeNativeHistograms = nil
		sc.AlwaysScrapeClassicHistograms = nil
		sc.ConvertClassicHistogramsToNHCB = nil
		sc.EnableCompression = false
		sc.MetricNameValidationScheme = 0
		sc.MetricNameEscapingScheme = ""
		sc.ExtraScrapeMetrics = nil
	}
}

// runningInformers returns the set of monitoring resource names whose informers
// are currently active.
func runningInformers(w *PrometheusCRWatcher) map[string]bool {
	w.informersMtx.RLock()
	defer w.informersMtx.RUnlock()
	out := map[string]bool{}
	for name := range w.informers {
		out[name] = true
	}
	return out
}

// TestWatchStartsHealthyWithoutCRDs verifies the TA does not error when neither
// the ServiceMonitor nor PodMonitor CRD exists: Watch returns no error, no
// informers are started, and LoadConfig yields a config with no scrape jobs.
func TestWatchStartsHealthyWithoutCRDs(t *testing.T) {
	w := getTestPrometheusCRWatcherWithCRDs(t, nil, nil, false, false)
	w.eventInterval = 5 * time.Millisecond
	defer func() { _ = w.Close() }()

	watchDone := make(chan error, 1)
	go func() { watchDone <- w.Watch(make(chan Event, 1), make(chan error, 1)) }()

	// Give the CRD watch time to sync and (not) start anything.
	require.Never(t, func() bool {
		return len(runningInformers(w)) > 0
	}, 200*time.Millisecond, 20*time.Millisecond)

	cfg, err := w.LoadConfig(context.Background())
	require.NoError(t, err)
	assert.Empty(t, cfg.ScrapeConfigs)

	// Watch must still be running (resilient), not have returned an error.
	select {
	case err := <-watchDone:
		t.Fatalf("Watch exited unexpectedly with: %v", err)
	default:
	}
}

// TestWatchPerTypeIndependence verifies that with only the ServiceMonitor CRD
// present, the SM informer starts and the PM informer does not.
func TestWatchPerTypeIndependence(t *testing.T) {
	w := getTestPrometheusCRWatcherWithCRDs(t, nil, nil, true, false)
	w.eventInterval = 5 * time.Millisecond
	defer func() { _ = w.Close() }()

	go func() { _ = w.Watch(make(chan Event, 1), make(chan error, 1)) }()

	require.Eventually(t, func() bool {
		return runningInformers(w)[monitoringv1.ServiceMonitorName]
	}, 5*time.Second, 10*time.Millisecond)

	// PodMonitor CRD is absent, so its informer must never start.
	require.Never(t, func() bool {
		return runningInformers(w)[monitoringv1.PodMonitorName]
	}, 200*time.Millisecond, 20*time.Millisecond)
}

// TestWatchStartsInformerWhenCRDCreated verifies the TA begins watching a type
// when its CRD is created at runtime — no restart required.
func TestWatchStartsInformerWhenCRDCreated(t *testing.T) {
	w := getTestPrometheusCRWatcherWithCRDs(t, nil, nil, false, false)
	w.eventInterval = 5 * time.Millisecond
	defer func() { _ = w.Close() }()

	go func() { _ = w.Watch(make(chan Event, 1), make(chan error, 1)) }()

	// Nothing running initially.
	require.Never(t, func() bool {
		return len(runningInformers(w)) > 0
	}, 200*time.Millisecond, 20*time.Millisecond)

	// Create the ServiceMonitor CRD after startup.
	_, err := w.crdClient.ApiextensionsV1().CustomResourceDefinitions().Create(
		context.Background(), crdFor(monitoringv1.ServiceMonitorName), metav1.CreateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return runningInformers(w)[monitoringv1.ServiceMonitorName]
	}, 5*time.Second, 10*time.Millisecond)
}

// TestStopMonitorInformerDropsType verifies the stop path used when a CRD is
// removed at runtime: the informer is stopped, dropped from the active set (so
// LoadConfig no longer emits its targets), and a reload is signalled. This is
// the logic invoked by the CRD watch's DeleteFunc; it is exercised directly so
// the assertion does not depend on fake-clientset delete-watch delivery.
func TestStopMonitorInformerDropsType(t *testing.T) {
	w := getTestPrometheusCRWatcherWithCRDs(t, nil, nil, true, false)
	defer func() { _ = w.Close() }()

	notifyEvents := make(chan struct{}, 1)
	require.NoError(t, w.startMonitorInformer(monitoringv1.ServiceMonitorName, notifyEvents))
	require.True(t, runningInformers(w)[monitoringv1.ServiceMonitorName])
	// drain the start notification
	select {
	case <-notifyEvents:
	default:
	}

	w.stopMonitorInformer(monitoringv1.ServiceMonitorName, notifyEvents)

	assert.False(t, runningInformers(w)[monitoringv1.ServiceMonitorName], "informer should be stopped and dropped")
	// a reload must be signalled so the dropped type's targets are removed
	select {
	case <-notifyEvents:
	default:
		t.Fatal("expected a reload notification after stopping the informer")
	}

	// LoadConfig must succeed and emit no scrape jobs once the type is dropped.
	cfg, err := w.LoadConfig(context.Background())
	require.NoError(t, err)
	assert.Empty(t, cfg.ScrapeConfigs)
}

// TestCRDObjectName verifies the CRD-name extraction used by the CRD watch,
// including the delete tombstone wrapper.
func TestCRDObjectName(t *testing.T) {
	crd := crdFor(monitoringv1.ServiceMonitorName)
	name, ok := crdObjectName(crd)
	require.True(t, ok)
	assert.Equal(t, "servicemonitors.monitoring.coreos.com", name)
	if _, tracked := crdNameToResource[name]; !tracked {
		t.Fatalf("CRD name %q is not mapped to a monitoring resource", name)
	}

	name, ok = crdObjectName(cache.DeletedFinalStateUnknown{Key: "k", Obj: crd})
	require.True(t, ok)
	assert.Equal(t, "servicemonitors.monitoring.coreos.com", name)

	_, ok = crdObjectName("not-a-crd")
	assert.False(t, ok)
}

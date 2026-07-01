// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/prometheus-operator/prometheus-operator/pkg/assets"
	monitoringclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/prometheus-operator/prometheus-operator/pkg/informers"
	"github.com/prometheus-operator/prometheus-operator/pkg/prometheus"
	promconfig "github.com/prometheus/prometheus/config"
	kubeDiscovery "github.com/prometheus/prometheus/discovery/kubernetes"
	"gopkg.in/yaml.v2"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	allocatorconfig "github.com/aws/amazon-cloudwatch-agent-operator/cmd/amazon-cloudwatch-agent-target-allocator/config"
)

const minEventInterval = time.Second * 5

// monitoringResources maps the prometheus-operator resource name to the
// GroupVersionResource used to build its informer.
var monitoringResources = map[string]schema.GroupVersionResource{
	monitoringv1.ServiceMonitorName: monitoringv1.SchemeGroupVersion.WithResource(monitoringv1.ServiceMonitorName),
	monitoringv1.PodMonitorName:     monitoringv1.SchemeGroupVersion.WithResource(monitoringv1.PodMonitorName),
}

// crdNameToResource maps a CustomResourceDefinition object name to the
// prometheus-operator resource key whose informer it backs. The TA watches
// these CRDs so it can start/stop each informer independently as the CRD
// appears or disappears, rather than requiring both CRDs to exist at startup.
var crdNameToResource = map[string]string{
	monitoringv1.ServiceMonitorName + "." + monitoringv1.SchemeGroupVersion.Group: monitoringv1.ServiceMonitorName,
	monitoringv1.PodMonitorName + "." + monitoringv1.SchemeGroupVersion.Group:     monitoringv1.PodMonitorName,
}

func NewPrometheusCRWatcher(logger logr.Logger, cfg allocatorconfig.Config) (*PrometheusCRWatcher, error) {
	mClient, err := monitoringclient.NewForConfig(cfg.ClusterConfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(cfg.ClusterConfig)
	if err != nil {
		return nil, err
	}

	crdClient, err := apiextensionsclient.NewForConfig(cfg.ClusterConfig)
	if err != nil {
		return nil, err
	}

	// TODO: We should make these durations configurable
	// The prometheus-operator config generator calls store.ForNamespace(prom.Namespace)
	// which panics on an empty namespace, so the synthetic Prometheus must carry the
	// allocator's own namespace (injected by the operator as OTELCOL_NAMESPACE).
	namespace := os.Getenv("OTELCOL_NAMESPACE")
	if namespace == "" {
		namespace = "amazon-cloudwatch"
	}
	prom := &monitoringv1.Prometheus{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
		},
		Spec: monitoringv1.PrometheusSpec{
			CommonPrometheusFields: monitoringv1.CommonPrometheusFields{
				ScrapeInterval: monitoringv1.Duration(cfg.PrometheusCR.ScrapeInterval.String()),
			},
			// The config generator unconditionally emits evaluation_interval; an empty
			// value makes the downstream Prometheus config parser fail with
			// "empty duration string", so give it the scrape interval as a sane default.
			EvaluationInterval: monitoringv1.Duration(cfg.PrometheusCR.ScrapeInterval.String()),
		},
	}

	promOperatorLogger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	generator, err := prometheus.NewConfigGenerator(promOperatorLogger, prom, prometheus.WithEndpointSliceSupport())

	if err != nil {
		return nil, err
	}

	servMonSelector := getSelector(cfg.ServiceMonitorSelector)

	podMonSelector := getSelector(cfg.PodMonitorSelector)

	// Informers are NOT built here. Each ServiceMonitor/PodMonitor informer is
	// started lazily, only once its CRD is observed to exist (see Watch). This
	// lets the TA start and run healthily whether or not the CRDs are present,
	// and pick up each type independently when its CRD appears.
	return &PrometheusCRWatcher{
		logger:                 logger,
		kubeMonitoringClient:   mClient,
		k8sClient:              clientset,
		crdClient:              crdClient,
		informers:              map[string]*informers.ForResource{},
		informerStopChannels:   map[string]chan struct{}{},
		stopChannel:            make(chan struct{}),
		eventInterval:          minEventInterval,
		configGenerator:        generator,
		prom:                   prom,
		kubeConfigPath:         cfg.KubeConfigFilePath,
		serviceMonitorSelector: servMonSelector,
		podMonitorSelector:     podMonSelector,
	}, nil
}

type PrometheusCRWatcher struct {
	logger               logr.Logger
	kubeMonitoringClient monitoringclient.Interface
	k8sClient            kubernetes.Interface
	crdClient            apiextensionsclient.Interface
	eventInterval        time.Duration
	stopChannel          chan struct{}
	configGenerator      *prometheus.ConfigGenerator
	prom                 *monitoringv1.Prometheus
	kubeConfigPath       string

	// informersMtx guards informers and informerStopChannels, which are mutated
	// from CRD-watch event handlers (separate goroutines) and read by LoadConfig.
	informersMtx         sync.RWMutex
	informers            map[string]*informers.ForResource
	informerStopChannels map[string]chan struct{}

	serviceMonitorSelector labels.Selector
	podMonitorSelector     labels.Selector
}

func getSelector(s map[string]string) labels.Selector {
	if s == nil {
		return labels.NewSelector()
	}
	return labels.SelectorFromSet(s)
}

// newMonitoringFactory builds a fresh monitoring informer factory. A new factory
// is created per informer start so that a stopped informer (its CRD was deleted)
// can be cleanly restarted later if the CRD is recreated — shared informers
// cannot be restarted once their stop channel is closed.
func (w *PrometheusCRWatcher) newMonitoringFactory() informers.FactoriesForNamespaces {
	return informers.NewMonitoringInformerFactories(
		map[string]struct{}{v1.NamespaceAll: {}},
		map[string]struct{}{},
		w.kubeMonitoringClient,
		allocatorconfig.DefaultResyncTime,
		nil,
	) //TODO decide what strategy to use regarding namespaces
}

// crdExists reports whether the named CustomResourceDefinition is currently
// registered in the cluster.
func (w *PrometheusCRWatcher) crdExists(ctx context.Context, crdName string) (bool, error) {
	_, err := w.crdClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crdName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// startMonitorInformer builds and starts the informer for a single monitoring
// resource (ServiceMonitor or PodMonitor), waits for its cache to sync, wires up
// the notification handler, and records it for use by LoadConfig. It is
// idempotent: calling it for an already-running informer is a no-op.
func (w *PrometheusCRWatcher) startMonitorInformer(resourceName string, notifyEvents chan struct{}) error {
	w.informersMtx.Lock()
	defer w.informersMtx.Unlock()

	if _, running := w.informers[resourceName]; running {
		return nil
	}

	gvr, ok := monitoringResources[resourceName]
	if !ok {
		return fmt.Errorf("unknown monitoring resource %q", resourceName)
	}

	informer, err := informers.NewInformersForResource(w.newMonitoringFactory(), gvr)
	if err != nil {
		return err
	}

	stopCh := make(chan struct{})
	informer.Start(stopCh)
	if ok := cache.WaitForNamedCacheSync(resourceName, stopCh, informer.HasSynced); !ok {
		close(stopCh)
		return fmt.Errorf("failed to sync %s informer cache", resourceName)
	}

	informer.AddEventHandler(notifyHandler(notifyEvents))

	w.informers[resourceName] = informer
	w.informerStopChannels[resourceName] = stopCh

	// A new resource type just became available; trigger a config reload.
	notify(notifyEvents)
	return nil
}

// stopMonitorInformer stops the informer for a single monitoring resource (its
// CRD was removed) and drops it from the active set so its targets are no longer
// generated. It is idempotent.
func (w *PrometheusCRWatcher) stopMonitorInformer(resourceName string, notifyEvents chan struct{}) {
	w.informersMtx.Lock()
	defer w.informersMtx.Unlock()

	stopCh, running := w.informerStopChannels[resourceName]
	if !running {
		return
	}
	close(stopCh)
	delete(w.informerStopChannels, resourceName)
	delete(w.informers, resourceName)

	// The type went away; trigger a config reload so its targets are dropped.
	notify(notifyEvents)
}

// crdObjectName extracts the CustomResourceDefinition name from an informer
// event object, tolerating the tombstone wrapper delivered on some deletes.
func crdObjectName(obj interface{}) (string, bool) {
	switch t := obj.(type) {
	case *apiextensionsv1.CustomResourceDefinition:
		return t.Name, true
	case cache.DeletedFinalStateUnknown:
		if crd, ok := t.Obj.(*apiextensionsv1.CustomResourceDefinition); ok {
			return crd.Name, true
		}
	}
	return "", false
}

// watchCRDs starts a CustomResourceDefinition informer that starts/stops the
// per-type monitoring informers as the ServiceMonitor/PodMonitor CRDs are
// created or deleted at runtime — no TA restart required. The informer replays
// existing CRDs on its initial sync, so CRDs present at startup are handled here
// too (startMonitorInformer is idempotent).
func (w *PrometheusCRWatcher) watchCRDs(notifyEvents chan struct{}) {
	factory := apiextensionsinformers.NewSharedInformerFactory(w.crdClient, allocatorconfig.DefaultResyncTime)
	crdInformer := factory.Apiextensions().V1().CustomResourceDefinitions().Informer()
	_, _ = crdInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			name, ok := crdObjectName(obj)
			if !ok {
				return
			}
			resourceName, tracked := crdNameToResource[name]
			if !tracked {
				return
			}
			if err := w.startMonitorInformer(resourceName, notifyEvents); err != nil {
				w.logger.Error(err, "prometheus-cr: failed to start informer after CRD became available", "crd", name)
				return
			}
			w.logger.Info("prometheus-cr: CRD available, started informer", "crd", name, "resource", resourceName)
		},
		DeleteFunc: func(obj interface{}) {
			name, ok := crdObjectName(obj)
			if !ok {
				return
			}
			resourceName, tracked := crdNameToResource[name]
			if !tracked {
				return
			}
			w.stopMonitorInformer(resourceName, notifyEvents)
			w.logger.Info("prometheus-cr: CRD removed, stopped informer and dropped targets", "crd", name, "resource", resourceName)
		},
	})
	factory.Start(w.stopChannel)
}

// notify performs a non-blocking send on the buffered notification channel.
func notify(notifyEvents chan struct{}) {
	select {
	case notifyEvents <- struct{}{}:
	default:
	}
}

// notifyHandler returns event handlers that coalesce ServiceMonitor/PodMonitor
// changes into the notification channel (non-blocking so rate-limiting upstream
// is never starved).
func notifyHandler(notifyEvents chan struct{}) cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { notify(notifyEvents) },
		UpdateFunc: func(oldObj, newObj interface{}) { notify(notifyEvents) },
		DeleteFunc: func(obj interface{}) { notify(notifyEvents) },
	}
}

// Watch starts watching for ServiceMonitor/PodMonitor changes. It is resilient
// to either CRD being absent: it starts the informer for each CRD that exists
// now, defers the rest until their CRDs appear (via a CustomResourceDefinition
// watch), and never fails startup just because a CRD is missing. This means the
// TA starts and stays healthy regardless of CRD install ordering, and begins
// (or stops) watching each type automatically as its CRD is created or deleted.
func (w *PrometheusCRWatcher) Watch(upstreamEvents chan Event, upstreamErrors chan error) error {
	// this channel needs to be buffered because notifications are asynchronous and neither producers nor consumers wait
	notifyEvents := make(chan struct{}, 1)

	// Start informers for CRDs that already exist. Absent CRDs are simply
	// skipped here; the CRD watch below starts them if/when they appear.
	for crdName, resourceName := range crdNameToResource {
		exists, err := w.crdExists(context.Background(), crdName)
		if err != nil {
			// Surface the error but keep going — a transient API error must not
			// take down the allocator. The CRD watch will recover the informer.
			w.logger.Error(err, "prometheus-cr: failed to check for CRD, deferring to CRD watch", "crd", crdName)
			continue
		}
		if !exists {
			w.logger.Info("prometheus-cr: CRD not present at startup, deferring informer until it is created", "crd", crdName)
			continue
		}
		if startErr := w.startMonitorInformer(resourceName, notifyEvents); startErr != nil {
			w.logger.Error(startErr, "prometheus-cr: failed to start informer for present CRD, deferring to CRD watch", "crd", crdName)
			continue
		}
		w.logger.Info("prometheus-cr: started informer for present CRD", "crd", crdName, "resource", resourceName)
	}

	// React to CRDs being created/deleted at runtime with no restart.
	w.watchCRDs(notifyEvents)

	// limit the rate of outgoing events
	w.rateLimitedEventSender(upstreamEvents, notifyEvents)

	<-w.stopChannel
	return nil
}

// rateLimitedEventSender sends events to the upstreamEvents channel whenever it gets a notification on the notifyEvents channel,
// but not more frequently than once per w.eventPeriod.
func (w *PrometheusCRWatcher) rateLimitedEventSender(upstreamEvents chan Event, notifyEvents chan struct{}) {
	ticker := time.NewTicker(w.eventInterval)
	defer ticker.Stop()

	event := Event{
		Source:  EventSourcePrometheusCR,
		Watcher: Watcher(w),
	}

	for {
		select {
		case <-w.stopChannel:
			return
		case <-ticker.C: // throttle events to avoid excessive updates
			select {
			case <-notifyEvents:
				select {
				case upstreamEvents <- event:
				default: // put the notification back in the queue if we can't send it upstream
					select {
					case notifyEvents <- struct{}{}:
					default:
					}
				}
			default:
			}
		}
	}
}

func (w *PrometheusCRWatcher) Close() error {
	// Stop any per-type monitoring informers (each has its own stop channel).
	w.informersMtx.Lock()
	for name, stopCh := range w.informerStopChannels {
		close(stopCh)
		delete(w.informerStopChannels, name)
		delete(w.informers, name)
	}
	w.informersMtx.Unlock()

	// Stop the CRD watch and release Watch's blocking read.
	close(w.stopChannel)
	return nil
}

func (w *PrometheusCRWatcher) LoadConfig(ctx context.Context) (*promconfig.Config, error) {
	// Snapshot the currently-running informers. Either may be absent if its CRD
	// is not (yet) installed; in that case its monitors are simply skipped and
	// no scrape jobs are generated for that type.
	w.informersMtx.RLock()
	smInformer := w.informers[monitoringv1.ServiceMonitorName]
	pmInformer := w.informers[monitoringv1.PodMonitorName]
	w.informersMtx.RUnlock()

	store := assets.NewStoreBuilder(w.k8sClient.CoreV1(), w.k8sClient.CoreV1())
	serviceMonitorInstances := make(map[string]*monitoringv1.ServiceMonitor)
	if smInformer != nil {
		smRetrieveErr := smInformer.ListAll(w.serviceMonitorSelector, func(sm interface{}) {
			monitor := sm.(*monitoringv1.ServiceMonitor)
			key, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(monitor)
			w.addStoreAssetsForServiceMonitor(ctx, monitor.Name, monitor.Namespace, monitor.Spec.Endpoints, store)
			serviceMonitorInstances[key] = monitor
		})
		if smRetrieveErr != nil {
			return nil, smRetrieveErr
		}
	}

	podMonitorInstances := make(map[string]*monitoringv1.PodMonitor)
	if pmInformer != nil {
		pmRetrieveErr := pmInformer.ListAll(w.podMonitorSelector, func(pm interface{}) {
			monitor := pm.(*monitoringv1.PodMonitor)
			key, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(monitor)
			w.addStoreAssetsForPodMonitor(ctx, monitor.Name, monitor.Namespace, monitor.Spec.PodMetricsEndpoints, store)
			podMonitorInstances[key] = monitor
		})
		if pmRetrieveErr != nil {
			return nil, pmRetrieveErr
		}
	}

	generatedConfig, err := w.configGenerator.GenerateServerConfiguration(
		w.prom,
		serviceMonitorInstances,
		podMonitorInstances,
		map[string]*monitoringv1.Probe{},
		map[string]*promv1alpha1.ScrapeConfig{},
		store,
		nil,
		nil,
		nil,
		[]string{})
	if err != nil {
		return nil, err
	}

	promCfg := &promconfig.Config{}
	unmarshalErr := yaml.Unmarshal(generatedConfig, promCfg)
	if unmarshalErr != nil {
		return nil, unmarshalErr
	}

	// set kubeconfig path to service discovery configs, else kubernetes_sd will always attempt in-cluster
	// authentication even if running with a detected kubeconfig
	for _, scrapeConfig := range promCfg.ScrapeConfigs {
		for _, serviceDiscoveryConfig := range scrapeConfig.ServiceDiscoveryConfigs {
			if serviceDiscoveryConfig.Name() == "kubernetes" {
				sdConfig := interface{}(serviceDiscoveryConfig).(*kubeDiscovery.SDConfig)
				sdConfig.KubeConfig = w.kubeConfigPath
			}
		}
	}
	return promCfg, nil
}

// addStoreAssetsForServiceMonitor adds authentication / authorization related information to the assets store,
// based on the service monitor and endpoints specs.
// This code borrows from
// https://github.com/prometheus-operator/prometheus-operator/blob/06b5c4189f3f72737766d86103d049115c3aff48/pkg/prometheus/resource_selector.go#L73.
func (w *PrometheusCRWatcher) addStoreAssetsForServiceMonitor(
	ctx context.Context,
	smName, smNamespace string,
	endps []monitoringv1.Endpoint,
	store *assets.StoreBuilder,
) {
	var err error
	for _, endp := range endps {
		if err = store.AddSafeAuthorizationCredentials(ctx, smNamespace, endp.Authorization); err != nil {
			break
		}

		if err = store.AddBasicAuth(ctx, smNamespace, endp.BasicAuth); err != nil {
			break
		}

		if endp.TLSConfig != nil {
			if err = store.AddTLSConfig(ctx, smNamespace, endp.TLSConfig); err != nil {
				break
			}
		}

		if err = store.AddOAuth2(ctx, smNamespace, endp.OAuth2); err != nil {
			break
		}
	}

	if err != nil {
		w.logger.Error(err, "Failed to obtain credentials for a ServiceMonitor", "serviceMonitor", smName)
	}
}

// addStoreAssetsForServiceMonitor adds authentication / authorization related information to the assets store,
// based on the service monitor and pod metrics endpoints specs.
// This code borrows from
// https://github.com/prometheus-operator/prometheus-operator/blob/06b5c4189f3f72737766d86103d049115c3aff48/pkg/prometheus/resource_selector.go#L314.
func (w *PrometheusCRWatcher) addStoreAssetsForPodMonitor(
	ctx context.Context,
	pmName, pmNamespace string,
	podMetricsEndps []monitoringv1.PodMetricsEndpoint,
	store *assets.StoreBuilder,
) {
	var err error
	for _, endp := range podMetricsEndps {
		if err = store.AddSafeAuthorizationCredentials(ctx, pmNamespace, endp.Authorization); err != nil {
			break
		}

		if err = store.AddBasicAuth(ctx, pmNamespace, endp.BasicAuth); err != nil {
			break
		}

		if endp.TLSConfig != nil {
			if err = store.AddSafeTLSConfig(ctx, pmNamespace, endp.TLSConfig); err != nil {
				break
			}
		}

		if err = store.AddOAuth2(ctx, pmNamespace, endp.OAuth2); err != nil {
			break
		}
	}

	if err != nil {
		w.logger.Error(err, "Failed to obtain credentials for a PodMonitor", "podMonitor", pmName)
	}
}

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
	v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/tools/cache"

	"github.com/aws/amazon-cloudwatch-agent-operator/cmd/amazon-cloudwatch-agent-target-allocator/collectornamespace"
	allocatorconfig "github.com/aws/amazon-cloudwatch-agent-operator/cmd/amazon-cloudwatch-agent-target-allocator/config"
)

const minEventInterval = time.Second * 5

// crdCheckTimeout bounds each startup CustomResourceDefinition existence check so
// a wedged apiserver cannot block the Watch startup loop indefinitely.
const crdCheckTimeout = 10 * time.Second

// Defaults for starting a monitor informer. A start that has not synced within
// defaultSyncTimeout gives up and is retried after a backoff that starts at
// defaultStartRetryInterval and doubles up to defaultMaxStartRetryInterval, so a
// type that cannot be listed (CRD not yet Established, missing list/watch, or a
// CRD that is not actually installed) never holds a goroutine for the pod lifetime.
const (
	// Must be well above client-go's 100ms cache-sync poll period.
	defaultSyncTimeout           = 2 * time.Minute
	defaultStartRetryInterval    = 10 * time.Second
	defaultMaxStartRetryInterval = 5 * time.Minute
	// defaultMaxBlindStartAttempts bounds starts made when CRD presence cannot be
	// checked (the CRD check is Forbidden): after this many failed attempts the
	// type is left stopped, since the CRD is most likely not installed.
	defaultMaxBlindStartAttempts = 3
)

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

	// Metadata-only client for the CRD watch: it caches just object metadata
	// (name), not each CustomResourceDefinition's full spec/OpenAPI schema, so
	// memory stays flat on clusters with many (large) CRDs.
	metadataClient, err := metadata.NewForConfig(cfg.ClusterConfig)
	if err != nil {
		return nil, err
	}

	// TODO: We should make these durations configurable
	// Namespace must be non-empty; the config generator panics otherwise.
	collectorNamespace := collectornamespace.Resolve(logger)
	prom := &monitoringv1.Prometheus{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: collectorNamespace,
		},
		Spec: monitoringv1.PrometheusSpec{
			CommonPrometheusFields: monitoringv1.CommonPrometheusFields{
				ScrapeInterval: monitoringv1.Duration(cfg.PrometheusCR.ScrapeInterval.String()),
			},
			// Must be non-empty; default to scrape interval.
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
		metadataClient:         metadataClient,
		informers:              map[string]*informers.ForResource{},
		informerStopChannels:   map[string]chan struct{}{},
		stopChannel:            make(chan struct{}),
		eventInterval:          minEventInterval,
		configGenerator:        generator,
		prom:                   prom,
		kubeConfigPath:         cfg.KubeConfigFilePath,
		serviceMonitorSelector: servMonSelector,
		podMonitorSelector:     podMonSelector,
		starting:               map[string]struct{}{},
		syncTimeout:            defaultSyncTimeout,
		startRetryInterval:     defaultStartRetryInterval,
		maxStartRetryInterval:  defaultMaxStartRetryInterval,
		maxBlindStartAttempts:  defaultMaxBlindStartAttempts,
	}, nil
}

type PrometheusCRWatcher struct {
	logger               logr.Logger
	kubeMonitoringClient monitoringclient.Interface
	k8sClient            kubernetes.Interface
	crdClient            apiextensionsclient.Interface
	metadataClient       metadata.Interface
	eventInterval        time.Duration
	stopChannel          chan struct{}
	closeOnce            sync.Once
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

	// startMtx guards starting: the monitor types with a start in flight, so
	// concurrent triggers (startup, CRD add/update, retries) never stack.
	startMtx sync.Mutex
	starting map[string]struct{}

	// upstreamErrors receives start failures; set by Watch.
	upstreamErrors chan error

	syncTimeout           time.Duration
	startRetryInterval    time.Duration
	maxStartRetryInterval time.Duration
	maxBlindStartAttempts int
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
// the notification handler, and records it for use by LoadConfig. It reports
// started=true only when it registered a new informer: an already-running
// informer, a watcher that closed during the sync, or a concurrent start that
// won the race all return started=false with no error. A cache that does not
// sync within w.syncTimeout returns an error. It blocks for up to the sync
// timeout, so callers that must not block use ensureMonitorInformer.
func (w *PrometheusCRWatcher) startMonitorInformer(resourceName string, notifyEvents chan struct{}) (bool, error) {
	gvr, ok := monitoringResources[resourceName]
	if !ok {
		return false, fmt.Errorf("unknown monitoring resource %q", resourceName)
	}

	select {
	case <-w.stopChannel:
		return false, nil
	default:
	}

	// Fast path: already running. Checked under a short read lock so we never hold
	// informersMtx across the (potentially slow) cache sync below.
	w.informersMtx.RLock()
	_, running := w.informers[resourceName]
	w.informersMtx.RUnlock()
	if running {
		return false, nil
	}

	informer, err := informers.NewInformersForResource(w.newMonitoringFactory(), gvr)
	if err != nil {
		return false, err
	}

	stopCh := make(chan struct{})
	informer.Start(stopCh)

	// Wait for the cache to sync WITHOUT holding informersMtx, bounded by
	// w.syncTimeout, and abort the wait if the watcher is shutting down. Holding
	// the lock here would block LoadConfig and deadlock Close(), which needs the
	// same lock. Closing stopCh on every early return stops the informer's
	// reflector and unblocks the WaitForNamedCacheSync goroutine.
	syncStop := make(chan struct{})
	synced := make(chan bool, 1)
	go func() {
		synced <- cache.WaitForNamedCacheSync(resourceName, syncStop, informer.HasSynced)
	}()
	timeout := time.NewTimer(w.syncTimeout)
	defer timeout.Stop()
	select {
	case ok := <-synced:
		if !ok {
			close(stopCh)
			return false, fmt.Errorf("failed to sync %s informer cache", resourceName)
		}
	case <-timeout.C:
		close(syncStop)
		close(stopCh)
		return false, fmt.Errorf("%s informer cache did not sync within %s", resourceName, w.syncTimeout)
	case <-w.stopChannel:
		close(syncStop)
		close(stopCh)
		return false, nil
	}

	informer.AddEventHandler(notifyHandler(notifyEvents))

	w.informersMtx.Lock()
	// If the watcher closed while we were syncing, don't register a live informer:
	// Close() has already drained the stop channels and would never stop this one.
	select {
	case <-w.stopChannel:
		w.informersMtx.Unlock()
		close(stopCh)
		return false, nil
	default:
	}
	// If another goroutine already started this resource, discard ours.
	if _, running := w.informers[resourceName]; running {
		w.informersMtx.Unlock()
		close(stopCh)
		return false, nil
	}
	w.informers[resourceName] = informer
	w.informerStopChannels[resourceName] = stopCh
	w.informersMtx.Unlock()

	// A new resource type just became available; trigger a config reload.
	notify(notifyEvents)
	return true, nil
}

// ensureMonitorInformer starts the informer for resourceName in the background
// and never blocks the caller. At most one start per type is in flight, so the
// startup loop, CRD add/update events and resyncs can all call it freely. A
// failed start is reported on upstreamErrors and retried with capped backoff
// until it succeeds, the informer is running, the CRD is deleted, or the watcher
// closes. reason is included in log lines to show what triggered the start.
func (w *PrometheusCRWatcher) ensureMonitorInformer(resourceName, crdName, reason string, notifyEvents chan struct{}) {
	w.startMtx.Lock()
	if _, inFlight := w.starting[resourceName]; inFlight {
		w.startMtx.Unlock()
		return
	}
	w.starting[resourceName] = struct{}{}
	w.startMtx.Unlock()

	go func() {
		defer func() {
			w.startMtx.Lock()
			delete(w.starting, resourceName)
			w.startMtx.Unlock()
		}()
		backoff := w.startRetryInterval
		for attempt := 1; ; attempt++ {
			started, err := w.startMonitorInformer(resourceName, notifyEvents)
			if err == nil {
				if started {
					w.logger.Info("prometheus-cr: started informer", "crd", crdName, "resource", resourceName, "trigger", reason)
				}
				return
			}
			w.reportError(fmt.Errorf("prometheus-cr: starting %s informer: %w", resourceName, err))

			present, known := w.crdPresence(crdName)
			switch {
			case known && !present:
				// The CRD is gone; the CRD watch starts the informer again if it returns.
				w.logger.Error(err, "prometheus-cr: failed to start informer; CRD no longer present, not retrying",
					"crd", crdName, "resource", resourceName, "trigger", reason)
				return
			case !known && attempt >= w.maxBlindStartAttempts:
				// CRD presence cannot be checked (e.g. Forbidden) and every attempt
				// failed, so the CRD is most likely not installed. Stop rather than
				// retry a LIST on an unserved resource for the pod lifetime.
				w.logger.Error(err, "prometheus-cr: failed to start informer and cannot check for its CRD; giving up",
					"crd", crdName, "resource", resourceName, "trigger", reason, "attempts", attempt)
				return
			}
			w.logger.Error(err, "prometheus-cr: failed to start informer, will retry",
				"crd", crdName, "resource", resourceName, "trigger", reason, "attempt", attempt, "retryIn", backoff.String())

			select {
			case <-w.stopChannel:
				return
			case <-time.After(backoff):
			}
			if backoff *= 2; backoff > w.maxStartRetryInterval {
				backoff = w.maxStartRetryInterval
			}
		}
	}()
}

// reportError sends err on upstreamErrors without blocking.
func (w *PrometheusCRWatcher) reportError(err error) {
	if w.upstreamErrors == nil {
		return
	}
	select {
	case w.upstreamErrors <- err:
	default:
	}
}

// crdPresence checks whether the CRD exists. known is false when the check
// itself failed (for example Forbidden), in which case present is meaningless.
func (w *PrometheusCRWatcher) crdPresence(crdName string) (present, known bool) {
	ctx, cancel := context.WithTimeout(context.Background(), crdCheckTimeout)
	defer cancel()
	exists, err := w.crdExists(ctx, crdName)
	return exists, err == nil
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

// crdObjectName extracts the CustomResourceDefinition name from a metadata
// informer event object, tolerating the tombstone wrapper delivered on some
// deletes. The metadata informer delivers *metav1.PartialObjectMetadata (name
// only), not the full CustomResourceDefinition.
func crdObjectName(obj interface{}) (string, bool) {
	switch t := obj.(type) {
	case *metav1.PartialObjectMetadata:
		return t.Name, true
	case cache.DeletedFinalStateUnknown:
		if crd, ok := t.Obj.(*metav1.PartialObjectMetadata); ok {
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
	crdGVR := apiextensionsv1.SchemeGroupVersion.WithResource("customresourcedefinitions")
	factory := metadatainformer.NewSharedInformerFactory(w.metadataClient, allocatorconfig.DefaultResyncTime)
	crdInformer := factory.ForResource(crdGVR).Informer()
	if _, err := crdInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			name, ok := crdObjectName(obj)
			if !ok {
				return
			}
			resourceName, tracked := crdNameToResource[name]
			if !tracked {
				return
			}
			w.ensureMonitorInformer(resourceName, name, "crd-added", notifyEvents)
		},
		// Resyncs and CRD changes arrive as updates. Starting is idempotent and
		// non-blocking, so each one doubles as a retry for a type that is not
		// running yet.
		UpdateFunc: func(_, newObj interface{}) {
			name, ok := crdObjectName(newObj)
			if !ok {
				return
			}
			resourceName, tracked := crdNameToResource[name]
			if !tracked {
				return
			}
			w.ensureMonitorInformer(resourceName, name, "crd-resync", notifyEvents)
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
	}); err != nil {
		// A failed registration means CRD create/delete would go unobserved; log it
		// so a CRD watch with no handler shows up in the logs.
		w.logger.Error(err, "prometheus-cr: failed to register CRD informer event handler; runtime CRD add/remove will not be observed")
	}
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
	w.upstreamErrors = upstreamErrors

	// Start informers for CRDs that already exist. Every start runs in the
	// background (ensureMonitorInformer), so a type that cannot sync never stops
	// the other type, the CRD watch or the event pump below from starting.
	for crdName, resourceName := range crdNameToResource {
		ctx, cancel := context.WithTimeout(context.Background(), crdCheckTimeout)
		exists, err := w.crdExists(ctx, crdName)
		cancel()
		if err != nil {
			// The check failed for a reason other than "not found", e.g. the
			// ServiceAccount cannot read customresourcedefinitions. The CRD watch
			// would hit the same error, so start the informer anyway: if the CRD
			// exists, discovery works. If it does not, each attempt gives up after
			// the sync timeout and is retried with capped backoff.
			w.logger.Info("prometheus-cr: WARNING could not check for CRD, starting its informer anyway; "+
				"grant get/list/watch on apiextensions.k8s.io customresourcedefinitions to the Target Allocator "+
				"so CRDs installed or removed later are detected",
				"crd", crdName, "error", err.Error())
			w.ensureMonitorInformer(resourceName, crdName, "crd-check-error", notifyEvents)
			continue
		}
		if !exists {
			w.logger.Info("prometheus-cr: CRD not present at startup, deferring informer until it is created", "crd", crdName)
			continue
		}
		w.ensureMonitorInformer(resourceName, crdName, "present-at-startup", notifyEvents)
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
	// sync.Once so a second Close() is a safe no-op rather than a panic on the
	// already-closed stopChannel.
	w.closeOnce.Do(func() {
		// Signal shutdown first so any in-flight startMonitorInformer aborts its cache
		// sync and will not register a new informer after this point.
		close(w.stopChannel)

		// Stop any per-type monitoring informers (each has its own stop channel).
		w.informersMtx.Lock()
		for name, stopCh := range w.informerStopChannels {
			close(stopCh)
			delete(w.informerStopChannels, name)
			delete(w.informers, name)
		}
		w.informersMtx.Unlock()
	})

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

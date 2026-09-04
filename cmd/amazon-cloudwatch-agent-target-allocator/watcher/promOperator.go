// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"os"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	allocatorconfig "github.com/aws/amazon-cloudwatch-agent-operator/cmd/amazon-cloudwatch-agent-target-allocator/config"
)

const defaultCollectorNamespace = "amazon-cloudwatch"

const minEventInterval = time.Second * 5

func NewPrometheusCRWatcher(logger logr.Logger, cfg allocatorconfig.Config) (*PrometheusCRWatcher, error) {
	mClient, err := monitoringclient.NewForConfig(cfg.ClusterConfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(cfg.ClusterConfig)
	if err != nil {
		return nil, err
	}

	factory := informers.NewMonitoringInformerFactories(map[string]struct{}{v1.NamespaceAll: {}}, map[string]struct{}{}, mClient, allocatorconfig.DefaultResyncTime, nil) //TODO decide what strategy to use regarding namespaces

	monitoringInformers, err := getInformers(factory)
	if err != nil {
		return nil, err
	}

	// TODO: We should make these durations configurable
	// Namespace must be non-empty; the config generator panics otherwise.
	collectorNamespace := os.Getenv("OTELCOL_NAMESPACE")
	if collectorNamespace == "" {
		if ns, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil && len(ns) > 0 {
			collectorNamespace = string(ns)
		} else {
			collectorNamespace = defaultCollectorNamespace
		}
		logger.Info("OTELCOL_NAMESPACE not set, resolved namespace", "namespace", collectorNamespace)
	}
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

	return &PrometheusCRWatcher{
		logger:                 logger,
		kubeMonitoringClient:   mClient,
		k8sClient:              clientset,
		informers:              monitoringInformers,
		stopChannel:            make(chan struct{}),
		eventInterval:          minEventInterval,
		configGenerator:        generator,
		prom:                   prom,
		kubeConfigPath:         cfg.KubeConfigFilePath,
		serviceMonitorSelector: servMonSelector,
		podMonitorSelector:     podMonSelector,
		scraperRole:            cfg.ScraperRole,
	}, nil
}

type PrometheusCRWatcher struct {
	logger               logr.Logger
	kubeMonitoringClient monitoringclient.Interface
	k8sClient            kubernetes.Interface
	informers            map[string]*informers.ForResource
	eventInterval        time.Duration
	stopChannel          chan struct{}
	configGenerator      *prometheus.ConfigGenerator
	prom                 *monitoringv1.Prometheus
	kubeConfigPath       string

	serviceMonitorSelector labels.Selector
	podMonitorSelector     labels.Selector
	scraperRole            string
}

func getSelector(s map[string]string) labels.Selector {
	if s == nil {
		return labels.NewSelector()
	}
	return labels.SelectorFromSet(s)
}

// ScraperAnnotationKey and clusterScraperRole implement annotation-based routing of
// ServiceMonitor/PodMonitor CRs across CloudWatch agents. A monitor annotated
// cloudwatch.aws/scraper: cluster-scraper is scraped only by the cluster-scraper agent's Target
// Allocator; all others are scraped only by the per-node agent's Target Allocator.
const (
	ScraperAnnotationKey = "cloudwatch.aws/scraper"
	clusterScraperRole   = "cluster-scraper"
)

// annotationRoleMatches reports whether a monitor with the given annotations belongs to this Target
// Allocator, based on its scraperRole. cluster-scraper role selects only monitors annotated
// cloudwatch.aws/scraper: cluster-scraper; the default role (empty) selects only monitors that are
// not so annotated, so the two roles partition monitors with no overlap and no gap.
// annotationRoleMatches reports whether a monitor with the given annotations
// belongs to scraperRole. Routing is intentionally BINARY today: the
// cluster-scraper role claims monitors annotated cloudwatch.aws/scraper:
// cluster-scraper, and every other role (the default per-node agent) claims the
// rest. NOTE: before a third role is introduced (e.g. a "gpu-scraper"), this must
// become an explicit role-to-annotation-value match — otherwise any unrecognized
// role would silently fall through to the per-node bucket here.
func annotationRoleMatches(scraperRole string, annotations map[string]string) bool {
	routed := annotations[ScraperAnnotationKey] == clusterScraperRole
	if scraperRole == clusterScraperRole {
		return routed
	}
	return !routed
}

// selectsMonitor reports whether a discovered ServiceMonitor/PodMonitor belongs to this Target
// Allocator's scraper role (see annotationRoleMatches). When this allocator is the cluster-scraper,
// it logs each monitor it claims, because that monitor was explicitly overridden onto the
// cluster-scraper via the cloudwatch.aws/scraper annotation.
func (w *PrometheusCRWatcher) selectsMonitor(kind, namespace, name string, annotations map[string]string) bool {
	if !annotationRoleMatches(w.scraperRole, annotations) {
		return false
	}
	if w.scraperRole == clusterScraperRole {
		// V(1): LoadConfig reruns on every reconcile, so this fires per claimed
		// monitor each cycle — keep it at debug verbosity to avoid log spam on
		// large annotated sets.
		w.logger.V(1).Info("routing monitor to cluster-scraper via annotation",
			"kind", kind, "namespace", namespace, "name", name,
			"annotation", ScraperAnnotationKey+"="+clusterScraperRole)
	}
	return true
}

// getInformers returns a map of informers for the given resources.
func getInformers(factory informers.FactoriesForNamespaces) (map[string]*informers.ForResource, error) {
	serviceMonitorInformers, err := informers.NewInformersForResource(factory, monitoringv1.SchemeGroupVersion.WithResource(monitoringv1.ServiceMonitorName))
	if err != nil {
		return nil, err
	}

	podMonitorInformers, err := informers.NewInformersForResource(factory, monitoringv1.SchemeGroupVersion.WithResource(monitoringv1.PodMonitorName))
	if err != nil {
		return nil, err
	}

	return map[string]*informers.ForResource{
		monitoringv1.ServiceMonitorName: serviceMonitorInformers,
		monitoringv1.PodMonitorName:     podMonitorInformers,
	}, nil
}

// Watch wrapped informers and wait for an initial sync.
func (w *PrometheusCRWatcher) Watch(upstreamEvents chan Event, upstreamErrors chan error) error {
	success := true
	// this channel needs to be buffered because notifications are asynchronous and neither producers nor consumers wait
	notifyEvents := make(chan struct{}, 1)

	for name, resource := range w.informers {
		resource.Start(w.stopChannel)

		if ok := cache.WaitForNamedCacheSync(name, w.stopChannel, resource.HasSynced); !ok {
			success = false
		}

		// only send an event notification if there isn't one already
		resource.AddEventHandler(cache.ResourceEventHandlerFuncs{
			// these functions only write to the notification channel if it's empty to avoid blocking
			// if scrape config updates are being rate-limited
			AddFunc: func(obj interface{}) {
				select {
				case notifyEvents <- struct{}{}:
				default:
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				select {
				case notifyEvents <- struct{}{}:
				default:
				}
			},
			DeleteFunc: func(obj interface{}) {
				select {
				case notifyEvents <- struct{}{}:
				default:
				}
			},
		})
	}
	if !success {
		return fmt.Errorf("failed to sync cache")
	}

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
	close(w.stopChannel)
	return nil
}

func (w *PrometheusCRWatcher) LoadConfig(ctx context.Context) (*promconfig.Config, error) {
	store := assets.NewStoreBuilder(w.k8sClient.CoreV1(), w.k8sClient.CoreV1())
	serviceMonitorInstances := make(map[string]*monitoringv1.ServiceMonitor)
	smRetrieveErr := w.informers[monitoringv1.ServiceMonitorName].ListAll(w.serviceMonitorSelector, func(sm interface{}) {
		monitor := sm.(*monitoringv1.ServiceMonitor)
		// Annotation-based routing: skip monitors that belong to the other agent's scraper role.
		if !w.selectsMonitor("ServiceMonitor", monitor.Namespace, monitor.Name, monitor.GetAnnotations()) {
			return
		}
		key, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(monitor)
		w.addStoreAssetsForServiceMonitor(ctx, monitor.Name, monitor.Namespace, monitor.Spec.Endpoints, store)
		serviceMonitorInstances[key] = monitor
	})
	if smRetrieveErr != nil {
		return nil, smRetrieveErr
	}

	podMonitorInstances := make(map[string]*monitoringv1.PodMonitor)
	pmRetrieveErr := w.informers[monitoringv1.PodMonitorName].ListAll(w.podMonitorSelector, func(pm interface{}) {
		monitor := pm.(*monitoringv1.PodMonitor)
		// Annotation-based routing: skip monitors that belong to the other agent's scraper role.
		if !w.selectsMonitor("PodMonitor", monitor.Namespace, monitor.Name, monitor.GetAnnotations()) {
			return
		}
		key, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(monitor)
		w.addStoreAssetsForPodMonitor(ctx, monitor.Name, monitor.Namespace, monitor.Spec.PodMetricsEndpoints, store)
		podMonitorInstances[key] = monitor
	})
	if pmRetrieveErr != nil {
		return nil, pmRetrieveErr
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

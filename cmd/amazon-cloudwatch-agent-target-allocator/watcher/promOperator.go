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
	prom := &monitoringv1.Prometheus{
		Spec: monitoringv1.PrometheusSpec{
			CommonPrometheusFields: monitoringv1.CommonPrometheusFields{
				ScrapeInterval: monitoringv1.Duration(cfg.PrometheusCR.ScrapeInterval.String()),
			},
		},
	}

	promOperatorLogger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	generator, err := prometheus.NewConfigGenerator(promOperatorLogger, prom)

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
		kubeConfigPath:         cfg.KubeConfigFilePath,
		serviceMonitorSelector: servMonSelector,
		podMonitorSelector:     podMonSelector,
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
	kubeConfigPath       string

	serviceMonitorSelector labels.Selector
	podMonitorSelector     labels.Selector
}

func getSelector(s map[string]string) labels.Selector {
	if s == nil {
		return labels.NewSelector()
	}
	return labels.SelectorFromSet(s)
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
		key, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(monitor)
		w.addStoreAssetsForPodMonitor(ctx, monitor.Name, monitor.Namespace, monitor.Spec.PodMetricsEndpoints, store)
		podMonitorInstances[key] = monitor
	})
	if pmRetrieveErr != nil {
		return nil, pmRetrieveErr
	}

	// Collect all namespaces from service monitors and pod monitors
	namespaces := make(map[string]struct{})
	for _, sm := range serviceMonitorInstances {
		namespaces[sm.Namespace] = struct{}{}
	}
	for _, pm := range podMonitorInstances {
		namespaces[pm.Namespace] = struct{}{}
	}

	// Use the first namespace found, or "default" if none
	promNamespace := "default"
	for ns := range namespaces {
		promNamespace = ns
		break
	}

	// Create a Prometheus object for the config generator
	prom := &monitoringv1.Prometheus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-allocator",
			Namespace: promNamespace,
		},
		Spec: monitoringv1.PrometheusSpec{
			CommonPrometheusFields: monitoringv1.CommonPrometheusFields{
				ScrapeInterval: monitoringv1.Duration("30s"),
				ScrapeTimeout:  monitoringv1.Duration("10s"),
			},
			EvaluationInterval: monitoringv1.Duration("30s"),
		},
	}

	generatedConfig, err := w.configGenerator.GenerateServerConfiguration(
		prom,
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

// addStoreAssetsForPodMonitor adds authentication / authorization related information to the assets store,
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

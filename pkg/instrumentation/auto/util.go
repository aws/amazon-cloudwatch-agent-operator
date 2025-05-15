// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"context"
	"encoding/json"
	"fmt"
	"k8s.io/api/apps/v1"
	v2 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"os"

	"github.com/go-logr/logr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
)

// configureAutoAnnotation handles the auto annotation configuration logic
func configureAutoAnnotation(autoAnnotationConfigStr string, client client.Client, reader client.Reader, setupLog logr.Logger) (InstrumentationAnnotator, error) {
	// Check environment variables first
	if os.Getenv("DISABLE_AUTO_ANNOTATION") == "true" {
		setupLog.Info("detected DISABLE_AUTO_ANNOTATION environment variable, disabling AutoAnnotation")
		return nil, nil
	}

	if autoAnnotationConfigStr == "" {
		return nil, fmt.Errorf("auto-annotation configuration not provided, disabling AutoAnnotation")
	}

	var autoAnnotationConfig AnnotationConfig
	if err := json.Unmarshal([]byte(autoAnnotationConfigStr), &autoAnnotationConfig); err != nil {
		return nil, fmt.Errorf("unable to unmarshal auto-annotation config, disabling AutoAnnotation: %w", err)
	}

	if autoAnnotationConfig.Empty() {
		return nil, fmt.Errorf("AutoAnnotation configuration is empty, disabling AutoAnnotation")
	}

	setupLog.Info("W! Using deprecated autoAnnotateAutoInstrumentation config, Disabling AutoMonitor. Please upgrade to AutoMonitor. autoAnnotateAutoInstrumentation will be removed in a future release.")
	return NewAnnotationMutators(
		client,
		reader,
		setupLog,
		autoAnnotationConfig,
		instrumentation.SupportedTypes,
	), nil
}

// configureAutoMonitor handles the auto monitor configuration logic
func configureAutoMonitor(ctx context.Context, autoMonitorConfigStr string, clientSet kubernetes.Interface, client client.Client, reader client.Reader, setupLog logr.Logger) (*Monitor, error) {
	// If auto-annotation is not configured or failed, try auto-monitor
	if os.Getenv("DISABLE_AUTO_MONITOR") == "true" {
		setupLog.Info("W! auto-monitor is disabled due to DISABLE_AUTO_MONITOR environment variable")
		return nil, nil
	}

	var autoMonitorConfig *MonitorConfig
	if err := json.Unmarshal([]byte(autoMonitorConfigStr), &autoMonitorConfig); err != nil {
		return nil, fmt.Errorf("unable to unmarshal auto-monitor config: %w", err)
	}

	logger := ctrl.Log.WithName("auto_monitor")
	return NewMonitor(ctx, *autoMonitorConfig, clientSet, client, reader, logger), nil
}

// CreateInstrumentationAnnotator creates an instrumentationAnnotator based on config and environment. Returns the InstrumentationAnnotator and whether AutoMonitor is enabled.
func CreateInstrumentationAnnotator(autoMonitorConfigStr string, autoAnnotationConfigStr string, ctx context.Context, client client.Client, reader client.Reader, setupLog logr.Logger) (InstrumentationAnnotator, bool) {
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		setupLog.Error(err, "unable to create in-cluster config")
	}

	clientSet, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		setupLog.Error(err, "unable to create clientset")
	}
	return createInstrumentationAnnotatorWithClientset(autoMonitorConfigStr, autoAnnotationConfigStr, ctx, clientSet, client, reader, setupLog)
}

// for testing
func createInstrumentationAnnotatorWithClientset(autoMonitorConfigStr string, autoAnnotationConfigStr string, ctx context.Context, clientSet kubernetes.Interface, client client.Client, reader client.Reader, setupLog logr.Logger) (InstrumentationAnnotator, bool) {
	autoAnnotation, err := configureAutoAnnotation(autoAnnotationConfigStr, client, reader, setupLog)
	if err != nil {
		setupLog.Error(err, "Failed to configure auto-annotation, trying AutoMonitor")
	} else if autoAnnotation != nil {
		return autoAnnotation, false
	}

	monitor, err := configureAutoMonitor(ctx, autoMonitorConfigStr, clientSet, client, reader, setupLog)
	if err != nil {
		setupLog.Error(err, "Failed to configure auto-monitor")
		return nil, false
	} else if monitor != nil {
		return monitor, monitor.config.MonitorAllServices
	}

	return nil, false
}

func createStatefulsetInformer(workloadFactory informers.SharedInformerFactory, err error) (cache.SharedIndexInformer, error) {
	statefulSetInformer := workloadFactory.Apps().V1().StatefulSets().Informer()
	err = statefulSetInformer.SetTransform(func(obj interface{}) (interface{}, error) {
		statefulSet, ok := obj.(*v1.StatefulSet)
		if !ok {
			return obj, fmt.Errorf("error transforming statefulset: %s not a statefulset", obj)
		}
		return &v1.StatefulSet{
			ObjectMeta: v2.ObjectMeta{
				Name:      statefulSet.Name,
				Namespace: statefulSet.Namespace,
			},
			Spec: v1.StatefulSetSpec{
				Template: statefulSet.Spec.Template,
			},
		}, nil
	})
	if err != nil {
		return nil, err
	}

	err = statefulSetInformer.AddIndexers(map[string]cache.IndexFunc{
		ByLabel: func(obj interface{}) ([]string, error) {
			return []string{labels.SelectorFromSet(obj.(*v1.StatefulSet).Spec.Template.Labels).String()}, nil
		},
	})
	return statefulSetInformer, err
}

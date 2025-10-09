// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
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

	resources, err := clientSet.Discovery().ServerResourcesForGroupVersion("opentelemetry.io/v1alpha1")
	if err == nil {
		for _, r := range resources.APIResources {
			if r.Name == "instrumentations" {
				setupLog.Info("W! auto-monitor is disabled due to the presence of opentelemetry.io/v1alpha1 group version")
				return nil, nil
			}
		}
	} else {
		if !errors.IsNotFound(err) {
			setupLog.Info(fmt.Sprintf("W! auto-monitor is disabled due to failures in retrieving server groups: %v", err))
			return nil, nil
		}
	}

	logger := ctrl.Log.WithName("auto_monitor")
	return NewMonitor(ctx, *autoMonitorConfig, clientSet, client, reader, logger), nil
}

// CreateInstrumentationAnnotator creates an instrumentationAnnotator based on config and environment. Returns the InstrumentationAnnotator and whether AutoMonitor is enabled.
func CreateInstrumentationAnnotator(autoMonitorConfigStr string, autoAnnotationConfigStr string, ctx context.Context, client client.Client, reader client.Reader, setupLog logr.Logger) InstrumentationAnnotator {
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
func createInstrumentationAnnotatorWithClientset(autoMonitorConfigStr string, autoAnnotationConfigStr string, ctx context.Context, clientSet kubernetes.Interface, client client.Client, reader client.Reader, setupLog logr.Logger) InstrumentationAnnotator {
	autoAnnotation, err := configureAutoAnnotation(autoAnnotationConfigStr, client, reader, setupLog)
	if err != nil {
		setupLog.Error(err, "Failed to configure auto-annotation, trying AutoMonitor")
	} else if autoAnnotation != nil {
		return autoAnnotation
	}

	monitor, err := configureAutoMonitor(ctx, autoMonitorConfigStr, clientSet, client, reader, setupLog)
	if err != nil {
		setupLog.Error(err, "Failed to configure auto-monitor")
		return nil
	} else if monitor != nil {
		return monitor
	}

	return nil
}

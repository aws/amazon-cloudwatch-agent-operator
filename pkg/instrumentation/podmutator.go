// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"context"
	"errors"
	"strings"

	"github.com/go-logr/logr"
	"github.com/open-telemetry/opentelemetry-operator/pkg/featuregate"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/webhookhandler"
)

const (
	defaultExporterEndpoint                = "http://amazon-cloudwatch-agent.amazon-cloudwatch:4317"
	defaultJavaImage                       = "160148376629.dkr.ecr.us-west-2.amazonaws.com/aws-apm-preview:latest"
	defaultAPIVersion                      = "cloudwatch.aws.amazon.com/v1alpha1"
	defaultInstrumenation                  = "java-instrumentation"
	defaultNamespace                       = "default"
	defaultKind                            = "Instrumentation"
	otelSampleEnabledKey                   = "OTEL_SMP_ENABLED"
	otelSampleEnabledDefaultValue          = "true"
	otelTracesSamplerArgKey                = "OTEL_TRACES_SAMPLER_ARG"
	otelTracesSamplerArgDefaultValue       = "0.05"
	otelTracesSamplerKey                   = "OTEL_TRACES_SAMPLER"
	otelTracesSamplerDefaultValue          = "parentbased_traceidratio"
	otelExporterTracesEndpointKey          = "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"
	otelExporterTracesEndpointDefaultValue = "http://amazon-cloudwatch-agent.amazon-cloudwatch:4317"
)

var (
	errMultipleInstancesPossible = errors.New("multiple OpenTelemetry Instrumentation instances available, cannot determine which one to select")

	defaultInst = &v1alpha1.Instrumentation{
		Status: v1alpha1.InstrumentationStatus{},
		TypeMeta: metav1.TypeMeta{
			APIVersion: defaultAPIVersion,
			Kind:       defaultKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultInstrumenation,
			Namespace: defaultNamespace,
		},
		Spec: v1alpha1.InstrumentationSpec{
			Exporter: v1alpha1.Exporter{Endpoint: defaultExporterEndpoint},
			Propagators: []v1alpha1.Propagator{
				v1alpha1.TraceContext,
				v1alpha1.Baggage,
				v1alpha1.B3,
				v1alpha1.XRay,
			},
			Java: v1alpha1.Java{
				Image: defaultJavaImage,
				Env: []corev1.EnvVar{
					{Name: otelSampleEnabledKey, Value: otelSampleEnabledDefaultValue},
					{Name: otelTracesSamplerArgKey, Value: otelTracesSamplerArgDefaultValue},
					{Name: otelTracesSamplerKey, Value: otelTracesSamplerDefaultValue},
					{Name: otelExporterTracesEndpointKey, Value: otelExporterTracesEndpointDefaultValue},
				},
			},
		},
	}
)

type instPodMutator struct {
	Client      client.Client
	sdkInjector *sdkInjector
	Logger      logr.Logger
	Recorder    record.EventRecorder
}

type languageInstrumentations struct {
	Java *v1alpha1.Instrumentation
	Sdk  *v1alpha1.Instrumentation
}

var _ webhookhandler.PodMutator = (*instPodMutator)(nil)

func NewMutator(logger logr.Logger, client client.Client, recorder record.EventRecorder) *instPodMutator {
	return &instPodMutator{
		Logger: logger,
		Client: client,
		sdkInjector: &sdkInjector{
			logger: logger,
			client: client,
		},
		Recorder: recorder,
	}
}

func (pm *instPodMutator) Mutate(ctx context.Context, namespace corev1.Namespace, pod corev1.Pod) (corev1.Pod, error) {
	logger := pm.Logger.WithValues("namespace", pod.Namespace, "name", pod.Name)

	// We check if Pod is already instrumented.
	if isAutoInstrumentationInjected(pod) {
		logger.Info("Skipping pod instrumentation - already instrumented")
		return pod, nil
	}

	var inst *v1alpha1.Instrumentation
	var err error

	insts := languageInstrumentations{}

	// We bail out if any annotation fails to process.

	if inst, err = pm.getInstrumentationInstance(ctx, namespace, pod, annotationInjectJava); err != nil {
		// we still allow the pod to be created, but we log a message to the operator's logs
		logger.Error(err, "failed to select an OpenTelemetry Instrumentation instance for this pod")
		return pod, err
	}
	if featuregate.EnableJavaAutoInstrumentationSupport.IsEnabled() || inst == nil {
		insts.Java = inst
	} else {
		logger.Error(nil, "support for Java auto instrumentation is not enabled")
		pm.Recorder.Event(pod.DeepCopy(), "Warning", "InstrumentationRequestRejected", "support for Java auto instrumentation is not enabled")
	}

	if inst, err = pm.getInstrumentationInstance(ctx, namespace, pod, annotationInjectSdk); err != nil {
		// we still allow the pod to be created, but we log a message to the operator's logs
		logger.Error(err, "failed to select an OpenTelemetry Instrumentation instance for this pod")
		return pod, err
	}
	insts.Sdk = inst

	if insts.Java == nil && insts.Sdk == nil {
		logger.V(1).Info("annotation not present in deployment, skipping instrumentation injection")
		return pod, nil
	}

	// We retrieve the annotation for podname
	var targetContainers = annotationValue(namespace.ObjectMeta, pod.ObjectMeta, annotationInjectContainerName)

	// once it's been determined that instrumentation is desired, none exists yet, and we know which instance it should talk to,
	// we should inject the instrumentation.
	modifiedPod := pod
	for _, currentContainer := range strings.Split(targetContainers, ",") {
		modifiedPod = pm.sdkInjector.inject(ctx, insts, namespace, modifiedPod, strings.TrimSpace(currentContainer))
	}

	return modifiedPod, nil
}

func (pm *instPodMutator) getInstrumentationInstance(ctx context.Context, namespace corev1.Namespace, pod corev1.Pod, instAnnotation string) (*v1alpha1.Instrumentation, error) {
	instValue := annotationValue(namespace.ObjectMeta, pod.ObjectMeta, instAnnotation)

	if len(instValue) == 0 || strings.EqualFold(instValue, "false") {
		return nil, nil
	}

	if strings.EqualFold(instValue, "true") {
		return pm.selectInstrumentationInstanceFromNamespace(ctx, namespace)
	}

	var instNamespacedName types.NamespacedName
	if instNamespace, instName, namespaced := strings.Cut(instValue, "/"); namespaced {
		instNamespacedName = types.NamespacedName{Name: instName, Namespace: instNamespace}
	} else {
		instNamespacedName = types.NamespacedName{Name: instValue, Namespace: namespace.Name}
	}

	otelInst := &v1alpha1.Instrumentation{}
	err := pm.Client.Get(ctx, instNamespacedName, otelInst)
	if err != nil {
		return nil, err
	}

	return otelInst, nil
}

func (pm *instPodMutator) selectInstrumentationInstanceFromNamespace(ctx context.Context, namespace corev1.Namespace) (*v1alpha1.Instrumentation, error) {
	var otelInsts v1alpha1.InstrumentationList
	if err := pm.Client.List(ctx, &otelInsts, client.InNamespace(namespace.Name)); err != nil {
		return nil, err
	}
	switch items := len(otelInsts.Items); {
	case items == 0:
		pm.Logger.Info("no OpenTelemetry Instrumentation instances available. Using default Instrumentation instance")
		return defaultInst, nil
	case items > 1:
		return nil, errMultipleInstancesPossible
	default:
		return &otelInsts.Items[0], nil
	}
}

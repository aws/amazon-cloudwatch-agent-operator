// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/constants"
)

const (
	// CloudWatch agent service endpoints
	cloudwatchAgentStandardEndpoint = "cloudwatch-agent.amazon-cloudwatch"
	cloudwatchAgentWindowsEndpoint  = "cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local"
)

// ADOT/OTel environment variable names
const (
	EnvOTelApplicationSignalsEnabled   = "OTEL_AWS_APPLICATION_SIGNALS_ENABLED"
	EnvOTelExporterOTLPEndpoint        = "OTEL_EXPORTER_OTLP_ENDPOINT"
	EnvOTelExporterOTLPTracesEndpoint  = "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"
	EnvOTelExporterOTLPMetricsEndpoint = "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"
	EnvOTelExporterOTLPLogsEndpoint    = "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT"
)

var defaultSize = resource.MustParse("200Mi")

// Calculate if we already inject InitContainers.
func isInitContainerMissing(pod corev1.Pod, containerName string) bool {
	for _, initContainer := range pod.Spec.InitContainers {
		if initContainer.Name == containerName {
			return false
		}
	}
	return true
}

// Checks if Pod is already instrumented by checking Instrumentation InitContainer presence.
func isAutoInstrumentationInjected(pod corev1.Pod) bool {
	for _, cont := range pod.Spec.InitContainers {
		if slices.Contains([]string{
			dotnetInitContainerName,
			javaInitContainerName,
			nodejsInitContainerName,
			pythonInitContainerName,
			apacheAgentInitContainerName,
			apacheAgentCloneContainerName,
		}, cont.Name) {
			return true
		}
	}

	for _, cont := range pod.Spec.Containers {
		// Go uses a sidecar
		if cont.Name == sideCarName {
			return true
		}

		// This environment variable is set in the sidecar and in the
		// collector containers. We look for it in any container that is not
		// the sidecar container to check if we already injected the
		// instrumentation or not
		if cont.Name != naming.Container() {
			for _, envVar := range cont.Env {
				if envVar.Name == constants.EnvNodeName {
					return true
				}
			}
		}
	}
	return false
}

// Look for duplicates in the provided containers.
func findDuplicatedContainers(ctrs []string) error {
	// Merge is needed because of multiple containers can be provided for single instrumentation.
	mergedContainers := strings.Join(ctrs, ",")

	// Split all containers.
	splitContainers := strings.Split(mergedContainers, ",")

	countMap := make(map[string]int)
	var duplicates []string
	for _, str := range splitContainers {
		countMap[str]++
	}

	// Find and collect the duplicates
	for str, count := range countMap {
		// omit empty container names
		if str == "" {
			continue
		}

		if count > 1 {
			duplicates = append(duplicates, str)
		}
	}

	if duplicates != nil {
		sort.Strings(duplicates)
		return fmt.Errorf("duplicated container names detected: %s", duplicates)
	}

	return nil
}

// Return positive for instrumentation with defined containers.
func isInstrWithContainers(inst instrumentationWithContainers) int {
	if inst.Containers != "" {
		return 1
	}

	return 0
}

// Return positive for instrumentation without defined containers.
func isInstrWithoutContainers(inst instrumentationWithContainers) int {
	if inst.Containers == "" {
		return 1
	}

	return 0
}

func volumeSize(quantity *resource.Quantity) *resource.Quantity {
	if quantity == nil {
		return &defaultSize
	}
	return quantity
}

// containsCloudWatchAgent checks if the endpoint's hostname is a CloudWatch agent service endpoint
func containsCloudWatchAgent(endpoint string) bool {
	// Check if the CloudWatch agent endpoint appears after the protocol separator (://)
	// This ensures we're matching the hostname, not a substring in the path
	return strings.Contains(endpoint, "://"+cloudwatchAgentStandardEndpoint) ||
		strings.Contains(endpoint, "://"+cloudwatchAgentWindowsEndpoint)
}

// getEnvValue returns the value of an environment variable from the container's env list
func getEnvValue(envs []corev1.EnvVar, name string) string {
	for _, env := range envs {
		if env.Name == name {
			return env.Value
		}
	}
	return ""
}

// isApplicationSignalsExplicitlyEnabled checks if OTEL_AWS_APPLICATION_SIGNALS_ENABLED is explicitly set to true
func isApplicationSignalsExplicitlyEnabled(envs []corev1.EnvVar) bool {
	value := getEnvValue(envs, EnvOTelApplicationSignalsEnabled)
	return strings.EqualFold(value, "true")
}

func isApplicationSignalsExplicitlyDisabled(envs []corev1.EnvVar) bool {
	value := getEnvValue(envs, EnvOTelApplicationSignalsEnabled)
	return strings.EqualFold(value, "false")
}

// resolveEnvFrom fetches ConfigMap data referenced by envFrom and returns as EnvVar slice
// Uses caches to avoid redundant API calls when multiple containers reference the same ConfigMap
func resolveEnvFrom(ctx context.Context, k8sClient client.Client, envFromSources []corev1.EnvFromSource, namespace string, logger logr.Logger, configMapCache map[string]*corev1.ConfigMap) []corev1.EnvVar {
	var resolvedEnvs []corev1.EnvVar

	for _, envFromSource := range envFromSources {
		if envFromSource.ConfigMapRef != nil {
			cmName := envFromSource.ConfigMapRef.Name
			var configMap *corev1.ConfigMap

			// Check cache first, only fetch ConfigMap once for all containers
			if cached, exists := configMapCache[cmName]; exists {
				configMap = cached
				logger.V(1).Info("using cached ConfigMap from envFrom",
					"configMap", cmName,
					"namespace", namespace)
			} else {
				// Fetch ConfigMap
				configMap = &corev1.ConfigMap{}
				err := k8sClient.Get(ctx, client.ObjectKey{
					Name:      cmName,
					Namespace: namespace,
				}, configMap)

				if err != nil {
					logger.Error(err, "failed to fetch ConfigMap for envFrom",
						"configMap", cmName,
						"namespace", namespace)
					continue
				}

				// Store in cache
				configMapCache[cmName] = configMap
				logger.V(1).Info("fetched and cached ConfigMap from envFrom",
					"configMap", cmName,
					"envCount", len(configMap.Data))
			}

			// Convert ConfigMap data to EnvVar slice
			for key, value := range configMap.Data {
				resolvedEnvs = append(resolvedEnvs, corev1.EnvVar{
					Name:  key,
					Value: value,
				})
			}
		}
	}

	return resolvedEnvs
}

// getAllEnvVars combines direct env vars and envFrom-resolved vars
// Always processes both direct env and envFrom for consistency, using caches to optimize performance
func getAllEnvVars(ctx context.Context, k8sClient client.Client, container *corev1.Container, namespace string, logger logr.Logger, configMapCache map[string]*corev1.ConfigMap) []corev1.EnvVar {
	allEnvs := make([]corev1.EnvVar, len(container.Env))
	copy(allEnvs, container.Env)

	// Always resolve envFrom sources for consistency (even if empty)
	if len(container.EnvFrom) > 0 {
		resolvedEnvs := resolveEnvFrom(ctx, k8sClient, container.EnvFrom, namespace, logger, configMapCache)

		// envFrom has lower precedence than direct env
		// Build map of existing env var names for O(1) lookup
		envMap := make(map[string]bool, len(allEnvs))
		for _, env := range allEnvs {
			envMap[env.Name] = true
		}

		// Add resolved envs only if not already defined in direct env
		for _, resolvedEnv := range resolvedEnvs {
			if !envMap[resolvedEnv.Name] {
				allEnvs = append(allEnvs, resolvedEnv)
			}
		}
	}

	return allEnvs
}

// shouldInjectADOTSDK determines if the ADOT SDK should be injected based on existing environment variables
// and the pod/container security context
func shouldInjectADOTSDK(envs []corev1.EnvVar, pod corev1.Pod, container *corev1.Container) bool {
	// Check Pod-level SecurityContext for runAsNonRoot without runAsUser
	// Pod-level SecurityContext inherits to init containers, so we must check it first
	podRunAsUser := int64(-1)
	if pod.Spec.SecurityContext != nil {
		podSC := pod.Spec.SecurityContext
		if podSC.RunAsUser != nil {
			podRunAsUser = *podSC.RunAsUser
		}
		if podSC.RunAsNonRoot != nil && *podSC.RunAsNonRoot && podSC.RunAsUser == nil {
			// Pod requires non-root but doesn't specify UID - init container will fail
			// Container-level runAsUser will NOT help because it doesn't inherit to init containers
			return false
		}
	}

	// Check container-level SecurityContext for runAsNonRoot without runAsUser
	// While container-level SecurityContext does not technically inherit to init containers,
	// cluster policies or admission controllers may enforce security requirements across all containers
	if container.SecurityContext != nil {
		containerSC := container.SecurityContext
		// Determine effective runAsUser for this container (container overrides pod)
		effectiveRunAsUser := podRunAsUser
		if containerSC.RunAsUser != nil {
			effectiveRunAsUser = *containerSC.RunAsUser
		}
		// If container has runAsNonRoot without an effective runAsUser, skip injection
		if containerSC.RunAsNonRoot != nil && *containerSC.RunAsNonRoot && effectiveRunAsUser == -1 {
			return false
		}
	}

	// If Application Signals is explicitly enabled, always inject regardless of endpoint configuration
	if isApplicationSignalsExplicitlyEnabled(envs) {
		return true
	}

	// If Application Signals is not explicitly enabled, check all OTLP endpoint configurations
	// Skip injection if any endpoint is configured to a third-party (non-CloudWatch) endpoint

	// Check OTEL_EXPORTER_OTLP_ENDPOINT
	otlpEndpoint := getEnvValue(envs, EnvOTelExporterOTLPEndpoint)
	if otlpEndpoint != "" && !containsCloudWatchAgent(otlpEndpoint) {
		return false
	}

	// Check OTEL_EXPORTER_OTLP_TRACES_ENDPOINT
	tracesEndpoint := getEnvValue(envs, EnvOTelExporterOTLPTracesEndpoint)
	if tracesEndpoint != "" && !containsCloudWatchAgent(tracesEndpoint) {
		return false
	}

	// Check OTEL_EXPORTER_OTLP_METRICS_ENDPOINT
	metricsEndpoint := getEnvValue(envs, EnvOTelExporterOTLPMetricsEndpoint)
	if metricsEndpoint != "" && !containsCloudWatchAgent(metricsEndpoint) {
		return false
	}

	// Check OTEL_EXPORTER_OTLP_LOGS_ENDPOINT
	logsEndpoint := getEnvValue(envs, EnvOTelExporterOTLPLogsEndpoint)
	if logsEndpoint != "" && !containsCloudWatchAgent(logsEndpoint) {
		return false
	}

	// Default: inject if no custom endpoints are configured and no problematic security context
	return true
}

// shouldInjectEnvVar determines whether a specific environment variable should be injected
// based on its name and the existing environment variables in the container
func shouldInjectEnvVar(envs []corev1.EnvVar, envName string) bool {
	// If the environment variable is already set by user, don't override it
	if getEnvValue(envs, envName) != "" {
		return false
	}

	// If Application Signals is explicitly disabled, skip all OTEL configuration overrides
	// This allows users to configure their own OTel settings when not using Application Signals
	if isApplicationSignalsExplicitlyDisabled(envs) {
		return false
	}

	// Inject if not already set
	return true
}

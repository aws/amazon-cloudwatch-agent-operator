// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"fmt"
	"sort"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/adapters"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

// maxPortLen allows us to truncate a port name according to what is considered valid port syntax:
// https://pkg.go.dev/k8s.io/apimachinery/pkg/util/validation#IsValidPortName
const maxPortLen = 15

// Container builds a container for the given collector.
func Container(cfg config.Config, logger logr.Logger, agent v1alpha1.AmazonCloudWatchAgent, addConfig bool) corev1.Container {
	image := agent.Spec.Image
	if len(image) == 0 {
		image = cfg.CollectorImage()
	}

	ports := getContainerPorts(logger, agent.Spec.Config, agent.Spec.Ports)

	var volumeMounts []corev1.VolumeMount
	argsMap := agent.Spec.Args
	if argsMap == nil {
		argsMap = map[string]string{}
	}
	// defines the output (sorted) array for final output
	var args []string
	// When adding a config via v1alpha1.AmazonCloudWatchAgentSpec.Config, we ensure that it is always the
	// first item in the args. At the time of writing, although multiple configs are allowed in the
	// cloudwatch agent, the operator has yet to implement such functionality.  When multiple configs
	// are present they should be merged in a deterministic manner using the order given, and because
	// v1alpha1.AmazonCloudWatchAgentSpec.Config is a required field we assume that it will always be the
	// "primary" config and in the future additional configs can be appended to the container args in a simple manner.

	if addConfig {
		volumeMounts = append(volumeMounts, getVolumeMounts(agent.Spec.NodeSelector["kubernetes.io/os"]))
	}

	// ensure that the v1alpha1.AmazonCloudWatchAgentSpec.Args are ordered when moved to container.Args,
	// where iterating over a map does not guarantee, so that reconcile will not be fooled by different
	// ordering in args.
	var sortedArgs []string
	for k, v := range argsMap {
		sortedArgs = append(sortedArgs, fmt.Sprintf("--%s=%s", k, v))
	}
	sort.Strings(sortedArgs)
	args = append(args, sortedArgs...)

	if len(agent.Spec.VolumeMounts) > 0 {
		volumeMounts = append(volumeMounts, agent.Spec.VolumeMounts...)
	}

	var envVars = agent.Spec.Env
	if agent.Spec.Env == nil {
		envVars = []corev1.EnvVar{}
	}

	envVars = append(envVars, corev1.EnvVar{
		Name: "POD_NAME",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.name",
			},
		},
	})

	if _, err := adapters.ConfigFromJSONString(agent.Spec.Config); err != nil {
		logger.Error(err, "error parsing config")
	}

	if agent.Spec.TargetAllocator.Enabled {
		// We need to add a SHARD here so the collector is able to keep targets after the hashmod operation which is
		// added by default by the Prometheus operator's config generator.
		// All collector instances use SHARD == 0 as they only receive targets
		// allocated to them and should not use the Prometheus hashmod-based
		// allocation.
		envVars = append(envVars, corev1.EnvVar{
			Name:  "SHARD",
			Value: "0",
		})
	}

	return corev1.Container{
		Name:            naming.Container(),
		Image:           image,
		ImagePullPolicy: agent.Spec.ImagePullPolicy,
		WorkingDir:      agent.Spec.WorkingDir,
		VolumeMounts:    volumeMounts,
		Args:            args,
		Env:             envVars,
		EnvFrom:         agent.Spec.EnvFrom,
		Resources:       agent.Spec.Resources,
		Ports:           portMapToContainerPortList(ports),
		SecurityContext: agent.Spec.SecurityContext,
		Lifecycle:       agent.Spec.Lifecycle,
	}
}

func getVolumeMounts(os string) corev1.VolumeMount {
	var volumeMount corev1.VolumeMount
	if os == "windows" {
		volumeMount = corev1.VolumeMount{
			Name:      naming.ConfigMapVolume(),
			MountPath: "C:\\Program Files\\Amazon\\AmazonCloudWatchAgent\\cwagentconfig",
		}
	} else {
		volumeMount = corev1.VolumeMount{
			Name:      naming.ConfigMapVolume(),
			MountPath: "/etc/cwagentconfig",
		}
	}
	return volumeMount
}

func portMapToContainerPortList(portMap map[string]corev1.ContainerPort) []corev1.ContainerPort {
	ports := make([]corev1.ContainerPort, 0, len(portMap))
	for _, p := range portMap {
		ports = append(ports, p)
	}
	sort.Slice(ports, func(i, j int) bool {
		return ports[i].Name < ports[j].Name
	})
	return ports
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/naming"
)

// DaemonSet builds the deployment for the given instance.
func DaemonSet(cfg config.Config, logger logr.Logger, agent v1alpha1.AmazonCloudWatchAgent) appsv1.DaemonSet {
	name := naming.Agent(agent)
	labels := Labels(agent, name, cfg.LabelsFilter())

	annotations := Annotations(agent)
	podAnnotations := PodAnnotations(agent)
	return appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        naming.Agent(agent),
			Namespace:   agent.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: SelectorLabels(agent),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: podAnnotations,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: ServiceAccountName(agent),
					Containers:         []corev1.Container{Container(cfg, logger, agent, true)},
					Volumes:            Volumes(cfg, agent),
					Tolerations:        agent.Spec.Tolerations,
					NodeSelector:       agent.Spec.NodeSelector,
					HostNetwork:        agent.Spec.HostNetwork,
					DNSPolicy:          getDNSPolicy(agent),
					PriorityClassName:  agent.Spec.PriorityClassName,
				},
			},
		},
	}
}

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

// Deployment builds the deployment for the given instance.
func Deployment(cfg config.Config, logger logr.Logger, otelcol v1alpha1.AmazonCloudWatchAgent) appsv1.Deployment {
	name := naming.Agent(otelcol)
	labels := Labels(otelcol, name, cfg.LabelsFilter())

	annotations := Annotations(otelcol)
	podAnnotations := PodAnnotations(otelcol)

	return appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   otelcol.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: otelcol.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: SelectorLabels(otelcol),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: podAnnotations,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: ServiceAccountName(otelcol),
					Containers:         []corev1.Container{Container(cfg, logger, otelcol, true)},
					Volumes:            Volumes(cfg, otelcol),
					DNSPolicy:          getDNSPolicy(otelcol),
					HostNetwork:        otelcol.Spec.HostNetwork,
					Tolerations:        otelcol.Spec.Tolerations,
					NodeSelector:       otelcol.Spec.NodeSelector,
					PriorityClassName:  otelcol.Spec.PriorityClassName,
				},
			},
		},
	}
}

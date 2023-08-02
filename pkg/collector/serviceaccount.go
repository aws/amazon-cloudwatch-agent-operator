package collector

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/naming"
)

// ServiceAccountName returns the name of the existing or self-provisioned service account to use for the given instance.
func ServiceAccountName(instance v1alpha1.AmazonCloudWatchAgent) string {
	if len(instance.Spec.ServiceAccount) == 0 {
		return naming.ServiceAccount(instance)
	}

	return instance.Spec.ServiceAccount
}

// ServiceAccount returns the service account for the given instance.
func ServiceAccount(otelcol v1alpha1.AmazonCloudWatchAgent) corev1.ServiceAccount {
	name := naming.ServiceAccount(otelcol)
	labels := Labels(otelcol, name, []string{})

	return corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   otelcol.Namespace,
			Labels:      labels,
			Annotations: otelcol.Annotations,
		},
	}
}

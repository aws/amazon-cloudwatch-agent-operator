package v1alpha1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var amazoncloudwatchagentlog = logf.Log.WithName("amazoncloudwatchagent-resource")

func (r *AmazonCloudWatchAgent) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-cloudwatch-aws-amazon-com-v1alpha1-amazoncloudwatchagent,mutating=true,failurePolicy=fail,groups=cloudwatch.aws.amazon.com,resources=amazoncloudwatchagents,verbs=create;update,versions=v1alpha1,name=mamazoncloudwatchagent.kb.io,sideEffects=none,admissionReviewVersions=v1

var _ webhook.Defaulter = &AmazonCloudWatchAgent{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *AmazonCloudWatchAgent) Default() {
	amazoncloudwatchagentlog.Info("default", "name", r.Name)

	if len(r.Spec.Mode) == 0 {
		r.Spec.Mode = ModeDeployment
	}
	if len(r.Spec.UpgradeStrategy) == 0 {
		r.Spec.UpgradeStrategy = UpgradeStrategyAutomatic
	}

	if r.Labels == nil {
		r.Labels = map[string]string{}
	}
	if r.Labels["app.kubernetes.io/managed-by"] == "" {
		r.Labels["app.kubernetes.io/managed-by"] = "amazon-cloudwatch-agent-operator"
	}

	// We can default to one because dependent objects Deployment and HorizontalPodAutoScaler
	// default to 1 as well.
	one := int32(1)
	if r.Spec.Replicas == nil {
		r.Spec.Replicas = &one
	}

	if r.Spec.Ingress.Type == IngressTypeRoute && r.Spec.Ingress.Route.Termination == "" {
		r.Spec.Ingress.Route.Termination = TLSRouteTerminationTypeEdge
	}
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-cloudwatch-aws-amazon-com-v1alpha1-amazoncloudwatchagent,mutating=false,failurePolicy=fail,groups=cloudwatch.aws.amazon.com,resources=amazoncloudwatchagents,versions=v1alpha1,name=vamazoncloudwatchagentcreateupdate.kb.io,sideEffects=none,admissionReviewVersions=v1
// +kubebuilder:webhook:verbs=delete,path=/validate-cloudwatch-aws-amazon-com-v1alpha1-amazoncloudwatchagent,mutating=false,failurePolicy=ignore,groups=cloudwatch.aws.amazon.com,resources=amazoncloudwatchagents,versions=v1alpha1,name=vamazoncloudwatchagentdelete.kb.io,sideEffects=none,admissionReviewVersions=v1

var _ webhook.Validator = &AmazonCloudWatchAgent{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *AmazonCloudWatchAgent) ValidateCreate() (admission.Warnings, error) {
	amazoncloudwatchagentlog.Info("validate create", "name", r.Name)
	return nil, r.validateCRDSpec()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *AmazonCloudWatchAgent) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	amazoncloudwatchagentlog.Info("validate update", "name", r.Name)
	return nil, r.validateCRDSpec()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *AmazonCloudWatchAgent) ValidateDelete() (admission.Warnings, error) {
	amazoncloudwatchagentlog.Info("validate delete", "name", r.Name)
	return nil, nil
}

func (r *AmazonCloudWatchAgent) validateCRDSpec() error {
	// validate volumeClaimTemplates
	if r.Spec.Mode != ModeStatefulSet && len(r.Spec.VolumeClaimTemplates) > 0 {
		return fmt.Errorf("the OpenTelemetry Collector mode is set to %s, which does not support the attribute 'volumeClaimTemplates'", r.Spec.Mode)
	}

	// validate tolerations
	if r.Spec.Mode == ModeSidecar && len(r.Spec.Tolerations) > 0 {
		return fmt.Errorf("the OpenTelemetry Collector mode is set to %s, which does not support the attribute 'tolerations'", r.Spec.Mode)
	}

	// validate priorityClassName
	if r.Spec.Mode == ModeSidecar && r.Spec.PriorityClassName != "" {
		return fmt.Errorf("the OpenTelemetry Collector mode is set to %s, which does not support the attribute 'priorityClassName'", r.Spec.Mode)
	}

	// validator port config
	for _, p := range r.Spec.Ports {
		nameErrs := validation.IsValidPortName(p.Name)
		numErrs := validation.IsValidPortNum(int(p.Port))
		if len(nameErrs) > 0 || len(numErrs) > 0 {
			return fmt.Errorf("the AmazonCloudWatchAgent Spec Ports configuration is incorrect, port name '%s' errors: %s, num '%d' errors: %s",
				p.Name, nameErrs, p.Port, numErrs)
		}
	}

	if r.Spec.Ingress.Type == IngressTypeNginx && r.Spec.Mode == ModeSidecar {
		return fmt.Errorf("the AmazonCloudWatchAgent Spec Ingress configuration is incorrect. Ingress can only be used in combination with the modes: %s, %s, %s",
			ModeDeployment, ModeDaemonSet, ModeStatefulSet,
		)
	}

	if r.Spec.Ingress.Type == IngressTypeNginx && r.Spec.Mode == ModeSidecar {
		return fmt.Errorf("the AmazonCloudWatchAgent Spec Ingress configuiration is incorrect. Ingress can only be used in combination with the modes: %s, %s, %s",
			ModeDeployment, ModeDaemonSet, ModeStatefulSet,
		)
	}

	return nil
}

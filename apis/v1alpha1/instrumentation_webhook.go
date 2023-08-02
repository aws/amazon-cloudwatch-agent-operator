package v1alpha1

import (
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	AnnotationDefaultAutoInstrumentationJava = "instrumentation.opentelemetry.io/default-auto-instrumentation-java-image"
	envPrefix                                = "OTEL_"
)

// log is for logging in this package.
var instrumentationlog = logf.Log.WithName("instrumentation-resource")

func (r *Instrumentation) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-cloudwatch-aws-amazon-com-v1alpha1-instrumentation,mutating=true,failurePolicy=fail,sideEffects=None,groups=cloudwatch.aws.amazon.com,resources=instrumentations,verbs=create;update,versions=v1alpha1,name=minstrumentation.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Instrumentation{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *Instrumentation) Default() {
	instrumentationlog.Info("default", "name", r.Name)
	if r.Labels == nil {
		r.Labels = map[string]string{}
	}
	if r.Labels["app.kubernetes.io/managed-by"] == "" {
		r.Labels["app.kubernetes.io/managed-by"] = "amazon-cloudwatch-agent-operator"
	}

	if r.Spec.Java.Image == "" {
		if val, ok := r.Annotations[AnnotationDefaultAutoInstrumentationJava]; ok {
			r.Spec.Java.Image = val
		}
	}
	if r.Spec.Java.Resources.Limits == nil {
		r.Spec.Java.Resources.Limits = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("64Mi"),
		}
	}
	if r.Spec.Java.Resources.Requests == nil {
		r.Spec.Java.Resources.Requests = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("50m"),
			corev1.ResourceMemory: resource.MustParse("64Mi"),
		}
	}
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-cloudwatch-aws-amazon-com-v1alpha1-instrumentation,mutating=false,failurePolicy=fail,groups=cloudwatch.aws.amazon.com,resources=instrumentations,versions=v1alpha1,name=vinstrumentationcreateupdate.kb.io,sideEffects=none,admissionReviewVersions=v1
// +kubebuilder:webhook:verbs=delete,path=/validate-cloudwatch-aws-amazon-com-v1alpha1-instrumentation,mutating=false,failurePolicy=ignore,groups=cloudwatch.aws.amazon.com,resources=instrumentations,versions=v1alpha1,name=vinstrumentationdelete.kb.io,sideEffects=none,admissionReviewVersions=v1

var _ webhook.Validator = &Instrumentation{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *Instrumentation) ValidateCreate() (admission.Warnings, error) {
	instrumentationlog.Info("validate create", "name", r.Name)
	return nil, r.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *Instrumentation) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	instrumentationlog.Info("validate update", "name", r.Name)
	return nil, r.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *Instrumentation) ValidateDelete() (admission.Warnings, error) {
	instrumentationlog.Info("validate delete", "name", r.Name)
	return nil, nil
}

func validateJaegerRemoteSamplerArgument(argument string) error {
	parts := strings.Split(argument, ",")

	for _, part := range parts {
		kv := strings.Split(part, "=")
		if len(kv) != 2 {
			return fmt.Errorf("invalid argument: %s, the argument should be in the form of key=value", part)
		}

		switch kv[0] {
		case "endpoint":
			if kv[1] == "" {
				return fmt.Errorf("endpoint cannot be empty")
			}
		case "pollingIntervalMs":
			if _, err := strconv.Atoi(kv[1]); err != nil {
				return fmt.Errorf("invalid pollingIntervalMs: %s", kv[1])
			}
		case "initialSamplingRate":
			rate, err := strconv.ParseFloat(kv[1], 64)
			if err != nil {
				return fmt.Errorf("invalid initialSamplingRate: %s", kv[1])
			}
			if rate < 0 || rate > 1 {
				return fmt.Errorf("initialSamplingRate should be in rage [0..1]: %s", kv[1])
			}
		}
	}
	return nil
}

func (r *Instrumentation) validate() error {
	switch r.Spec.Sampler.Type {
	case "": // not set, do nothing
	case TraceIDRatio, ParentBasedTraceIDRatio:
		if r.Spec.Sampler.Argument != "" {
			rate, err := strconv.ParseFloat(r.Spec.Sampler.Argument, 64)
			if err != nil {
				return fmt.Errorf("spec.sampler.argument is not a number: %s", r.Spec.Sampler.Argument)
			}
			if rate < 0 || rate > 1 {
				return fmt.Errorf("spec.sampler.argument should be in rage [0..1]: %s", r.Spec.Sampler.Argument)
			}
		}
	case JaegerRemote, ParentBasedJaegerRemote:
		// value is a comma separated list of endpoint, pollingIntervalMs, initialSamplingRate
		// Example: `endpoint=http://localhost:14250,pollingIntervalMs=5000,initialSamplingRate=0.25`
		if r.Spec.Sampler.Argument != "" {
			err := validateJaegerRemoteSamplerArgument(r.Spec.Sampler.Argument)

			if err != nil {
				return fmt.Errorf("spec.sampler.argument is not a valid argument for sampler %s: %w", r.Spec.Sampler.Type, err)
			}
		}
	case AlwaysOn, AlwaysOff, ParentBasedAlwaysOn, ParentBasedAlwaysOff, XRaySampler:
	default:
		return fmt.Errorf("spec.sampler.type is not valid: %s", r.Spec.Sampler.Type)
	}
	// validate env vars
	if err := r.validateEnv(r.Spec.Java.Env); err != nil {
		return err
	}

	return nil
}

func (r *Instrumentation) validateEnv(envs []corev1.EnvVar) error {
	for _, env := range envs {
		if !strings.HasPrefix(env.Name, envPrefix) {
			return fmt.Errorf("env name should start with \"OTEL_\": %s", env.Name)
		}
	}
	return nil
}

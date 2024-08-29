// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.

package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/spf13/pflag"
	colfeaturegate "go.opentelemetry.io/collector/featuregate"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	k8sapiflag "k8s.io/component-base/cli/flag"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	otelv1alpha1 "github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/controllers"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/version"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/webhook/namespacemutation"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/webhook/podmutation"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/webhook/workloadmutation"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/featuregate"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/auto"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/sidecar"
	// +kubebuilder:scaffold:imports
)

const (
	cloudwatchAgentImageRepository           = "public.ecr.aws/cloudwatch-agent/cloudwatch-agent"
	autoInstrumentationJavaImageRepository   = "public.ecr.aws/aws-observability/adot-autoinstrumentation-java"
	autoInstrumentationPythonImageRepository = "public.ecr.aws/aws-observability/adot-autoinstrumentation-python"
	autoInstrumentationDotNetImageRepository = "ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-dotnet"
	autoInstrumentationNodeJSImageRepository = "ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-nodejs" //TODO: Update with the latest release of ADOT SDK for NodeJS
	dcgmExporterImageRepository              = "nvcr.io/nvidia/k8s/dcgm-exporter"
	neuronMonitorImageRepository             = "public.ecr.aws/neuron"
)

var (
	scheme   = k8sruntime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

type tlsConfig struct {
	minVersion   string
	cipherSuites []string
}

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(otelv1alpha1.AddToScheme(scheme))
	utilruntime.Must(routev1.AddToScheme(scheme))
	utilruntime.Must(monitoringv1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// stringFlagOrEnv defines a string flag which can be set by an environment variable.
// Precedence: flag > env var > default value.
func stringFlagOrEnv(p *string, name string, envName string, defaultValue string, usage string) {
	envValue := os.Getenv(envName)
	if envValue != "" {
		defaultValue = envValue
	}
	pflag.StringVar(p, name, defaultValue, usage)
}

func setLangEnvVarsForResource(langStr string, resourceStr string, resource map[string]string) {
	if cpu, ok := resource["cpu"]; ok {
		os.Setenv("AUTO_INSTRUMENTATION_"+langStr+"_CPU_"+resourceStr, cpu)
	}
	if memory, ok := resource["memory"]; ok {
		os.Setenv("AUTO_INSTRUMENTATION_"+langStr+"_MEM_"+resourceStr, memory)
	}
}

func setLangEnvVars(langStr string, cfg map[string]map[string]string) {
	if limits, ok := cfg["limits"]; ok {
		setLangEnvVarsForResource(langStr, "LIMIT", limits)
	}
	if requests, ok := cfg["requests"]; ok {
		setLangEnvVarsForResource(langStr, "REQUEST", requests)
	}
}

func main() {
	// registers any flags that underlying libraries might use
	opts := zap.Options{}
	flagset := featuregate.Flags(colfeaturegate.GlobalRegistry())
	opts.BindFlags(flag.CommandLine)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.CommandLine.AddGoFlagSet(flagset)

	v := version.Get()

	// add flags related to this operator
	var (
		metricsAddr                  string
		probeAddr                    string
		pprofAddr                    string
		agentImage                   string
		autoInstrumentationJava      string
		autoInstrumentationPython    string
		autoInstrumentationDotNet    string
		autoInstrumentationNodeJS    string
		autoAnnotationConfigStr      string
		autoInstrumentationConfigStr string
		webhookPort                  int
		tlsOpt                       tlsConfig
		dcgmExporterImage            string
		neuronMonitorImage           string
	)

	pflag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	pflag.StringVar(&probeAddr, "health-probe-addr", ":8081", "The address the probe endpoint binds to.")
	pflag.StringVar(&pprofAddr, "pprof-addr", "", "The address to expose the pprof server. Default is empty string which disables the pprof server.")
	stringFlagOrEnv(&agentImage, "agent-image", "RELATED_IMAGE_COLLECTOR", fmt.Sprintf("%s:%s", cloudwatchAgentImageRepository, v.AmazonCloudWatchAgent), "The default CloudWatch Agent image. This image is used when no image is specified in the CustomResource.")
	stringFlagOrEnv(&autoInstrumentationJava, "auto-instrumentation-java-image", "RELATED_IMAGE_AUTO_INSTRUMENTATION_JAVA", fmt.Sprintf("%s:%s", autoInstrumentationJavaImageRepository, v.AutoInstrumentationJava), "The default OpenTelemetry Java instrumentation image. This image is used when no image is specified in the CustomResource.")
	stringFlagOrEnv(&autoInstrumentationPython, "auto-instrumentation-python-image", "RELATED_IMAGE_AUTO_INSTRUMENTATION_PYTHON", fmt.Sprintf("%s:%s", autoInstrumentationPythonImageRepository, v.AutoInstrumentationPython), "The default OpenTelemetry Python instrumentation image. This image is used when no image is specified in the CustomResource.")
	stringFlagOrEnv(&autoInstrumentationDotNet, "auto-instrumentation-dotnet-image", "RELATED_IMAGE_AUTO_INSTRUMENTATION_DOTNET", fmt.Sprintf("%s:%s", autoInstrumentationDotNetImageRepository, v.AutoInstrumentationDotNet), "The default OpenTelemetry Dotnet instrumentation image. This image is used when no image is specified in the CustomResource.")
	stringFlagOrEnv(&autoInstrumentationNodeJS, "auto-instrumentation-nodejs-image", "RELATED_IMAGE_AUTO_INSTRUMENTATION_NODEJS", fmt.Sprintf("%s:%s", autoInstrumentationNodeJSImageRepository, v.AutoInstrumentationNodeJS), "The default OpenTelemetry NodeJS instrumentation image. This image is used when no image is specified in the CustomResource.")
	stringFlagOrEnv(&autoAnnotationConfigStr, "auto-annotation-config", "AUTO_ANNOTATION_CONFIG", "", "The configuration for auto-annotation.")
	pflag.StringVar(&autoInstrumentationConfigStr, "auto-instrumentation-config", "", "The configuration for auto-instrumentation.")
	stringFlagOrEnv(&dcgmExporterImage, "dcgm-exporter-image", "RELATED_IMAGE_DCGM_EXPORTER", fmt.Sprintf("%s:%s", dcgmExporterImageRepository, v.DcgmExporter), "The default DCGM Exporter image. This image is used when no image is specified in the CustomResource.")
	stringFlagOrEnv(&neuronMonitorImage, "neuron-monitor-image", "RELATED_IMAGE_NEURON_MONITOR", fmt.Sprintf("%s:%s", neuronMonitorImageRepository, v.NeuronMonitor), "The default Neuron monitor image. This image is used when no image is specified in the CustomResource.")
	pflag.Parse()

	// set instrumentation cpu and memory limits in environment variables to be used for default instrumentation; default values received from https://github.com/open-telemetry/opentelemetry-operator/blob/main/apis/v1alpha1/instrumentation_webhook.go
	autoInstrumentationConfig := map[string]map[string]map[string]string{"java": {"limits": {"cpu": "500m", "memory": "64Mi"}, "requests": {"cpu": "50m", "memory": "64Mi"}}, "python": {"limits": {"cpu": "500m", "memory": "32Mi"}, "requests": {"cpu": "50m", "memory": "32Mi"}}, "dotnet": {"limits": {"cpu": "500m", "memory": "128Mi"}, "requests": {"cpu": "50m", "memory": "128Mi"}}}
	err := json.Unmarshal([]byte(autoInstrumentationConfigStr), &autoInstrumentationConfig)
	if err != nil {
		setupLog.Info(fmt.Sprintf("Using default values: %v", autoInstrumentationConfig))
	}
	if javaVar, ok := autoInstrumentationConfig["java"]; ok {
		setLangEnvVars("JAVA", javaVar)
	}
	if pythonVar, ok := autoInstrumentationConfig["python"]; ok {
		setLangEnvVars("PYTHON", pythonVar)
	}
	if dotNetVar, ok := autoInstrumentationConfig["dotnet"]; ok {
		setLangEnvVars("DOTNET", dotNetVar)
	}
	if dotNetVar, ok := autoInstrumentationConfig["nodejs"]; ok {
		setLangEnvVars("NODEJS", dotNetVar)
	}

	// set supported language instrumentation images in environment variable to be used for default instrumentation
	os.Setenv("AUTO_INSTRUMENTATION_JAVA", autoInstrumentationJava)
	os.Setenv("AUTO_INSTRUMENTATION_PYTHON", autoInstrumentationPython)
	os.Setenv("AUTO_INSTRUMENTATION_DOTNET", autoInstrumentationDotNet)
	os.Setenv("AUTO_INSTRUMENTATION_NODEJS", autoInstrumentationNodeJS)

	logger := zap.New(zap.UseFlagOptions(&opts))
	ctrl.SetLogger(logger)

	logger.Info("Starting the Amazon CloudWatch Agent Operator",
		"amazon-cloudwatch-agent-operator", v.Operator,
		"cloudwatch-agent", agentImage,
		"auto-instrumentation-java", autoInstrumentationJava,
		"auto-instrumentation-python", autoInstrumentationPython,
		"auto-instrumentation-dotnet", autoInstrumentationDotNet,
		"auto-instrumentation-nodejs", autoInstrumentationNodeJS,
		"dcgm-exporter", dcgmExporterImage,
		"neuron-monitor", neuronMonitorImage,
		"build-date", v.BuildDate,
		"go-version", v.Go,
		"go-arch", runtime.GOARCH,
		"go-os", runtime.GOOS,
	)

	cfg := config.New(
		config.WithLogger(ctrl.Log.WithName("config")),
		config.WithVersion(v),
		config.WithCollectorImage(agentImage),
		config.WithAutoInstrumentationJavaImage(autoInstrumentationJava),
		config.WithAutoInstrumentationPythonImage(autoInstrumentationPython),
		config.WithAutoInstrumentationDotNetImage(autoInstrumentationDotNet),
		config.WithAutoInstrumentationNodeJSImage(autoInstrumentationNodeJS),
		config.WithDcgmExporterImage(dcgmExporterImage),
		config.WithNeuronMonitorImage(neuronMonitorImage),
	)

	watchNamespace, found := os.LookupEnv("WATCH_NAMESPACE")
	if found {
		setupLog.Info("watching namespace(s)", "namespaces", watchNamespace)
	} else {
		setupLog.Info("the env var WATCH_NAMESPACE isn't set, watching all namespaces")
	}

	optionsTlSOptsFuncs := []func(*tls.Config){
		func(config *tls.Config) { tlsConfigSetting(config, tlsOpt) },
	}
	var namespaces map[string]cache.Config
	if strings.Contains(watchNamespace, ",") {
		namespaces = map[string]cache.Config{}
		for _, ns := range strings.Split(watchNamespace, ",") {
			namespaces[ns] = cache.Config{}
		}
	}

	mgrOptions := ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		PprofBindAddress:       pprofAddr,
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    webhookPort,
			TLSOpts: optionsTlSOptsFuncs,
		}),
		Cache: cache.Options{
			DefaultNamespaces: namespaces,
		},
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), mgrOptions)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()

	if err = controllers.NewReconciler(controllers.Params{
		Client:   mgr.GetClient(),
		Log:      ctrl.Log.WithName("controllers").WithName("AmazonCloudWatchAgent"),
		Scheme:   mgr.GetScheme(),
		Config:   cfg,
		Recorder: mgr.GetEventRecorderFor("amazon-cloudwatch-agent-operator"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "AmazonCloudWatchAgent")
		os.Exit(1)
	}

	if err = controllers.NewDcgmExporterReconciler(controllers.Params{
		Client:   mgr.GetClient(),
		Log:      ctrl.Log.WithName("controllers").WithName("DcgmExporter"),
		Scheme:   mgr.GetScheme(),
		Config:   cfg,
		Recorder: mgr.GetEventRecorderFor("amazon-cloudwatch-agent-operator"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DcgmExporter")
		os.Exit(1)
	}

	if err = controllers.NewNeuronMonitorReconciler(controllers.Params{
		Client:   mgr.GetClient(),
		Log:      ctrl.Log.WithName("controllers").WithName("NeuronMonitor"),
		Scheme:   mgr.GetScheme(),
		Config:   cfg,
		Recorder: mgr.GetEventRecorderFor("amazon-cloudwatch-agent-operator"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NeuronMonitor")
		os.Exit(1)
	}

	decoder := admission.NewDecoder(mgr.GetScheme())

	if os.Getenv("DISABLE_AUTO_ANNOTATION") == "true" || autoAnnotationConfigStr == "" {
		setupLog.Info("Auto-annotation is disabled")
	} else {
		var autoAnnotationConfig auto.AnnotationConfig
		if err = json.Unmarshal([]byte(autoAnnotationConfigStr), &autoAnnotationConfig); err != nil {
			setupLog.Error(err, "Unable to unmarshal auto-annotation config")
		} else {
			autoAnnotationMutators := auto.NewAnnotationMutators(
				mgr.GetClient(),
				mgr.GetAPIReader(),
				logger,
				autoAnnotationConfig,
				instrumentation.NewTypeSet(
					instrumentation.TypeJava,
					instrumentation.TypePython,
					instrumentation.TypeDotNet,
					instrumentation.TypeNodeJS,
				),
			)
			mgr.GetWebhookServer().Register("/mutate-v1-workload", &webhook.Admission{
				Handler: workloadmutation.NewWebhookHandler(decoder, autoAnnotationMutators)})
			mgr.GetWebhookServer().Register("/mutate-v1-namespace", &webhook.Admission{
				Handler: namespacemutation.NewWebhookHandler(decoder, autoAnnotationMutators),
			})
			setupLog.Info("Auto-annotation is enabled")
			go waitForWebhookServerStart(
				ctx,
				mgr.GetWebhookServer().StartedChecker(),
				func(ctx context.Context) {
					setupLog.Info("Applying auto-annotation")
					autoAnnotationMutators.MutateAndPatchAll(ctx)
				},
			)
		}
	}

	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err = otelv1alpha1.SetupCollectorWebhook(mgr, cfg); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "AmazonCloudWatchAgent")
			os.Exit(1)
		}
		if err = otelv1alpha1.SetupInstrumentationWebhook(mgr, cfg); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Instrumentation")
			os.Exit(1)
		}
		mgr.GetWebhookServer().Register("/mutate-v1-pod", &webhook.Admission{
			Handler: podmutation.NewWebhookHandler(cfg, ctrl.Log.WithName("pod-webhook"), decoder, mgr.GetClient(),
				[]podmutation.PodMutator{
					sidecar.NewMutator(logger, cfg, mgr.GetClient()),
					instrumentation.NewMutator(logger, mgr.GetClient(), mgr.GetEventRecorderFor("amazon-cloudwatch-agent-operator")),
				}),
		})
	} else {
		ctrl.Log.Info("Webhooks are disabled, operator is running an unsupported mode", "ENABLE_WEBHOOKS", "false")
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func waitForWebhookServerStart(ctx context.Context, checker healthz.Checker, callback func(context.Context)) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := checker(nil); err == nil {
				setupLog.Info("Webhook server has started")
				callback(ctx)
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// This function get the option from command argument (tlsConfig), check the validity through k8sapiflag
// and set the config for webhook server.
// refer to https://pkg.go.dev/k8s.io/component-base/cli/flag
func tlsConfigSetting(cfg *tls.Config, tlsOpt tlsConfig) {
	// TLSVersion helper function returns the TLS Version ID for the version name passed.
	tlsVersion, err := k8sapiflag.TLSVersion(tlsOpt.minVersion)
	if err != nil {
		setupLog.Error(err, "TLS version invalid")
	}
	cfg.MinVersion = tlsVersion

	// TLSCipherSuites helper function returns a list of cipher suite IDs from the cipher suite names passed.
	cipherSuiteIDs, err := k8sapiflag.TLSCipherSuites(tlsOpt.cipherSuites)
	if err != nil {
		setupLog.Error(err, "Failed to convert TLS cipher suite name to ID")
	}
	cfg.CipherSuites = cipherSuiteIDs
}

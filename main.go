// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.

package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/open-telemetry/opentelemetry-operator/pkg/featuregate"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/spf13/pflag"
	colfeaturegate "go.opentelemetry.io/collector/featuregate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	k8sapiflag "k8s.io/component-base/cli/flag"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	cwv1alphav1 "github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/controllers"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/version"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/webhookhandler"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/sidecar"
)

const (
	cloudwatchAgentImageRepository = "public.ecr.aws/cloudwatch-agent/cloudwatch-agent"
	cloudwatchAgentImageTag        = "latest"

	autoInstrumentationJavaImageRepository = "ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-java"
	autoInstrumentationJavaImageTag        = "latest"
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

	utilruntime.Must(cwv1alphav1.AddToScheme(scheme))
	utilruntime.Must(routev1.AddToScheme(scheme))
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
		agentImage              string
		autoInstrumentationJava string
		webhookPort             int
		tlsOpt                  tlsConfig
	)

	pflag.StringVar(&agentImage, "agent-image", fmt.Sprintf("%s:%s", cloudwatchAgentImageRepository, cloudwatchAgentImageTag), "The default cloudwatch agent image. This image is used when no image is specified in the CustomResource.")
	pflag.StringVar(&autoInstrumentationJava, "auto-instrumentation-java-image", fmt.Sprintf("%s:%s", autoInstrumentationJavaImageRepository, autoInstrumentationJavaImageTag), "The default OpenTelemetry Java instrumentation image. This image is used when no image is specified in the CustomResource.")
	pflag.Parse()

	logger := zap.New(zap.UseFlagOptions(&opts))
	ctrl.SetLogger(logger)

	logger.Info("Starting the Amazon CloudWatch Agent Operator",
		"amazon-cloudwatch-agent-operator", v.Operator,
		"amazon-cloudwatch-agent", agentImage,
		"auto-instrumentation-java", autoInstrumentationJava,
		"build-date", v.BuildDate,
		"go-version", v.Go,
		"go-arch", runtime.GOARCH,
		"go-os", runtime.GOOS,
	)

	cfg := config.New(
		config.WithLogger(ctrl.Log.WithName("config")),
		config.WithVersion(v),
		config.WithCollectorImage(agentImage),
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
	var namespaces []string
	if strings.Contains(watchNamespace, ",") {
		namespaces = strings.Split(watchNamespace, ",")
	} else {
		namespaces = []string{watchNamespace}
	}

	mgrOptions := ctrl.Options{
		Scheme: scheme,
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    webhookPort,
			TLSOpts: optionsTlSOptsFuncs,
		}),
		Cache: cache.Options{
			Namespaces: namespaces,
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

	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		// Create webhook to create cloudwatch agent operator resources
		if err = (&cwv1alphav1.AmazonCloudWatchAgent{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "AmazonCloudWatch")
			os.Exit(1)
		}

		// Create webhook to install instrumentation sdk to pods
		if err = (&cwv1alphav1.Instrumentation{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					cwv1alphav1.AnnotationDefaultAutoInstrumentationJava: autoInstrumentationJava,
				},
			},
		}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Instrumentation")
			os.Exit(1)
		}
		decoder := admission.NewDecoder(mgr.GetScheme())
		mgr.GetWebhookServer().Register("/mutate-v1-pod", &webhook.Admission{
			Handler: webhookhandler.NewWebhookHandler(cfg, ctrl.Log.WithName("pod-webhook"), decoder, mgr.GetClient(),
				[]webhookhandler.PodMutator{
					sidecar.NewMutator(logger, cfg, mgr.GetClient()),
					instrumentation.NewMutator(logger, mgr.GetClient(), mgr.GetEventRecorderFor("opentelemetry-operator")),
				}),
		})
	} else {
		ctrl.Log.Info("Webhooks are disabled, operator is running an unsupported mode", "ENABLE_WEBHOOKS", "false")
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
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

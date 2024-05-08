// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"flag"
	"path/filepath"

	"github.com/spf13/pflag"
	"k8s.io/client-go/util/homedir"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// Flag names.
const (
	targetAllocatorName         = "target-allocator"
	configFilePathFlagName      = "config-file"
	listenAddrFlagName          = "listen-addr"
	prometheusCREnabledFlagName = "enable-prometheus-cr-watcher"
	kubeConfigPathFlagName      = "kubeconfig-path"
	reloadConfigFlagName        = "reload-config"
	httpsEnabledFlagName         = "enable-https-server"
	listenAddrHttpsFlagName      = "listen-addr-https"
	httpsCAFilePathFlagName      = "https-ca-file"
	httpsTLSCertFilePathFlagName = "https-tls-cert-file"
	httpsTLSKeyFilePathFlagName  = "https-tls-key-file"
)

// We can't bind this flag to our FlagSet, so we need to handle it separately.
var zapCmdLineOpts zap.Options

func getFlagSet(errorHandling pflag.ErrorHandling) *pflag.FlagSet {
	flagSet := pflag.NewFlagSet(targetAllocatorName, errorHandling)
	flagSet.String(configFilePathFlagName, DefaultConfigFilePath, "The path to the config file.")
	flagSet.String(listenAddrFlagName, ":8080", "The address where this service serves.")
	flagSet.Bool(prometheusCREnabledFlagName, false, "Enable Prometheus CRs as target sources")
	flagSet.String(kubeConfigPathFlagName, filepath.Join(homedir.HomeDir(), ".kube", "config"), "absolute path to the KubeconfigPath file")
	flagSet.Bool(reloadConfigFlagName, false, "Enable automatic configuration reloading. This functionality is deprecated and will be removed in a future release.")
	flagSet.Bool(httpsEnabledFlagName, false, "Enable HTTPS additional server")
	flagSet.String(listenAddrHttpsFlagName, ":8443", "The address where this service serves over HTTPS.")
	flagSet.String(httpsCAFilePathFlagName, "", "The path to the HTTPS server TLS CA file.")
	flagSet.String(httpsTLSCertFilePathFlagName, "", "The path to the HTTPS server TLS certificate file.")
	flagSet.String(httpsTLSKeyFilePathFlagName, "", "The path to the HTTPS server TLS key file.")
	zapFlagSet := flag.NewFlagSet("", flag.ErrorHandling(errorHandling))
	zapCmdLineOpts.BindFlags(zapFlagSet)
	flagSet.AddGoFlagSet(zapFlagSet)
	return flagSet
}

func getConfigFilePath(flagSet *pflag.FlagSet) (string, error) {
	return flagSet.GetString(configFilePathFlagName)
}

func getKubeConfigFilePath(flagSet *pflag.FlagSet) (string, error) {
	return flagSet.GetString(kubeConfigPathFlagName)
}

func getListenAddr(flagSet *pflag.FlagSet) (string, error) {
	return flagSet.GetString(listenAddrFlagName)
}

func getPrometheusCREnabled(flagSet *pflag.FlagSet) (bool, error) {
	return flagSet.GetBool(prometheusCREnabledFlagName)
}

func getConfigReloadEnabled(flagSet *pflag.FlagSet) (bool, error) {
	return flagSet.GetBool(reloadConfigFlagName)
}

func getHttpsListenAddr(flagSet *pflag.FlagSet) (string, error) {
	return flagSet.GetString(listenAddrHttpsFlagName)
}

func getHttpsEnabled(flagSet *pflag.FlagSet) (bool, error) {
	return flagSet.GetBool(httpsEnabledFlagName)
}

func getHttpsCAFilePath(flagSet *pflag.FlagSet) (string, error) {
	return flagSet.GetString(httpsCAFilePathFlagName)
}

func getHttpsTLSCertFilePath(flagSet *pflag.FlagSet) (string, error) {
	return flagSet.GetString(httpsTLSCertFilePathFlagName)
}

func getHttpsTLSKeyFilePath(flagSet *pflag.FlagSet) (string, error) {
	return flagSet.GetString(httpsTLSKeyFilePathFlagName)
}

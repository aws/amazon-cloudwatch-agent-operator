// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/common/model"
	promconfig "github.com/prometheus/prometheus/config"
	_ "github.com/prometheus/prometheus/discovery/install"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	DefaultResyncTime                        = 5 * time.Minute
	DefaultConfigFilePath     string         = "/conf/targetallocator.yaml"
	DefaultCRScrapeInterval   model.Duration = model.Duration(time.Second * 30)
	DefaultAllocationStrategy                = "consistent-hashing"
	DefaultFilterStrategy                    = "relabel-config"
)

type Config struct {
	ListenAddr             string             `yaml:"listen_addr,omitempty"`
	KubeConfigFilePath     string             `yaml:"kube_config_file_path,omitempty"`
	ClusterConfig          *rest.Config       `yaml:"-"`
	RootLogger             logr.Logger        `yaml:"-"`
	ReloadConfig           bool               `yaml:"-"`
	LabelSelector          map[string]string  `yaml:"label_selector,omitempty"`
	PromConfig             *promconfig.Config `yaml:"config"`
	AllocationStrategy     *string            `yaml:"allocation_strategy,omitempty"`
	FilterStrategy         *string            `yaml:"filter_strategy,omitempty"`
	PrometheusCR           PrometheusCRConfig `yaml:"prometheus_cr,omitempty"`
	PodMonitorSelector     map[string]string  `yaml:"pod_monitor_selector,omitempty"`
	ServiceMonitorSelector map[string]string  `yaml:"service_monitor_selector,omitempty"`
	CollectorSelector  *metav1.LabelSelector `yaml:"collector_selector,omitempty"`
	HTTPS              HTTPSServerConfig     `yaml:"https,omitempty"`
}

type PrometheusCRConfig struct {
	Enabled        bool           `yaml:"enabled,omitempty"`
	ScrapeInterval model.Duration `yaml:"scrape_interval,omitempty"`
}

type HTTPSServerConfig struct {
	Enabled         bool   `yaml:"enabled,omitempty"`
	ListenAddr      string `yaml:"listen_addr,omitempty"`
	CAFilePath      string `yaml:"ca_file_path,omitempty"`
	TLSCertFilePath string `yaml:"tls_cert_file_path,omitempty"`
	TLSKeyFilePath  string `yaml:"tls_key_file_path,omitempty"`
}

func (c Config) GetAllocationStrategy() string {
	if c.AllocationStrategy != nil {
		return *c.AllocationStrategy
	}
	return DefaultAllocationStrategy
}

func (c Config) GetTargetsFilterStrategy() string {
	if c.FilterStrategy != nil {
		return *c.FilterStrategy
	}
	return ""
}

func LoadFromFile(file string, target *Config) error {
	return unmarshal(target, file)
}

func LoadFromCLI(target *Config, flagSet *pflag.FlagSet) error {
	var err error
	// set the rest of the config attributes based on command-line flag values
	target.RootLogger = zap.New(zap.UseFlagOptions(&zapCmdLineOpts))
	klog.SetLogger(target.RootLogger)
	ctrl.SetLogger(target.RootLogger)

	target.KubeConfigFilePath, err = getKubeConfigFilePath(flagSet)
	if err != nil {
		return err
	}
	clusterConfig, err := clientcmd.BuildConfigFromFlags("", target.KubeConfigFilePath)
	if err != nil {
		pathError := &fs.PathError{}
		if ok := errors.As(err, &pathError); !ok {
			return err
		}
		clusterConfig, err = rest.InClusterConfig()
		if err != nil {
			return err
		}
		target.KubeConfigFilePath = ""
	}
	target.ClusterConfig = clusterConfig

	target.ListenAddr, err = getListenAddr(flagSet)
	if err != nil {
		return err
	}

	target.PrometheusCR.Enabled, err = getPrometheusCREnabled(flagSet)
	if err != nil {
		return err
	}

	target.ReloadConfig, err = getConfigReloadEnabled(flagSet)
	if err != nil {
		return err
	}

	target.HTTPS.Enabled, err = getHttpsEnabled(flagSet)
	if err != nil {
		return err
	}

	target.HTTPS.ListenAddr, err = getHttpsListenAddr(flagSet)
	if err != nil {
		return err
	}

	target.HTTPS.CAFilePath, err = getHttpsCAFilePath(flagSet)
	if err != nil {
		return err
	}

	target.HTTPS.TLSCertFilePath, err = getHttpsTLSCertFilePath(flagSet)
	if err != nil {
		return err
	}

	target.HTTPS.TLSKeyFilePath, err = getHttpsTLSKeyFilePath(flagSet)
	if err != nil {
		return err
	}

	return nil
}

func unmarshal(cfg *Config, configFile string) error {

	yamlFile, err := os.ReadFile(configFile)
	if err != nil {
		return err
	}
	if err = yaml.UnmarshalStrict(yamlFile, cfg); err != nil {
		return fmt.Errorf("error unmarshaling YAML: %w", err)
	}
	return nil
}

func CreateDefaultConfig() Config {
	var allocation_strategy = DefaultAllocationStrategy
	return Config{
		PrometheusCR: PrometheusCRConfig{
			ScrapeInterval: DefaultCRScrapeInterval,
		},
		AllocationStrategy: &allocation_strategy,
	}
}

func Load() (*Config, string, error) {
	var err error

	flagSet := getFlagSet(pflag.ExitOnError)
	err = flagSet.Parse(os.Args)
	if err != nil {
		return nil, "", err
	}

	config := CreateDefaultConfig()

	// load the config from the config file
	configFilePath, err := getConfigFilePath(flagSet)
	if err != nil {
		return nil, "", err
	}
	err = LoadFromFile(configFilePath, &config)
	if err != nil {
		return nil, "", err
	}

	err = LoadFromCLI(&config, flagSet)
	if err != nil {
		return nil, "", err
	}

	return &config, configFilePath, nil
}

// ValidateConfig validates the cli and file configs together.
func ValidateConfig(config *Config) error {
	scrapeConfigsPresent := (config.PromConfig != nil && len(config.PromConfig.ScrapeConfigs) > 0)
	if !(config.PrometheusCR.Enabled || scrapeConfigsPresent) {
		return fmt.Errorf("at least one scrape config must be defined, or Prometheus CR watching must be enabled")
	}
	return nil
}

func (c HTTPSServerConfig) NewTLSConfig() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(c.TLSCertFilePath, c.TLSKeyFilePath)
	if err != nil {
		return nil, err
	}

	caCert, err := os.ReadFile(c.CAFilePath)
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caCertPool,
		MinVersion:   tls.VersionTLS12,
	}
	return tlsConfig, nil
}

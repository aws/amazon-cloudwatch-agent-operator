// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/amazon-cloudwatch-agent-operator/cmd/amazon-cloudwatch-agent-target-allocator/allocation"
	allocatorconfig "github.com/aws/amazon-cloudwatch-agent-operator/cmd/amazon-cloudwatch-agent-target-allocator/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/cmd/amazon-cloudwatch-agent-target-allocator/target"
)

var (
	logger       = logf.Log.WithName("server-unit-tests")
	baseLabelSet = model.LabelSet{
		"test_label": "test-value",
	}
	testJobLabelSetTwo = model.LabelSet{
		"test_label": "test-value2",
	}
	baseTargetItem       = target.NewItem("test-job", "test-url", baseLabelSet, "test-collector")
	secondTargetItem     = target.NewItem("test-job", "test-url", baseLabelSet, "test-collector")
	testJobTargetItemTwo = target.NewItem("test-job", "test-url2", testJobLabelSetTwo, "test-collector2")
)

func TestServer_LivenessProbeHandler(t *testing.T) {
	consistentHashing, _ := allocation.New("consistent-hashing", logger)
	listenAddr := ":8080"
	s := NewServer(logger, consistentHashing, listenAddr)
	request := httptest.NewRequest("GET", "/livez", nil)
	w := httptest.NewRecorder()

	s.server.Handler.ServeHTTP(w, request)
	result := w.Result()

	assert.Equal(t, http.StatusOK, result.StatusCode)
}

func TestServer_TargetsHandler(t *testing.T) {
	consistentHashing, _ := allocation.New("consistent-hashing", logger)
	type args struct {
		collector string
		job       string
		cMap      map[string]*target.Item
		allocator allocation.Allocator
	}
	type want struct {
		items     []*target.Item
		errString string
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "Empty target map",
			args: args{
				collector: "test-collector",
				job:       "test-job",
				cMap:      map[string]*target.Item{},
				allocator: consistentHashing,
			},
			want: want{
				items: []*target.Item{},
			},
		},
		{
			name: "Single entry target map",
			args: args{
				collector: "test-collector",
				job:       "test-job",
				cMap: map[string]*target.Item{
					baseTargetItem.Hash(): baseTargetItem,
				},
				allocator: consistentHashing,
			},
			want: want{
				items: []*target.Item{
					{
						TargetURL: []string{"test-url"},
						Labels: map[model.LabelName]model.LabelValue{
							"test_label": "test-value",
						},
					},
				},
			},
		},
		{
			name: "Multiple entry target map",
			args: args{
				collector: "test-collector",
				job:       "test-job",
				cMap: map[string]*target.Item{
					baseTargetItem.Hash():   baseTargetItem,
					secondTargetItem.Hash(): secondTargetItem,
				},
				allocator: consistentHashing,
			},
			want: want{
				items: []*target.Item{
					{
						TargetURL: []string{"test-url"},
						Labels: map[model.LabelName]model.LabelValue{
							"test_label": "test-value",
						},
					},
				},
			},
		},
		{
			name: "Multiple entry target map of same job with label merge",
			args: args{
				collector: "test-collector",
				job:       "test-job",
				cMap: map[string]*target.Item{
					baseTargetItem.Hash():       baseTargetItem,
					testJobTargetItemTwo.Hash(): testJobTargetItemTwo,
				},
				allocator: consistentHashing,
			},
			want: want{
				items: []*target.Item{
					{
						TargetURL: []string{"test-url"},
						Labels: map[model.LabelName]model.LabelValue{
							"test_label": "test-value",
						},
					},
					{
						TargetURL: []string{"test-url2"},
						Labels: map[model.LabelName]model.LabelValue{
							"test_label": "test-value2",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listenAddr := ":8080"
			s := NewServer(logger, tt.args.allocator, listenAddr)
			tt.args.allocator.SetCollectors(map[string]*allocation.Collector{"test-collector": {Name: "test-collector"}})
			tt.args.allocator.SetTargets(tt.args.cMap)
			request := httptest.NewRequest("GET", fmt.Sprintf("/jobs/%s/targets?collector_id=%s", tt.args.job, tt.args.collector), nil)
			w := httptest.NewRecorder()

			s.server.Handler.ServeHTTP(w, request)
			result := w.Result()

			assert.Equal(t, http.StatusOK, result.StatusCode)
			body := result.Body
			bodyBytes, err := io.ReadAll(body)
			assert.NoError(t, err)
			if len(tt.want.errString) != 0 {
				assert.EqualError(t, err, tt.want.errString)
				return
			}
			var itemResponse []*target.Item
			err = json.Unmarshal(bodyBytes, &itemResponse)
			assert.NoError(t, err)
			assert.ElementsMatch(t, tt.want.items, itemResponse)
		})
	}
}

func TestServer_ScrapeConfigsHandler(t *testing.T) {
	svrConfig := allocatorconfig.HTTPSServerConfig{}
	tlsConfig, _ := svrConfig.NewTLSConfig(context.TODO())
	tests := []struct {
		description   string
		scrapeConfigs map[string]*promconfig.ScrapeConfig
		expectedCode  int
		expectedBody  []byte
		serverOptions []Option
	}{
		{
			description:   "nil scrape config",
			scrapeConfigs: nil,
			expectedCode:  http.StatusOK,
			expectedBody:  []byte("{}"),
		},
		{
			description:   "empty scrape config",
			scrapeConfigs: map[string]*promconfig.ScrapeConfig{},
			expectedCode:  http.StatusOK,
			expectedBody:  []byte("{}"),
		},
		{
			description: "single entry",
			scrapeConfigs: map[string]*promconfig.ScrapeConfig{
				"serviceMonitor/testapp/testapp/0": {
					JobName:         "serviceMonitor/testapp/testapp/0",
					HonorTimestamps: true,
					ScrapeInterval:  model.Duration(30 * time.Second),
					ScrapeTimeout:   model.Duration(30 * time.Second),
					MetricsPath:     "/metrics",
					Scheme:          "http",
					HTTPClientConfig: config.HTTPClientConfig{
						FollowRedirects: true,
					},
					RelabelConfigs: []*relabel.Config{
						{
							SourceLabels: model.LabelNames{model.LabelName("job")},
							Separator:    ";",
							Regex:        relabel.MustNewRegexp("(.*)"),
							TargetLabel:  "__tmp_prometheus_job_name",
							Replacement:  "$$1",
							Action:       relabel.Replace,
						},
					},
				},
			},
			expectedCode: http.StatusOK,
		},
		{
			description: "multiple entries",
			scrapeConfigs: map[string]*promconfig.ScrapeConfig{
				"serviceMonitor/testapp/testapp/0": {
					JobName:         "serviceMonitor/testapp/testapp/0",
					HonorTimestamps: true,
					ScrapeInterval:  model.Duration(30 * time.Second),
					ScrapeTimeout:   model.Duration(30 * time.Second),
					MetricsPath:     "/metrics",
					Scheme:          "http",
					HTTPClientConfig: config.HTTPClientConfig{
						FollowRedirects: true,
					},
					RelabelConfigs: []*relabel.Config{
						{
							SourceLabels: model.LabelNames{model.LabelName("job")},
							Separator:    ";",
							Regex:        relabel.MustNewRegexp("(.*)"),
							TargetLabel:  "__tmp_prometheus_job_name",
							Replacement:  "$$1",
							Action:       relabel.Replace,
						},
						{
							SourceLabels: model.LabelNames{
								model.LabelName("__meta_kubernetes_service_label_app_kubernetes_io_name"),
								model.LabelName("__meta_kubernetes_service_labelpresent_app_kubernetes_io_name"),
							},
							Separator:   ";",
							Regex:       relabel.MustNewRegexp("(testapp);true"),
							Replacement: "$$1",
							Action:      relabel.Keep,
						},
						{
							SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_endpoint_port_name")},
							Separator:    ";",
							Regex:        relabel.MustNewRegexp("http"),
							Replacement:  "$$1",
							Action:       relabel.Keep,
						},
						{
							SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_namespace")},
							Separator:    ";",
							Regex:        relabel.MustNewRegexp("(.*)"),
							TargetLabel:  "namespace",
							Replacement:  "$$1",
							Action:       relabel.Replace,
						},
						{
							SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_service_name")},
							Separator:    ";",
							Regex:        relabel.MustNewRegexp("(.*)"),
							TargetLabel:  "service",
							Replacement:  "$$1",
							Action:       relabel.Replace,
						},
						{
							SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_pod_name")},
							Separator:    ";",
							Regex:        relabel.MustNewRegexp("(.*)"),
							TargetLabel:  "pod",
							Replacement:  "$$1",
							Action:       relabel.Replace,
						},
						{
							SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_pod_container_name")},
							Separator:    ";",
							Regex:        relabel.MustNewRegexp("(.*)"),
							TargetLabel:  "container",
							Replacement:  "$$1",
							Action:       relabel.Replace,
						},
					},
				},
				"serviceMonitor/testapp/testapp1/0": {
					JobName:         "serviceMonitor/testapp/testapp1/0",
					HonorTimestamps: true,
					ScrapeInterval:  model.Duration(5 * time.Minute),
					ScrapeTimeout:   model.Duration(10 * time.Second),
					MetricsPath:     "/v2/metrics",
					Scheme:          "http",
					HTTPClientConfig: config.HTTPClientConfig{
						FollowRedirects: true,
					},
					RelabelConfigs: []*relabel.Config{
						{
							SourceLabels: model.LabelNames{model.LabelName("job")},
							Separator:    ";",
							Regex:        relabel.MustNewRegexp("(.*)"),
							TargetLabel:  "__tmp_prometheus_job_name",
							Replacement:  "$$1",
							Action:       relabel.Replace,
						},
						{
							SourceLabels: model.LabelNames{
								model.LabelName("__meta_kubernetes_service_label_app_kubernetes_io_name"),
								model.LabelName("__meta_kubernetes_service_labelpresent_app_kubernetes_io_name"),
							},
							Separator:   ";",
							Regex:       relabel.MustNewRegexp("(testapp);true"),
							Replacement: "$$1",
							Action:      relabel.Keep,
						},
						{
							SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_endpoint_port_name")},
							Separator:    ";",
							Regex:        relabel.MustNewRegexp("http"),
							Replacement:  "$$1",
							Action:       relabel.Keep,
						},
						{
							SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_namespace")},
							Separator:    ";",
							Regex:        relabel.MustNewRegexp("(.*)"),
							TargetLabel:  "namespace",
							Replacement:  "$$1",
							Action:       relabel.Replace,
						},
						{
							SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_service_name")},
							Separator:    ";",
							Regex:        relabel.MustNewRegexp("(.*)"),
							TargetLabel:  "service",
							Replacement:  "$$1",
							Action:       relabel.Replace,
						},
						{
							SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_pod_name")},
							Separator:    ";",
							Regex:        relabel.MustNewRegexp("(.*)"),
							TargetLabel:  "pod",
							Replacement:  "$$1",
							Action:       relabel.Replace,
						},
						{
							SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_pod_container_name")},
							Separator:    ";",
							Regex:        relabel.MustNewRegexp("(.*)"),
							TargetLabel:  "container",
							Replacement:  "$$1",
							Action:       relabel.Replace,
						},
					},
				},
				"serviceMonitor/testapp/testapp2/0": {
					JobName:         "serviceMonitor/testapp/testapp2/0",
					HonorTimestamps: true,
					ScrapeInterval:  model.Duration(30 * time.Minute),
					ScrapeTimeout:   model.Duration(2 * time.Minute),
					MetricsPath:     "/metrics",
					Scheme:          "http",
					HTTPClientConfig: config.HTTPClientConfig{
						FollowRedirects: true,
					},
					RelabelConfigs: []*relabel.Config{
						{
							SourceLabels: model.LabelNames{model.LabelName("job")},
							Separator:    ";",
							Regex:        relabel.MustNewRegexp("(.*)"),
							TargetLabel:  "__tmp_prometheus_job_name",
							Replacement:  "$$1",
							Action:       relabel.Replace,
						},
						{
							SourceLabels: model.LabelNames{
								model.LabelName("__meta_kubernetes_service_label_app_kubernetes_io_name"),
								model.LabelName("__meta_kubernetes_service_labelpresent_app_kubernetes_io_name"),
							},
							Separator:   ";",
							Regex:       relabel.MustNewRegexp("(testapp);true"),
							Replacement: "$$1",
							Action:      relabel.Keep,
						},
						{
							SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_endpoint_port_name")},
							Separator:    ";",
							Regex:        relabel.MustNewRegexp("http"),
							Replacement:  "$$1",
							Action:       relabel.Keep,
						},
						{
							SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_namespace")},
							Separator:    ";",
							Regex:        relabel.MustNewRegexp("(.*)"),
							TargetLabel:  "namespace",
							Replacement:  "$$1",
							Action:       relabel.Replace,
						},
						{
							SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_service_name")},
							Separator:    ";",
							Regex:        relabel.MustNewRegexp("(.*)"),
							TargetLabel:  "service",
							Replacement:  "$$1",
							Action:       relabel.Replace,
						},
						{
							SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_pod_name")},
							Separator:    ";",
							Regex:        relabel.MustNewRegexp("(.*)"),
							TargetLabel:  "pod",
							Replacement:  "$$1",
							Action:       relabel.Replace,
						},
						{
							SourceLabels: model.LabelNames{model.LabelName("__meta_kubernetes_pod_container_name")},
							Separator:    ";",
							Regex:        relabel.MustNewRegexp("(.*)"),
							TargetLabel:  "container",
							Replacement:  "$$1",
							Action:       relabel.Replace,
						},
					},
				},
			},
			expectedCode: http.StatusOK,
		},
		{
			description: "https secret handling",
			scrapeConfigs: map[string]*promconfig.ScrapeConfig{
				"serviceMonitor/testapp/testapp3/0": {
					JobName:         "serviceMonitor/testapp/testapp3/0",
					HonorTimestamps: true,
					ScrapeInterval:  model.Duration(30 * time.Second),
					ScrapeTimeout:   model.Duration(30 * time.Second),
					MetricsPath:     "/metrics",
					Scheme:          "http",
					HTTPClientConfig: config.HTTPClientConfig{
						FollowRedirects: true,
						BasicAuth: &config.BasicAuth{
							Username: "test",
							Password: "P@$$w0rd1!?",
						},
					},
				},
			},
			expectedCode: http.StatusOK,
			serverOptions: []Option{
				WithTLSConfig(tlsConfig, ""),
			},
		},
		{
			description: "http secret handling",
			scrapeConfigs: map[string]*promconfig.ScrapeConfig{
				"serviceMonitor/testapp/testapp3/0": {
					JobName:         "serviceMonitor/testapp/testapp3/0",
					HonorTimestamps: true,
					ScrapeInterval:  model.Duration(30 * time.Second),
					ScrapeTimeout:   model.Duration(30 * time.Second),
					MetricsPath:     "/metrics",
					Scheme:          "http",
					HTTPClientConfig: config.HTTPClientConfig{
						FollowRedirects: true,
						BasicAuth: &config.BasicAuth{
							Username: "test",
							Password: "P@$$w0rd1!?",
						},
					},
				},
			},
			expectedCode: http.StatusOK,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			listenAddr := ":8080"
			s := NewServer(logger, nil, listenAddr, tc.serverOptions...)
			assert.NoError(t, s.UpdateScrapeConfigResponse(tc.scrapeConfigs))

			request := httptest.NewRequest("GET", "/scrape_configs", nil)
			w := httptest.NewRecorder()

			if s.httpsServer != nil {
				request.TLS = &tls.ConnectionState{}
				s.httpsServer.Handler.ServeHTTP(w, request)
			} else {
				s.server.Handler.ServeHTTP(w, request)
			}
			result := w.Result()

			assert.Equal(t, tc.expectedCode, result.StatusCode)
			bodyBytes, err := io.ReadAll(result.Body)
			require.NoError(t, err)
			if tc.expectedBody != nil {
				assert.Equal(t, tc.expectedBody, bodyBytes)
				return
			}
			scrapeConfigs := map[string]*promconfig.ScrapeConfig{}
			err = yaml.Unmarshal(bodyBytes, scrapeConfigs)
			require.NoError(t, err)

			for _, c := range scrapeConfigs {
				if s.httpsServer == nil && c.HTTPClientConfig.BasicAuth != nil {
					assert.Equal(t, c.HTTPClientConfig.BasicAuth.Password, config.Secret("<secret>"))
				}
			}

			for _, c := range tc.scrapeConfigs {
				if s.httpsServer == nil && c.HTTPClientConfig.BasicAuth != nil {
					c.HTTPClientConfig.BasicAuth.Password = "<secret>"
				}
			}

			assert.Equal(t, tc.scrapeConfigs, scrapeConfigs)
		})
	}
}

func TestServer_JobHandler(t *testing.T) {
	tests := []struct {
		description  string
		targetItems  map[string]*target.Item
		expectedCode int
		expectedJobs map[string]target.LinkJSON
	}{
		{
			description:  "nil jobs",
			targetItems:  nil,
			expectedCode: http.StatusOK,
			expectedJobs: make(map[string]target.LinkJSON),
		},
		{
			description:  "empty jobs",
			targetItems:  map[string]*target.Item{},
			expectedCode: http.StatusOK,
			expectedJobs: make(map[string]target.LinkJSON),
		},
		{
			description: "one job",
			targetItems: map[string]*target.Item{
				"targetitem": target.NewItem("job1", "", model.LabelSet{}, ""),
			},
			expectedCode: http.StatusOK,
			expectedJobs: map[string]target.LinkJSON{
				"job1": newLink("job1"),
			},
		},
		{
			description: "multiple jobs",
			targetItems: map[string]*target.Item{
				"a": target.NewItem("job1", "", model.LabelSet{}, ""),
				"b": target.NewItem("job2", "", model.LabelSet{}, ""),
				"c": target.NewItem("job3", "", model.LabelSet{}, ""),
				"d": target.NewItem("job3", "", model.LabelSet{}, ""),
				"e": target.NewItem("job3", "", model.LabelSet{}, "")},
			expectedCode: http.StatusOK,
			expectedJobs: map[string]target.LinkJSON{
				"job1": newLink("job1"),
				"job2": newLink("job2"),
				"job3": newLink("job3"),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			listenAddr := ":8080"
			a := &mockAllocator{targetItems: tc.targetItems}
			s := NewServer(logger, a, listenAddr)
			request := httptest.NewRequest("GET", "/jobs", nil)
			w := httptest.NewRecorder()

			s.server.Handler.ServeHTTP(w, request)
			result := w.Result()

			assert.Equal(t, tc.expectedCode, result.StatusCode)
			bodyBytes, err := io.ReadAll(result.Body)
			require.NoError(t, err)
			jobs := map[string]target.LinkJSON{}
			err = json.Unmarshal(bodyBytes, &jobs)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedJobs, jobs)
		})
	}
}

func TestServer_Readiness(t *testing.T) {
	tests := []struct {
		description   string
		scrapeConfigs map[string]*promconfig.ScrapeConfig
		expectedCode  int
		expectedBody  []byte
	}{
		{
			description:   "nil scrape config",
			scrapeConfigs: nil,
			expectedCode:  http.StatusServiceUnavailable,
		},
		{
			description:   "empty scrape config",
			scrapeConfigs: map[string]*promconfig.ScrapeConfig{},
			expectedCode:  http.StatusOK,
		},
		{
			description: "single entry",
			scrapeConfigs: map[string]*promconfig.ScrapeConfig{
				"serviceMonitor/testapp/testapp/0": {
					JobName:         "serviceMonitor/testapp/testapp/0",
					HonorTimestamps: true,
					ScrapeInterval:  model.Duration(30 * time.Second),
					ScrapeTimeout:   model.Duration(30 * time.Second),
					MetricsPath:     "/metrics",
					Scheme:          "http",
					HTTPClientConfig: config.HTTPClientConfig{
						FollowRedirects: true,
					},
					RelabelConfigs: []*relabel.Config{
						{
							SourceLabels: model.LabelNames{model.LabelName("job")},
							Separator:    ";",
							Regex:        relabel.MustNewRegexp("(.*)"),
							TargetLabel:  "__tmp_prometheus_job_name",
							Replacement:  "$$1",
							Action:       relabel.Replace,
						},
					},
				},
			},
			expectedCode: http.StatusOK,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			listenAddr := ":8080"
			s := NewServer(logger, nil, listenAddr)
			if tc.scrapeConfigs != nil {
				assert.NoError(t, s.UpdateScrapeConfigResponse(tc.scrapeConfigs))
			}

			request := httptest.NewRequest("GET", "/readyz", nil)
			w := httptest.NewRecorder()

			s.server.Handler.ServeHTTP(w, request)
			result := w.Result()

			assert.Equal(t, tc.expectedCode, result.StatusCode)
		})
	}
}

func TestServer_ValidCAonTLS(t *testing.T) {
	listenAddr := ":8443"
	server, clientTlsConfig, err := createTestTLSServer(listenAddr)
	assert.NoError(t, err)
	go func() {
		assert.ErrorIs(t, server.StartHTTPS(), http.ErrServerClosed)
	}()
	time.Sleep(100 * time.Millisecond) // wait for server to launch
	defer func() {
		err := server.ShutdownHTTPS(context.Background())
		if err != nil {
			assert.NoError(t, err)
		}
	}()
	tests := []struct {
		description  string
		endpoint     string
		expectedCode int
	}{
		{
			description:  "with tls test for scrape config",
			endpoint:     "scrape_configs",
			expectedCode: http.StatusOK,
		},
		{
			description:  "with tls test for jobs",
			endpoint:     "jobs",
			expectedCode: http.StatusOK,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			// Create a custom HTTP client with TLS transport
			client := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: clientTlsConfig,
				},
			}

			// Make the GET request
			request, err := client.Get(fmt.Sprintf("https://localhost%s/%s", listenAddr, tc.endpoint))

			// Verify if a certificate verification error occurred
			require.NoError(t, err)

			// Only check the status code if there was no error
			if err == nil {
				assert.Equal(t, tc.expectedCode, request.StatusCode)
			} else {
				t.Log(err)
			}
		})
	}
}

func TestServer_MissingCAonTLS(t *testing.T) {
	listenAddr := ":8443"
	server, _, err := createTestTLSServer(listenAddr)
	assert.NoError(t, err)
	go func() {
		assert.ErrorIs(t, server.StartHTTPS(), http.ErrServerClosed)
	}()
	time.Sleep(100 * time.Millisecond) // wait for server to launch
	defer func() {
		err := server.ShutdownHTTPS(context.Background())
		if err != nil {
			assert.NoError(t, err)
		}
	}()
	tests := []struct {
		description  string
		endpoint     string
		expectedCode int
	}{
		{
			description:  "no tls test for scrape config",
			endpoint:     "scrape_configs",
			expectedCode: http.StatusBadRequest,
		},
		{
			description:  "no tls test for jobs",
			endpoint:     "jobs",
			expectedCode: http.StatusBadRequest,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			request, err := http.Get(fmt.Sprintf("https://localhost%s/%s", listenAddr, tc.endpoint))

			// Verify if a certificate verification error occurred
			require.Error(t, err)

			// Only check the status code if there was no error
			if err == nil {
				assert.Equal(t, tc.expectedCode, request.StatusCode)
			}
		})
	}
}

func TestServer_HTTPOnTLS(t *testing.T) {
	listenAddr := ":8443"
	server, _, err := createTestTLSServer(listenAddr)
	assert.NoError(t, err)
	go func() {
		assert.ErrorIs(t, server.StartHTTPS(), http.ErrServerClosed)
	}()
	time.Sleep(100 * time.Millisecond) // wait for server to launch

	defer func(s *Server, ctx context.Context) {
		err := s.Shutdown(ctx)
		if err != nil {
			assert.NoError(t, err)
		}
	}(server, context.Background())
	tests := []struct {
		description  string
		endpoint     string
		expectedCode int
	}{
		{
			description:  "no tls test for scrape config",
			endpoint:     "scrape_configs",
			expectedCode: http.StatusBadRequest,
		},
		{
			description:  "no tls test for jobs",
			endpoint:     "jobs",
			expectedCode: http.StatusBadRequest,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			request, err := http.Get(fmt.Sprintf("http://localhost%s/%s", listenAddr, tc.endpoint))

			// Only check the status code if there was no error
			if err == nil {
				assert.Equal(t, tc.expectedCode, request.StatusCode)
			}
		})
	}
}

func createTestTLSServer(listenAddr string) (*Server, *tls.Config, error) {
	//testing using this function replicates customer environment
	svrConfig := allocatorconfig.HTTPSServerConfig{}
	caBundle, caCert, caKey, clientCert, clientKey, err := generateTestingCerts()
	if err != nil {
		return nil, nil, err
	}
	svrConfig.TLSKeyFilePath = caKey
	svrConfig.TLSCertFilePath = caCert
	svrConfig.CAFilePath = caBundle
	tlsConfig, err := svrConfig.NewTLSConfig(context.TODO())
	if err != nil {
		return nil, nil, err
	}
	//generate ca bundle
	bundle, err := readClient(caBundle, clientCert, clientKey)
	if err != nil {
		return nil, nil, err
	}
	httpOptions := []Option{}
	httpOptions = append(httpOptions, WithTLSConfig(tlsConfig, listenAddr))

	allocator := &mockAllocator{targetItems: map[string]*target.Item{
		"a": target.NewItem("job1", "", model.LabelSet{}, ""),
	}}

	return NewServer(logger, allocator, listenAddr, httpOptions...), bundle, nil
}

func newLink(jobName string) target.LinkJSON {
	return target.LinkJSON{Link: fmt.Sprintf("/jobs/%s/targets", url.QueryEscape(jobName))}
}

func readClient(caBundlePath, clientCertPath, clientKeyPath string) (*tls.Config, error) {
	// Load the CA bundle
	caCert, err := os.ReadFile(caBundlePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA bundle: %w", err)
	}

	// Create a CA pool and add the CA certificate(s)
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to add CA certificates to pool")
	}

	clientCert, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate: %w", err)
	}

	// Set up TLS configuration with the CA pool
	tlsConfig := &tls.Config{
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{clientCert},
	}
	return tlsConfig, nil
}

func generateTestingCerts() (caBundlePath, caCertPath, caKeyPath, clientCertPath, clientKeyPath string, err error) {
	// Generate CA private key
	caPrivateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("error generating CA private key: %w", err)
	}

	// Create CA certificate template
	caTemplate := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	// Self-sign the CA certificate
	caCertBytes, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caPrivateKey.PublicKey, caPrivateKey)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("error creating CA certificate: %w", err)
	}

	// Marshal the CA private key
	caKeyBytes, err := x509.MarshalECPrivateKey(caPrivateKey)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("error marshaling CA private key: %w", err)
	}

	// Generate server private key
	serverPrivateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("error generating server private key: %w", err)
	}

	// Create server certificate template
	serverTemplate := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
	}

	// Sign the server certificate with the CA
	serverCertBytes, err := x509.CreateCertificate(rand.Reader, &serverTemplate, &caTemplate, &serverPrivateKey.PublicKey, caPrivateKey)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("error creating server certificate: %w", err)
	}

	// Marshal the server private key
	serverKeyBytes, err := x509.MarshalECPrivateKey(serverPrivateKey)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("error marshaling server private key: %w", err)
	}

	// Generate client private key
	clientPrivateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("error generating client private key: %w", err)
	}

	// Create client certificate template
	clientTemplate := x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "Test Client"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	// Sign the client certificate with the CA
	clientCertBytes, err := x509.CreateCertificate(rand.Reader, &clientTemplate, &caTemplate, &clientPrivateKey.PublicKey, caPrivateKey)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("error creating client certificate: %w", err)
	}

	// Marshal the client private key
	clientKeyBytes, err := x509.MarshalECPrivateKey(clientPrivateKey)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("error marshaling client private key: %w", err)
	}

	// Create temporary files for CA, server, and client certificates and keys
	tempDir := os.TempDir()

	caCertFile, err := os.CreateTemp(tempDir, "ca-cert-*.crt")
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("error creating temp CA cert file: %w", err)
	}
	defer caCertFile.Close()

	caKeyFile, err := os.CreateTemp(tempDir, "ca-key-*.key")
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("error creating temp CA key file: %w", err)
	}
	defer caKeyFile.Close()

	serverCertFile, err := os.CreateTemp(tempDir, "server-cert-*.crt")
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("error creating temp server cert file: %w", err)
	}
	defer serverCertFile.Close()

	serverKeyFile, err := os.CreateTemp(tempDir, "server-key-*.key")
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("error creating temp server key file: %w", err)
	}
	defer serverKeyFile.Close()

	clientCertFile, err := os.CreateTemp(tempDir, "client-cert-*.crt")
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("error creating temp client cert file: %w", err)
	}
	defer clientCertFile.Close()

	clientKeyFile, err := os.CreateTemp(tempDir, "client-key-*.key")
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("error creating temp client key file: %w", err)
	}
	defer clientKeyFile.Close()

	// Write the CA, server, and client certificates and keys to their respective files
	caCertPEMBlock := &pem.Block{Type: "CERTIFICATE", Bytes: caCertBytes}
	if err := pem.Encode(caCertFile, caCertPEMBlock); err != nil {
		return "", "", "", "", "", fmt.Errorf("error writing CA certificate: %w", err)
	}
	caKeyPEMBlock := &pem.Block{Type: "EC PRIVATE KEY", Bytes: caKeyBytes}
	if err := pem.Encode(caKeyFile, caKeyPEMBlock); err != nil {
		return "", "", "", "", "", fmt.Errorf("error writing CA key: %w", err)
	}
	serverCertPEMBlock := &pem.Block{Type: "CERTIFICATE", Bytes: serverCertBytes}
	if err := pem.Encode(serverCertFile, serverCertPEMBlock); err != nil {
		return "", "", "", "", "", fmt.Errorf("error writing server certificate: %w", err)
	}
	serverKeyPEMBlock := &pem.Block{Type: "EC PRIVATE KEY", Bytes: serverKeyBytes}
	if err := pem.Encode(serverKeyFile, serverKeyPEMBlock); err != nil {
		return "", "", "", "", "", fmt.Errorf("error writing server key: %w", err)
	}
	clientCertPEMBlock := &pem.Block{Type: "CERTIFICATE", Bytes: clientCertBytes}
	if err := pem.Encode(clientCertFile, clientCertPEMBlock); err != nil {
		return "", "", "", "", "", fmt.Errorf("error writing client certificate: %w", err)
	}
	clientKeyPEMBlock := &pem.Block{Type: "EC PRIVATE KEY", Bytes: clientKeyBytes}
	if err := pem.Encode(clientKeyFile, clientKeyPEMBlock); err != nil {
		return "", "", "", "", "", fmt.Errorf("error writing client key: %w", err)
	}

	// Return the file paths
	return caCertFile.Name(), serverCertFile.Name(), serverKeyFile.Name(), clientCertFile.Name(), clientKeyFile.Name(), nil
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"fmt"
	"os"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/tools/record"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
)

var (
	logger      = logf.Log.WithName("unit-tests")
	instanceUID = uuid.NewUUID()
)

const (
	defaultCollectorImage = "default-collector"
)

func deploymentParams() manifests.Params {
	return paramsWithMode(v1alpha1.ModeDeployment)
}

func paramsWithMode(mode v1alpha1.Mode) manifests.Params {
	replicas := int32(2)
	configJSON, err := os.ReadFile("testdata/test.json")
	if err != nil {
		fmt.Printf("Error getting yaml file: %v", err)
	}
	return manifests.Params{
		Config: config.New(config.WithCollectorImage(defaultCollectorImage)),
		OtelCol: v1alpha1.AmazonCloudWatchAgent{
			TypeMeta: metav1.TypeMeta{
				Kind:       "cloudwatch.aws.amazon.com",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "default",
				UID:       instanceUID,
			},
			Spec: v1alpha1.AmazonCloudWatchAgentSpec{
				Image: "public.ecr.aws/cloudwatch-agent/cloudwatch-agent:0.0.0",
				Ports: []v1.ServicePort{{
					Name: "web",
					Port: 80,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 80,
					},
					NodePort: 0,
				}},
				Replicas: &replicas,
				Config:   string(configJSON),
				Mode:     mode,
			},
		},
		Log:      logger,
		Recorder: record.NewFakeRecorder(10),
	}
}

func otelConfigParams() manifests.Params {
	configYAML, err := os.ReadFile("testdata/otel-test.yaml")
	if err != nil {
		fmt.Printf("Error getting yaml file: %v", err)
	}
	return paramsWithOtelConfig(string(configYAML))
}

func paramsWithOtelConfig(otelCfg string) manifests.Params {
	replicas := int32(2)
	configJSON, err := os.ReadFile("testdata/test.json")
	if err != nil {
		fmt.Printf("Error getting yaml file: %v", err)
	}
	return manifests.Params{
		Config: config.New(config.WithCollectorImage(defaultCollectorImage)),
		OtelCol: v1alpha1.AmazonCloudWatchAgent{
			TypeMeta: metav1.TypeMeta{
				Kind:       "cloudwatch.aws.amazon.com",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "default",
				UID:       instanceUID,
			},
			Spec: v1alpha1.AmazonCloudWatchAgentSpec{
				Image: "public.ecr.aws/cloudwatch-agent/cloudwatch-agent:0.0.0",
				Ports: []v1.ServicePort{{
					Name: "web",
					Port: 80,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 80,
					},
					NodePort: 0,
				}},
				Replicas:   &replicas,
				Config:     string(configJSON),
				OtelConfig: otelCfg,
				Mode:       v1alpha1.ModeDeployment,
			},
		},
		Log:      logger,
		Recorder: record.NewFakeRecorder(10),
	}
}

func newParams(taContainerImage string, file string) (manifests.Params, error) {
	replicas := int32(1)
	var configJSON []byte
	var err error

	if file == "" {
		configJSON, err = os.ReadFile("testdata/test.json")
	} else {
		configJSON, err = os.ReadFile(file)
	}
	if err != nil {
		return manifests.Params{}, fmt.Errorf("Error getting json file: %w", err)
	}

	cfg := config.New(
		config.WithCollectorImage(defaultCollectorImage),
	)

	return manifests.Params{
		Config: cfg,
		OtelCol: v1alpha1.AmazonCloudWatchAgent{
			TypeMeta: metav1.TypeMeta{
				Kind:       "cloudwatch.aws.amazon.com",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "default",
				UID:       instanceUID,
			},
			Spec: v1alpha1.AmazonCloudWatchAgentSpec{
				Mode: v1alpha1.ModeStatefulSet,
				Ports: []v1.ServicePort{{
					Name: "web",
					Port: 80,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 80,
					},
					NodePort: 0,
				}},
				Replicas: &replicas,
				Config:   string(configJSON),
			},
		},
		Log: logger,
	}, nil
}

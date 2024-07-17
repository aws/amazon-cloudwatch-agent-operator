// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifests

import (
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1beta1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
)

// Params holds the reconciliation-specific parameters.
type Params struct {
	Client    client.Client
	Recorder  record.EventRecorder
	Scheme    *runtime.Scheme
	Log       logr.Logger
	OtelCol   v1beta1.AmazonCloudWatchAgent
	DcgmExp   v1beta1.DcgmExporter
	NeuronExp v1beta1.NeuronMonitor
	Config    config.Config
}

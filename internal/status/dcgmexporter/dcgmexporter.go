// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dcgmexporter

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/version"
)

func UpdateDcgmExporterStatus(ctx context.Context, cli client.Client, changed *v1alpha1.DcgmExporter) error {
	if changed.Status.Version == "" {
		changed.Status.Version = version.DcgmExporter()
	}

	return nil
}

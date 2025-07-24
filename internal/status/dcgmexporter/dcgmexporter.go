// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dcgmexporter

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/version"
)

func extractVersionFromImage(image string) string {
	if image == "" {
		return ""
	}

	// Split by ':' to get the tag part
	parts := strings.Split(image, ":")
	if len(parts) < 2 {
		return ""
	}

	// Return the tag (version) part
	return parts[len(parts)-1]
}

func UpdateDcgmExporterStatus(ctx context.Context, cli client.Client, changed *v1alpha1.DcgmExporter) error {
	// Get DaemonSet status (DCGM Exporter runs as DaemonSet)
	objKey := client.ObjectKey{
		Namespace: changed.GetNamespace(),
		Name:      naming.DcgmExporter(changed.Name),
	}

	obj := &appsv1.DaemonSet{}
	if err := cli.Get(ctx, objKey, obj); err != nil {
		// If DaemonSet doesn't exist yet, set default values
		changed.Status.Scale.StatusReplicas = "0/0"
		changed.Status.Image = ""
		if changed.Status.Version == "" {
			changed.Status.Version = version.DcgmExporter()
		}
		return nil
	}

	// Update replica status for READY column
	readyReplicas := obj.Status.NumberReady
	totalReplicas := obj.Status.DesiredNumberScheduled
	changed.Status.Scale.StatusReplicas = fmt.Sprintf("%d/%d", readyReplicas, totalReplicas)

	// Update image for IMAGE column
	if len(obj.Spec.Template.Spec.Containers) > 0 {
		changed.Status.Image = obj.Spec.Template.Spec.Containers[0].Image

		// Extract and set version from image tag if not already set
		if changed.Status.Version == "" || changed.Status.Version == "0.0.0" {
			if imageVersion := extractVersionFromImage(changed.Status.Image); imageVersion != "" {
				changed.Status.Version = imageVersion
			} else {
				changed.Status.Version = version.DcgmExporter()
			}
		}
	}

	return nil
}

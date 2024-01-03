// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package upgrade_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/version"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/collector/upgrade"
)

var logger = logf.Log.WithName("unit-tests")

func TestShouldUpgradeAllToLatestBasedOnUpgradeStrategy(t *testing.T) {
	const beginV = "0.0.1" // this is the first version we have an upgrade function

	currentV := version.Get()
	currentV.AmazonCloudWatchAgent = upgrade.Latest.String()

	for _, tt := range []struct {
		strategy  v1alpha1.UpgradeStrategy
		expectedV string
	}{
		{v1alpha1.UpgradeStrategyAutomatic, upgrade.Latest.String()},
		{v1alpha1.UpgradeStrategyNone, beginV},
	} {
		t.Run("spec.UpgradeStrategy = "+string(tt.strategy), func(t *testing.T) {
			// prepare
			nsn := types.NamespacedName{Name: "my-instance", Namespace: "default"}
			existing := makeOtelcol(nsn, v1alpha1.ManagementStateManaged)
			err := k8sClient.Create(context.Background(), &existing)
			require.NoError(t, err)

			existing.Status.Version = beginV
			err = k8sClient.Status().Update(context.Background(), &existing)
			require.NoError(t, err)

			// sanity check
			persisted := &v1alpha1.AmazonCloudWatchAgent{}
			err = k8sClient.Get(context.Background(), nsn, persisted)
			require.NoError(t, err)
			require.Equal(t, beginV, persisted.Status.Version)
			up := &upgrade.VersionUpgrade{
				Log:      logger,
				Version:  currentV,
				Client:   k8sClient,
				Recorder: record.NewFakeRecorder(upgrade.RecordBufferSize),
			}

			// test
			err = up.ManagedInstances(context.Background())
			assert.NoError(t, err)

			// verify
			err = k8sClient.Get(context.Background(), nsn, persisted)
			assert.NoError(t, err)
			assert.Equal(t, upgrade.Latest.String(), persisted.Status.Version)

			// cleanup
			assert.NoError(t, k8sClient.Delete(context.Background(), &existing))
		})
	}
}

func TestUpgradeUpToLatestKnownVersion(t *testing.T) {
	for _, tt := range []struct {
		desc      string
		v         string
		expectedV string
	}{
		{"upgrade-routine", "0.8.0", "0.10.0"},     // we don't have a 0.10.0 upgrade, but we have a 0.9.0
		{"no-upgrade-routine", "0.61.1", "0.62.0"}, // No upgrade routines between these two versions
	} {
		t.Run(tt.desc, func(t *testing.T) {
			// prepare
			nsn := types.NamespacedName{Name: "my-instance", Namespace: "default"}
			existing := makeOtelcol(nsn, v1alpha1.ManagementStateManaged)
			existing.Status.Version = tt.v

			currentV := version.Get()
			currentV.AmazonCloudWatchAgent = tt.expectedV
			up := &upgrade.VersionUpgrade{
				Log:      logger,
				Version:  currentV,
				Client:   k8sClient,
				Recorder: record.NewFakeRecorder(upgrade.RecordBufferSize),
			}
			// test
			res, err := up.ManagedInstance(context.Background(), existing)

			// verify
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedV, res.Status.Version)
		})
	}
}

func TestVersionsShouldNotBeChanged(t *testing.T) {
	nsn := types.NamespacedName{Name: "my-instance", Namespace: "default"}
	for _, tt := range []struct {
		desc            string
		v               string
		expectedV       string
		failureExpected bool
		managementState v1alpha1.ManagementStateType
	}{
		{"new-instance", "", "", false, v1alpha1.ManagementStateManaged},
		{"newer-than-our-newest", "100.0.0", "100.0.0", false, v1alpha1.ManagementStateManaged},
		{"unparseable", "unparseable", "unparseable", true, v1alpha1.ManagementStateManaged},
		// Ignore unmanaged instances
		{"unmanaged-instance", "1.0.0", "1.0.0", false, v1alpha1.ManagementStateUnmanaged},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			// prepare
			existing := makeOtelcol(nsn, tt.managementState)
			existing.Status.Version = tt.v

			currentV := version.Get()
			currentV.AmazonCloudWatchAgent = upgrade.Latest.String()

			up := &upgrade.VersionUpgrade{
				Log:      logger,
				Version:  currentV,
				Client:   k8sClient,
				Recorder: record.NewFakeRecorder(upgrade.RecordBufferSize),
			}

			// test
			res, err := up.ManagedInstance(context.Background(), existing)
			if tt.failureExpected {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// verify
			assert.Equal(t, tt.expectedV, res.Status.Version)
		})
	}
}

func makeOtelcol(nsn types.NamespacedName, managementState v1alpha1.ManagementStateType) v1alpha1.AmazonCloudWatchAgent {
	return v1alpha1.AmazonCloudWatchAgent{
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			ManagementState: managementState,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsn.Name,
			Namespace: nsn.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
			},
		},
	}
}

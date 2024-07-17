// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1beta1"
)

func TestUpdateCollectorStatusUnsupported(t *testing.T) {
	ctx := context.TODO()
	cli := client.Client(fake.NewFakeClient())

	changed := &v1beta1.AmazonCloudWatchAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sidecar",
			Namespace: "default",
		},
		Spec: v1beta1.AmazonCloudWatchAgentSpec{
			Mode: v1beta1.ModeSidecar,
		},
	}

	err := UpdateCollectorStatus(ctx, cli, changed)
	assert.NoError(t, err)

	assert.Equal(t, int32(0), changed.Status.Scale.Replicas, "expected replicas to be 0")
	assert.Equal(t, "", changed.Status.Scale.Selector, "expected selector to be empty")
}

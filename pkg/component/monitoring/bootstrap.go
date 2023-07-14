// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package monitoring

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/component"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Values is a set of configuration values for the monitoring components.
type Values struct {
	// GlobalMonitoringSecret is the global monitoring secret for the garden cluster.
	GlobalMonitoringSecret *corev1.Secret
	// HVPAEnabled states whether HVPA is enabled or not.
	HVPAEnabled bool
}

// New creates a new instance of DeployWaiter for the monitoring components.
func New(
	client client.Client,
	namespace string,
	values Values,
) component.Deployer {
	return &bootstrapper{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type bootstrapper struct {
	client    client.Client
	namespace string
	values    Values
}

func (b *bootstrapper) Deploy(ctx context.Context) error {
	if b.values.HVPAEnabled {
		if err := kubernetesutils.DeleteObjects(ctx, b.client,
			&vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "prometheus-vpa", Namespace: r.GardenNamespace}},
			&vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "aggregate-prometheus-vpa", Namespace: r.GardenNamespace}},
		); err != nil {
			return err
		}
	}

	return nil
}

func (b *bootstrapper) Destroy(ctx context.Context) error {
	return nil
}

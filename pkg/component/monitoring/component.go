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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/operation/common"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// Values is a set of configuration values for the monitoring components.
type Values struct {
}

// New creates a new instance of DeployWaiter for the monitoring components.
func New(
	client client.Client,
	chartApplier kubernetes.ChartApplier,
	secretsManager secretsmanager.Interface,
	namespace string,
	values Values,
) component.Deployer {
	return &monitoring{
		client:         client,
		chartApplier:   chartApplier,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type monitoring struct {
	client         client.Client
	chartApplier   kubernetes.ChartApplier
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

func (m *monitoring) Deploy(ctx context.Context) error {
	return nil
}

func (m *monitoring) Destroy(ctx context.Context) error {
	if err := common.DeleteAlertmanager(ctx, m.client, m.namespace); err != nil {
		return err
	}

	objects := []client.Object{
		&networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.namespace,
				Name:      "allow-from-prometheus",
			},
		},
		&networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.namespace,
				Name:      "allow-prometheus",
			},
		},
		gardenerutils.NewShootAccessSecret(v1beta1constants.StatefulSetNamePrometheus, m.namespace).Secret,
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.namespace,
				Name:      "prometheus-config",
			},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.namespace,
				Name:      "prometheus-rules",
			},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.namespace,
				Name:      "blackbox-exporter-config-prometheus",
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.namespace,
				Name:      "prometheus-basic-auth",
			},
		},
		&networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.namespace,
				Name:      "prometheus",
			},
		},
		&vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.namespace,
				Name:      "prometheus-vpa",
			},
		},
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.namespace,
				Name:      "prometheus",
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.namespace,
				Name:      "prometheus-web",
			},
		},
		&appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.namespace,
				Name:      "prometheus",
			},
		},
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.namespace,
				Name:      "prometheus-" + m.namespace,
			},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.namespace,
				Name:      "prometheus-db-prometheus-0",
			},
		},
	}

	return kubernetesutils.DeleteObjects(ctx, m.client, objects...)
}

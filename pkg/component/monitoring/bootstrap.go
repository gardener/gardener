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
	"embed"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

var (
	//go:embed charts/bootstrap
	chartBootstrap     embed.FS
	chartPathBootstrap = filepath.Join("charts", "bootstrap")
)

// ValuesBootstrap is a set of configuration values for the monitoring components.
type ValuesBootstrap struct {
	// GlobalMonitoringSecret is the global monitoring secret for the garden cluster.
	GlobalMonitoringSecret *corev1.Secret
	// HVPAEnabled states whether HVPA is enabled or not.
	HVPAEnabled bool
	// ImageAlpine is the image of Alpine.
	ImageAlpine string
	// ImageConfigmapReloader is the image of ConfigmapReloader.
	ImageConfigmapReloader string
	// ImagePrometheus is the image of Prometheus.
	ImagePrometheus string
	// IngressHost is the host name of Prometheus.
	IngressHost string
	// SeedName is the name of the seed.
	SeedName string
	// StorageCapacityAggregatePrometheus is the storage capacity of AggregatePrometheus.
	StorageCapacityAggregatePrometheus string
	// WildcardCertName is name of wildcard tls certificate which is issued for the seed's ingress domain.
	WildcardCertName *string
}

// NewBootstrap creates a new instance of Deployer for the monitoring components.
func NewBootstrap(
	client client.Client,
	chartApplier kubernetes.ChartApplier,
	secretsManager secretsmanager.Interface,
	namespace string,
	values ValuesBootstrap,
) component.Deployer {
	return &bootstrapper{
		client:         client,
		chartApplier:   chartApplier,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type bootstrapper struct {
	client         client.Client
	chartApplier   kubernetes.ChartApplier
	namespace      string
	secretsManager secretsmanager.Interface
	values         ValuesBootstrap
}

func (b *bootstrapper) Deploy(ctx context.Context) error {
	if b.values.HVPAEnabled {
		if err := kubernetesutils.DeleteObjects(ctx, b.client,
			&vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "prometheus-vpa", Namespace: b.namespace}},
			&vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "aggregate-prometheus-vpa", Namespace: b.namespace}},
		); err != nil {
			return err
		}
	}

	// Monitoring resource values
	monitoringResources := map[string]interface{}{
		"aggregate-prometheus": map[string]interface{}{},
	}

	if b.values.HVPAEnabled {
		for resource := range monitoringResources {
			currentResources, err := kubernetesutils.GetContainerResourcesInStatefulSet(ctx, b.client, kubernetesutils.Key(b.namespace, resource))
			if err != nil {
				return err
			}
			if len(currentResources) != 0 && currentResources["prometheus"] != nil {
				monitoringResources[resource] = map[string]interface{}{
					"prometheus": currentResources["prometheus"],
				}
			}
		}
	}

	var (
		vpaGK    = schema.GroupKind{Group: "autoscaling.k8s.io", Kind: "VerticalPodAutoscaler"}
		hvpaGK   = schema.GroupKind{Group: "autoscaling.k8s.io", Kind: "Hvpa"}
		issuerGK = schema.GroupKind{Group: "certmanager.k8s.io", Kind: "ClusterIssuer"}

		applierOptions          = kubernetes.CopyApplierOptions(kubernetes.DefaultMergeFuncs)
		retainStatusInformation = func(new, old *unstructured.Unstructured) {
			// Apply status from old Object to retain status information
			new.Object["status"] = old.Object["status"]
		}
	)

	applierOptions[vpaGK] = retainStatusInformation
	applierOptions[hvpaGK] = retainStatusInformation
	applierOptions[issuerGK] = retainStatusInformation

	values := kubernetes.Values(map[string]interface{}{
		"global": map[string]interface{}{
			"ingressClass": v1beta1constants.SeedNginxIngressClass,
			"images": map[string]string{
				"alpine":             b.values.ImageAlpine,
				"configmap-reloader": b.values.ImageConfigmapReloader,
				"prometheus":         b.values.ImagePrometheus,
			},
		},
		"aggregatePrometheus": map[string]interface{}{
			"resources": monitoringResources["aggregate-prometheus"],
			"storage":   b.values.StorageCapacityAggregatePrometheus,
		},
		"hvpa": map[string]interface{}{
			"enabled": b.values.HVPAEnabled,
		},
		"istio": map[string]interface{}{
			"enabled": true,
		},
		"ingress": map[string]interface{}{
			"authSecretName": b.values.GlobalMonitoringSecret.Name,
		},
	})

	return b.chartApplier.ApplyFromEmbeddedFS(ctx, chartBootstrap, chartPathBootstrap, b.namespace, "monitoring", values, applierOptions)
}

func (b *bootstrapper) Destroy(_ context.Context) error {
	return nil
}

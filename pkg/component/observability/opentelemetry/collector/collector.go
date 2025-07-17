// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"context"
	"time"

	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	valiconstants "github.com/gardener/gardener/pkg/component/observability/logging/vali/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	managedResourceName            = "opentelemetry-collector"
	otelCollectorConfigName        = "opentelemetry-collector-config"
	timeoutWaitForManagedResources = 2 * time.Minute
)

// Values is the values for otel-collector configurations
type Values struct {
	// Image is the collector image.
	Image string
}

type otelCollector struct {
	client       client.Client
	namespace    string
	values       Values
	lokiEndpoint string
}

// New creates a new instance of otel-collector deployer.
func New(
	client client.Client,
	namespace string,
	values Values,
	lokiEndpoint string,
) component.DeployWaiter {
	return &otelCollector{
		client:       client,
		namespace:    namespace,
		values:       values,
		lokiEndpoint: lokiEndpoint,
	}
}

func (o *otelCollector) Deploy(ctx context.Context) error {
	registry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

	serializedResources, err := registry.AddAllAndSerialize(o.openTelemetryCollector(o.namespace, o.lokiEndpoint))
	if err != nil {
		return err
	}

	return managedresources.CreateForSeedWithLabels(ctx, o.client, o.namespace, managedResourceName, false, map[string]string{v1beta1constants.LabelCareConditionType: v1beta1constants.ObservabilityComponentsHealthy}, serializedResources)
}

func (o *otelCollector) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, o.client, o.namespace, managedResourceName)
}

func (o *otelCollector) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, o.client, o.namespace, managedResourceName)
}

func (o *otelCollector) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, o.client, o.namespace, managedResourceName)
}

func (o *otelCollector) openTelemetryCollector(namespace, lokiEndpoint string) *otelv1beta1.OpenTelemetryCollector {
	return &otelv1beta1.OpenTelemetryCollector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "opentelemetry-collector",
			Namespace: namespace,
			Labels:    getLabels(),
		},
		Spec: otelv1beta1.OpenTelemetryCollectorSpec{
			Mode:            "deployment",
			UpgradeStrategy: "none",
			OpenTelemetryCommonFields: otelv1beta1.OpenTelemetryCommonFields{
				Image: o.values.Image,
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: ptr.To(false),
				},
			},
			Config: otelv1beta1.Config{
				Receivers: otelv1beta1.AnyConfig{
					Object: map[string]any{
						"loki": map[string]any{
							"protocols": map[string]any{
								"http": map[string]any{
									"endpoint": "0.0.0.0:4317",
								},
							},
						},
					},
				},
				Processors: &otelv1beta1.AnyConfig{
					Object: map[string]any{
						"batch": map[string]any{
							"timeout": "10s",
						},
						"attributes/labels": map[string]any{
							"actions": []any{
								map[string]any{
									"key":    "loki.attribute.labels",
									"value":  "job, unit, nodename, origin, pod_name, container_name, origin, namespace_name, nodename, gardener_cloud_role",
									"action": "insert",
								},
								map[string]any{
									"key":    "loki.format",
									"value":  "logfmt",
									"action": "insert",
								},
							},
						},
					},
				},
				Exporters: otelv1beta1.AnyConfig{
					Object: map[string]any{
						"loki": map[string]any{
							"endpoint": lokiEndpoint,
						},
					},
				},
				Service: otelv1beta1.Service{
					Pipelines: map[string]*otelv1beta1.Pipeline{
						"logs": {
							Exporters:  []string{"loki"},
							Receivers:  []string{"loki"},
							Processors: []string{"attributes/labels", "batch"},
						},
					},
				},
			},
		},
	}
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelRole:  v1beta1constants.LabelObservability,
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleObservability,
		gardenerutils.NetworkPolicyLabel(valiconstants.ServiceName, valiconstants.ValiPort): v1beta1constants.LabelNetworkPolicyAllowed,
		v1beta1constants.LabelNetworkPolicyToDNS:                                            v1beta1constants.LabelNetworkPolicyAllowed,
		v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer:                               v1beta1constants.LabelNetworkPolicyAllowed,
		v1beta1constants.LabelObservabilityApplication:                                      "opentelemetry-collector",
	}
}

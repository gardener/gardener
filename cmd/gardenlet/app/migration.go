// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/cmd/internal/migration"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/observability/opentelemetry/collector/constants"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

func (g *garden) runMigrations(ctx context.Context, log logr.Logger) error {
	if features.DefaultFeatureGate.Enabled(features.VPAInPlaceUpdates) {
		if err := migration.MigrateVPAEmptyPatch(ctx, g.mgr.GetClient(), log); err != nil {
			return fmt.Errorf("failed to migrate VerticalPodAutoscaler with 'MigrateVPAEmptyPatch' migration: %w", err)
		}
	} else {
		if err := migration.MigrateVPAUpdateModeToRecreate(ctx, g.mgr.GetClient(), log); err != nil {
			return fmt.Errorf("failed to migrate VerticalPodAutoscaler with 'MigrateVPAUpdateModeToRecreate' migration: %w", err)
		}
	}

	if features.DefaultFeatureGate.Enabled(features.OpenTelemetryCollector) {
		if err := migrateOTelCollectorAnnotations(ctx, g.mgr.GetClient(), log); err != nil {
			return fmt.Errorf("failed to migrate OpenTelemetry Collector annotations: %w", err)
		}
	}

	return nil
}

// TODO(nickytd): Remove this migration in the following release after the one that introduces it.
func migrateOTelCollectorAnnotations(ctx context.Context, seedClient client.Client, log logr.Logger) error {
	namespaceList := &corev1.NamespaceList{}
	if err := seedClient.List(ctx, namespaceList, client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot}); err != nil {
		return fmt.Errorf("failed listing namespaces: %w", err)
	}

	var tasks []flow.TaskFn

	for _, namespace := range namespaceList.Items {
		if namespace.DeletionTimestamp != nil || namespace.Status.Phase == corev1.NamespaceTerminating {
			continue
		}

		tasks = append(tasks, func(ctx context.Context) error {
			var (
				otelCollectorKey             = client.ObjectKey{Namespace: namespace.Name, Name: constants.OpenTelemetryCollectorResourceName}
				otelCollectorManagedResource = &resourcesv1alpha1.ManagedResource{}
			)

			// Get the otel collector managed resource and check if it is already migrated.
			if err := seedClient.Get(ctx, otelCollectorKey, otelCollectorManagedResource); err != nil {
				if apierrors.IsNotFound(err) {
					log.Info("Managed resource not found, skipping migration", "managedResource", otelCollectorKey)
					return nil
				}
				return fmt.Errorf("failed to get ManagedResource %q: %w", otelCollectorKey, err)
			}
			if otelCollectorManagedResource.DeletionTimestamp != nil {
				log.Info("Managed resource is in deletion, skipping migration", "managedResource", otelCollectorKey)
				return nil
			}

			otelCollectorManagedResourceObjects, err := managedresources.GetObjects(ctx, seedClient, otelCollectorManagedResource.Namespace, otelCollectorManagedResource.Name)
			if err != nil {
				if apierrors.IsNotFound(err) {
					log.Info("Managed resource secret not found, skipping migration", "managedResource", otelCollectorKey)
					return nil
				}
				return fmt.Errorf("failed to get objects for ManagedResource %q: %w", otelCollectorKey, err)
			}

			var otelCollector client.Object
			var migratedOtelCollectorManagedResourceObjects = make([]client.Object, len(otelCollectorManagedResourceObjects))
			for _, object := range otelCollectorManagedResourceObjects {
				if _, ok := object.(*otelv1beta1.OpenTelemetryCollector); ok && object.GetName() == constants.OpenTelemetryCollectorResourceName {
					otelCollector = object
					continue
				}
				migratedOtelCollectorManagedResourceObjects = append(migratedOtelCollectorManagedResourceObjects, object)
			}

			if otelCollector == nil {
				log.Info("OpenTelemetry Collector object not found in ManagedResource, skipping migration", "managedResource", otelCollectorKey)
				return nil
			}

			// Cast to OpenTelemetryCollector type
			otelCollectorTyped, ok := otelCollector.(*otelv1beta1.OpenTelemetryCollector)
			if !ok {
				return fmt.Errorf("failed to cast object to OpenTelemetryCollector")
			}

			// Check if the annotations are present.
			annotations := otelCollector.GetAnnotations()
			if annotations == nil {
				annotations = make(map[string]string)
			}

			var (
				expectedNamespaceSelectors = `[{"matchLabels":{"kubernetes.io/metadata.name":"garden"}}]`
			)

			// Check NetworkingPodLabelSelectorNamespaceAlias annotation
			if val, exists := annotations[resourcesv1alpha1.NetworkingPodLabelSelectorNamespaceAlias]; !exists || val != v1beta1constants.LabelNetworkPolicyShootNamespaceAlias {
				log.Info("Annotation missing or incorrect, will patch",
					"annotation", resourcesv1alpha1.NetworkingPodLabelSelectorNamespaceAlias,
					"expected", v1beta1constants.LabelNetworkPolicyShootNamespaceAlias,
					"current", val,
					"collector", otelCollector.GetName())
				annotations[resourcesv1alpha1.NetworkingPodLabelSelectorNamespaceAlias] = v1beta1constants.LabelNetworkPolicyShootNamespaceAlias
			}

			// Check NetworkingNamespaceSelectors annotation
			if val, exists := annotations[resourcesv1alpha1.NetworkingNamespaceSelectors]; !exists || val != expectedNamespaceSelectors {
				log.Info("Annotation missing or incorrect, will patch",
					"annotation", resourcesv1alpha1.NetworkingNamespaceSelectors,
					"expected", expectedNamespaceSelectors,
					"current", val,
					"collector", otelCollector.GetName())
				annotations[resourcesv1alpha1.NetworkingNamespaceSelectors] = expectedNamespaceSelectors
			}

			// patch the OpenTelemetry Collector pipeline configuration
			ensureOtelColPipelineConfiguration(otelCollectorTyped, log)

			otelCollector.SetAnnotations(annotations)
			migratedOtelCollectorManagedResourceObjects = append(migratedOtelCollectorManagedResourceObjects, otelCollector)

			otelCollectorRegistry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

			var otelCollectorManagedResources map[string][]byte
			otelCollectorManagedResources, err = otelCollectorRegistry.AddAllAndSerialize(migratedOtelCollectorManagedResourceObjects...)
			if err != nil {
				return fmt.Errorf("failed serializing objects for ManagedResource %q: %w", otelCollectorKey, err)
			}
			if err = managedresources.CreateForSeedWithLabels(ctx, seedClient, otelCollectorManagedResource.Namespace, otelCollectorManagedResource.Name, false, map[string]string{v1beta1constants.LabelCareConditionType: v1beta1constants.ObservabilityComponentsHealthy}, otelCollectorManagedResources); err != nil {
				return fmt.Errorf("failed updating ManagedResource %q: %w", otelCollectorKey, err)
			}

			timeoutSeedCtx, cancelSeedCtx := context.WithTimeout(ctx, 2*time.Minute)
			defer cancelSeedCtx()
			if err = managedresources.WaitUntilHealthy(timeoutSeedCtx, seedClient, otelCollectorManagedResource.Namespace, otelCollectorManagedResource.Name); err != nil {
				return fmt.Errorf("waiting for ManagedResource %q to become healthy failed: %w", otelCollectorKey, err)
			}
			log.Info("Successfully migrated collector annotations", "collector", otelCollector.GetName())

			return nil
		})
	}
	return flow.Parallel(tasks...)(ctx)
}

// ensureOtelColPipelineConfiguration ensures the OpenTelemetry Collector has the required pipeline configuration.
// Always applies the expected configuration.
func ensureOtelColPipelineConfiguration(collector *otelv1beta1.OpenTelemetryCollector, log logr.Logger) {
	log.Info("Applying otelcol pipeline configuration", "collector", collector.Name)

	// Set receivers configuration
	if collector.Spec.Config.Receivers.Object == nil {
		collector.Spec.Config.Receivers.Object = make(map[string]any)
	}
	collector.Spec.Config.Receivers.Object["otlp"] = map[string]any{
		"protocols": map[string]any{
			"grpc": map[string]any{
				"endpoint": "[::]:4317",
			},
		},
	}

	// Set processors configuration
	if collector.Spec.Config.Processors == nil {
		collector.Spec.Config.Processors = &otelv1beta1.AnyConfig{Object: make(map[string]any)}
	}
	if collector.Spec.Config.Processors.Object == nil {
		collector.Spec.Config.Processors.Object = make(map[string]any)
	}

	collector.Spec.Config.Processors.Object["batch"] = map[string]any{
		"timeout": "10s",
	}

	collector.Spec.Config.Processors.Object["resource/vali"] = map[string]any{
		"attributes": []any{
			map[string]any{"action": "insert", "from_attribute": "k8s.node.name", "key": "nodename"},
			map[string]any{"action": "insert", "from_attribute": "k8s.pod.name", "key": "pod_name"},
			map[string]any{"action": "insert", "from_attribute": "k8s.container.name", "key": "container_name"},
			map[string]any{"action": "insert", "from_attribute": "k8s.namespace.name", "key": "namespace_name"},
			map[string]any{"action": "insert", "key": "loki.resource.labels", "value": "job, unit, nodename, origin, pod_name, container_name, namespace_name, gardener_cloud_role"},
			map[string]any{"action": "insert", "key": "loki.format", "value": "raw"},
		},
	}

	collector.Spec.Config.Processors.Object["attributes/vali"] = map[string]any{
		"actions": []any{
			map[string]any{"action": "insert", "from_attribute": "k8s.node.name", "key": "nodename"},
			map[string]any{"action": "insert", "from_attribute": "k8s.pod.name", "key": "pod_name"},
			map[string]any{"action": "insert", "from_attribute": "k8s.container.name", "key": "container_name"},
			map[string]any{"action": "insert", "from_attribute": "k8s.namespace.name", "key": "namespace_name"},
			map[string]any{"action": "upsert", "key": "loki.attribute.labels", "value": "priority, level, process.command, process.pid, host.name, host.id, service.name, service.namespace, job, unit, nodename, origin, pod_name, container_name, namespace_name, gardener_cloud_role"},
			map[string]any{"action": "insert", "key": "loki.format", "value": "raw"},
		},
	}

	// Set exporters configuration
	if collector.Spec.Config.Exporters.Object == nil {
		collector.Spec.Config.Exporters.Object = make(map[string]any)
	}

	collector.Spec.Config.Exporters.Object["loki"] = map[string]any{
		"endpoint": "http://logging:3100/vali/api/v1/push",
		"default_labels_enabled": map[string]any{
			"exporter": false,
			"job":      false,
		},
	}

	collector.Spec.Config.Exporters.Object["debug/logs"] = map[string]any{
		"verbosity": "basic",
	}

	// Set service pipelines configuration
	if collector.Spec.Config.Service.Pipelines == nil {
		collector.Spec.Config.Service.Pipelines = make(map[string]*otelv1beta1.Pipeline)
	}

	collector.Spec.Config.Service.Pipelines["logs/vali"] = &otelv1beta1.Pipeline{
		Receivers:  []string{"otlp"},
		Processors: []string{"resource/vali", "attributes/vali", "batch"},
		Exporters:  []string{"loki", "debug/logs"},
	}
}

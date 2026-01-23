// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package migration

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/observability/opentelemetry/collector/constants"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

// MigrateOTelCollectorAnnotations migrates the annotations of OpenTelemetryCollector resources in all shoot control plane namespaces.
// It ensures that the required networking annotations are present allowing ingress traffic from the fluent-bits in the garden namespace.
func MigrateOTelCollectorAnnotations(ctx context.Context, c client.Client, log logr.Logger) error {
	log.Info("Migrating OpentelemetryCollectors annotations in shoots control plane namespaces")
	if err := migrateOTelCollectorAnnotations(ctx, c, log); err != nil {
		return fmt.Errorf("failed migrating OpentelemetryCollectors: %w", err)
	}

	return nil
}

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

			// Check if the annotations are present.
			annotations := otelCollector.GetAnnotations()
			if annotations == nil {
				annotations = make(map[string]string)
			}

			var (
				needsUpdate                = false
				expectedNamespaceSelectors = `[{"matchLabels":{"kubernetes.io/metadata.name":"garden"}}]`
			)

			// Check NetworkingPodLabelSelectorNamespaceAlias annotation
			if val, exists := annotations[resourcesv1alpha1.NetworkingPodLabelSelectorNamespaceAlias]; !exists || val != v1beta1constants.LabelNetworkPolicyShootNamespaceAlias {
				needsUpdate = true
				log.Info("Annotation missing or incorrect, will patch",
					"annotation", resourcesv1alpha1.NetworkingPodLabelSelectorNamespaceAlias,
					"expected", v1beta1constants.LabelNetworkPolicyShootNamespaceAlias,
					"current", val,
					"collector", otelCollector.GetName())
				annotations[resourcesv1alpha1.NetworkingPodLabelSelectorNamespaceAlias] = v1beta1constants.LabelNetworkPolicyShootNamespaceAlias
			}

			// Check NetworkingNamespaceSelectors annotation
			if val, exists := annotations[resourcesv1alpha1.NetworkingNamespaceSelectors]; !exists || val != expectedNamespaceSelectors {
				needsUpdate = true
				log.Info("Annotation missing or incorrect, will patch",
					"annotation", resourcesv1alpha1.NetworkingNamespaceSelectors,
					"expected", expectedNamespaceSelectors,
					"current", val,
					"collector", otelCollector.GetName())
				annotations[resourcesv1alpha1.NetworkingNamespaceSelectors] = expectedNamespaceSelectors
			}

			// Perform patch if needed
			if needsUpdate {
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

				twoMinutes := 2 * time.Minute
				timeoutSeedCtx, cancelSeedCtx := context.WithTimeout(ctx, twoMinutes)
				defer cancelSeedCtx()
				if err = managedresources.WaitUntilHealthy(timeoutSeedCtx, seedClient, otelCollectorManagedResource.Namespace, otelCollectorManagedResource.Name); err != nil {
					return fmt.Errorf("waiting for ManagedResource %q to become healthy failed: %w", otelCollectorKey, err)
				}

				log.Info("Successfully migrated collector annotations", "collector", otelCollector.GetName())
			} else {
				log.Info("Collector already has correct annotations, skipping migration", "collector", otelCollector.GetName())
			}

			return nil
		})
	}
	return flow.Parallel(tasks...)(ctx)
}

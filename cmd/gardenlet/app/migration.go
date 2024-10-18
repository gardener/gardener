// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

func (g *garden) runMigrations(ctx context.Context, log logr.Logger) error {
	log.Info("Migrating deprecated failure-domain.beta.kubernetes.io labels to topology.kubernetes.io")
	if err := migrateDeprecatedTopologyLabels(ctx, log, g.mgr.GetClient(), g.mgr.GetConfig()); err != nil {
		return err
	}

	log.Info("Migrating targetRef of shoot alertmanager VPAs")
	if err := migrateAlertManagerVPAs(ctx, log, g.mgr.GetClient()); err != nil {
		return err
	}

	return nil
}

// TODO(oliver-goetz): Remove this function after v1.108 has been released.
func migrateAlertManagerVPAs(ctx context.Context, log logr.Logger, seedClient client.Client) error {
	managedResourceList := v1alpha1.ManagedResourceList{}

	if err := seedClient.List(ctx, &managedResourceList, client.MatchingLabels{v1beta1constants.LabelCareConditionType: v1beta1constants.ObservabilityComponentsHealthy}); err != nil {
		if meta.IsNoMatchError(err) {
			return nil
		}
		return fmt.Errorf("failed listing ManagedResources: %w", err)
	}

	var taskFns []flow.TaskFn

	for _, managedResource := range managedResourceList.Items {
		if managedResource.Name != "alertmanager-shoot" {
			continue
		}

		taskFns = append(taskFns, func(ctx context.Context) error {
			objects, err := managedresources.GetObjects(ctx, seedClient, managedResource.Namespace, managedResource.Name)
			if err != nil {
				return fmt.Errorf("failed getting objects for ManagedResource %q: %w", client.ObjectKeyFromObject(&managedResource), err)
			}

			var needUpdate bool
			for i, obj := range objects {
				vpa, ok := obj.(*vpaautoscalingv1.VerticalPodAutoscaler)
				if !ok || vpa.Name != "alertmanager-shoot" {
					continue
				}
				if vpa.Spec.TargetRef != nil && vpa.Spec.TargetRef.Kind != "Alertmanager" {
					vpa.Spec.TargetRef = &autoscalingv1.CrossVersionObjectReference{
						APIVersion: monitoringv1.SchemeGroupVersion.String(),
						Kind:       "Alertmanager",
						Name:       "shoot",
					}
					objects[i] = vpa
					needUpdate = true
					break
				}
			}

			if !needUpdate {
				return nil
			}

			log.Info("Migrating targetRef of alertmanager VPA in managed resource", "managedResource", client.ObjectKeyFromObject(&managedResource))
			registry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
			resources, err := registry.AddAllAndSerialize(objects...)
			if err != nil {
				return fmt.Errorf("failed serializing objects for managed resource %q: %w", client.ObjectKeyFromObject(&managedResource), err)
			}

			return managedresources.CreateForSeedWithLabels(ctx, seedClient, managedResource.Namespace, managedResource.Name, false, managedResource.Labels, resources)
		})
	}

	return flow.Parallel(taskFns...)(ctx)
}

// TODO: Remove this function when Kubernetes 1.27 support gets dropped.
func migrateDeprecatedTopologyLabels(ctx context.Context, log logr.Logger, seedClient client.Client, restConfig *rest.Config) error {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed creating discovery client: %w", err)
	}

	version, err := discoveryClient.ServerVersion()
	if err != nil {
		return fmt.Errorf("failed reading the server version of seed cluster: %w", err)
	}

	seedVersion, err := semver.NewVersion(version.GitVersion)
	if err != nil {
		return fmt.Errorf("failed parsing server version to semver: %w", err)
	}

	//  PV node affinities were immutable until Kubernetes 1.27, see https://github.com/kubernetes/kubernetes/pull/115391
	if !versionutils.ConstraintK8sGreaterEqual127.Check(seedVersion) {
		return nil
	}

	persistentVolumeList := &corev1.PersistentVolumeList{}
	if err := seedClient.List(ctx, persistentVolumeList); err != nil {
		return fmt.Errorf("failed listing persistent volumes for migrating deprecated topology labels: %w", err)
	}

	var taskFns []flow.TaskFn

	for _, pv := range persistentVolumeList.Items {
		persistentVolume := pv

		taskFns = append(taskFns, func(ctx context.Context) error {
			patch := client.MergeFrom(persistentVolume.DeepCopy())

			if persistentVolume.Spec.NodeAffinity == nil {
				// when PV is very old and has no node affinity, we just replace the topology labels
				if v, ok := persistentVolume.Labels[corev1.LabelFailureDomainBetaRegion]; ok {
					persistentVolume.Labels[corev1.LabelTopologyRegion] = v
				}
				if v, ok := persistentVolume.Labels[corev1.LabelFailureDomainBetaZone]; ok {
					persistentVolume.Labels[corev1.LabelTopologyZone] = v
				}
			} else if persistentVolume.Spec.NodeAffinity.Required != nil {
				// when PV has node affinity then we do not need the labels but just need to replace the topology keys
				// in the node selector term match expressions
				for i, term := range persistentVolume.Spec.NodeAffinity.Required.NodeSelectorTerms {
					for j, expression := range term.MatchExpressions {
						if expression.Key == corev1.LabelFailureDomainBetaRegion {
							persistentVolume.Spec.NodeAffinity.Required.NodeSelectorTerms[i].MatchExpressions[j].Key = corev1.LabelTopologyRegion
						}

						if expression.Key == corev1.LabelFailureDomainBetaZone {
							persistentVolume.Spec.NodeAffinity.Required.NodeSelectorTerms[i].MatchExpressions[j].Key = corev1.LabelTopologyZone
						}
					}
				}
			}

			// either new topology labels were added above, or node affinity keys were adjusted
			// in both cases, the old, deprecated topology labels are no longer needed and can be removed
			delete(persistentVolume.Labels, corev1.LabelFailureDomainBetaRegion)
			delete(persistentVolume.Labels, corev1.LabelFailureDomainBetaZone)

			// prevent sending empty patches
			if data, err := patch.Data(&persistentVolume); err != nil {
				return fmt.Errorf("failed getting patch data for PV %s: %w", persistentVolume.Name, err)
			} else if string(data) == `{}` {
				return nil
			}

			log.Info("Migrating deprecated topology labels", "persistentVolumeName", persistentVolume.Name)
			return seedClient.Patch(ctx, &persistentVolume, patch)
		})
	}

	return flow.Parallel(taskFns...)(ctx)
}

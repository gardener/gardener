// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package migration

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
)

const (
	// vpaParallelWorkersCount defines the number of workers processing the VPAs resources in parallel.
	vpaParallelWorkersCount = 50
)

// vpaMigrationConfig defines a VerticalPodAutoscaler migration configuration.
type vpaMigrationConfig struct {
	migrationName string
	log           logr.Logger
	client        client.Client
	listOpts      client.ListOptions
	filterFn      func(vpa *vpaautoscalingv1.VerticalPodAutoscaler) bool
	mutateFn      func(vpa *vpaautoscalingv1.VerticalPodAutoscaler) *vpaautoscalingv1.VerticalPodAutoscaler
}

// migrateVPA performs VerticalPodAutoscalers migration based on the provided configuration.
func migrateVPA(ctx context.Context, cfg *vpaMigrationConfig) error {
	list := vpaautoscalingv1.VerticalPodAutoscalerList{}

	if err := cfg.client.List(ctx, &list, &cfg.listOpts); err != nil {
		if meta.IsNoMatchError(err) {
			cfg.log.Info("Resources kind not found, skipping migration",
				"kind", "VerticalPodAutoscaler",
				"migration", cfg.migrationName,
			)
			return nil
		}
		return fmt.Errorf("failed listing VerticalPodAutoscaler resources: %w", err)
	}

	var tasks []flow.TaskFn
	for _, vpa := range list.Items {
		task := func(ctx context.Context) error {
			if isValid := cfg.filterFn(&vpa); !isValid {
				return nil
			}

			key := client.ObjectKeyFromObject(&vpa)
			cfg.log.Info("Migrating VerticalPodAutoscaler resource",
				"vpa", key,
				"migration", cfg.migrationName,
			)

			patch := client.MergeFrom(vpa.DeepCopy())
			vpaNew := cfg.mutateFn(&vpa)
			if err := cfg.client.Patch(ctx, vpaNew, patch); err != nil {
				if apierrors.IsNotFound(err) {
					cfg.log.Info("Resource not found, skipping migration",
						"vpa", key,
						"migration", cfg.migrationName,
					)
				}
				return fmt.Errorf("failed to patch VerticalPodAutoscaler '%s': %w", key, err)
			}
			return nil
		}
		tasks = append(tasks, task)
	}

	return flow.ParallelN(vpaParallelWorkersCount, tasks...)(ctx)
}

// MigrateVPAEmptyPatch performs an empty patch updates to VerticalPodAutoscaler resources,
// that are elighible to adopt the InPlaceOrRecreate update mode, but are filtered out, because of
// the Resource Manager's alwaysUpdate=false configuration.
// TODO(vitanovs): Remove the migration once the VPAInPlaceUpdates feature gate is promoted to GA.
func MigrateVPAEmptyPatch(ctx context.Context, c client.Client, log logr.Logger) error {
	log.Info("Migrating VerticalPodAutoscalers")
	var (
		labelSkipShouldNotExist    = utils.MustNewRequirement(resourcesv1alpha1.VPAInPlaceUpdatesSkip, selection.DoesNotExist)
		labelMutatedShouldNotExist = utils.MustNewRequirement(resourcesv1alpha1.VPAInPlaceUpdatesMutated, selection.DoesNotExist)
		labelSelector              = labels.NewSelector().Add(
			labelMutatedShouldNotExist,
			labelSkipShouldNotExist,
		)
		listOpts = client.ListOptions{
			LabelSelector: labelSelector,
		}

		filterFn = func(vpa *vpaautoscalingv1.VerticalPodAutoscaler) bool {
			if vpa.Namespace == metav1.NamespaceSystem || vpa.Namespace == v1beta1constants.KubernetesDashboardNamespace {
				return false
			}

			updateMode := ptr.Deref(vpa.Spec.UpdatePolicy.UpdateMode, vpaautoscalingv1.UpdateModeRecreate)
			if updateMode != vpaautoscalingv1.UpdateModeAuto && updateMode != vpaautoscalingv1.UpdateModeRecreate {
				return false
			}

			return true
		}
		mutateFunc = func(vpa *vpaautoscalingv1.VerticalPodAutoscaler) *vpaautoscalingv1.VerticalPodAutoscaler {
			return vpa
		}
	)

	cfg := vpaMigrationConfig{
		migrationName: "MigrateVPAEmptyPatch",
		log:           log,
		client:        c,
		listOpts:      listOpts,
		filterFn:      filterFn,
		mutateFn:      mutateFunc,
	}
	if err := migrateVPA(ctx, &cfg); err != nil {
		return err
	}

	log.Info("Successfully migrated VerticalPodAutoscalers")
	return nil
}

// MigrateVPAUpdateModeToRecreate applies a patch to VerticalPodAutoscaler resources that
// sets their update modes to Recreate, in order to undo the change applied by a GRM mutation webhook.
// TODO(vitanovs): Remove the migration once the VPAInPlaceUpdates feature gate is promoted to GA.
func MigrateVPAUpdateModeToRecreate(ctx context.Context, c client.Client, log logr.Logger) error {
	log.Info("Migrating VerticalPodAutoscalers to update mode Recreate")

	var (
		labelSkipShouldNotExist = utils.MustNewRequirement(resourcesv1alpha1.VPAInPlaceUpdatesSkip, selection.DoesNotExist)
		labelMutatedShouldExist = utils.MustNewRequirement(resourcesv1alpha1.VPAInPlaceUpdatesMutated, selection.Exists)
		labelSelector           = labels.NewSelector().Add(
			labelMutatedShouldExist,
			labelSkipShouldNotExist,
		)
		listOpts = client.ListOptions{
			LabelSelector: labelSelector,
		}

		filterFn = func(vpa *vpaautoscalingv1.VerticalPodAutoscaler) bool {
			if vpa.Namespace == metav1.NamespaceSystem || vpa.Namespace == v1beta1constants.KubernetesDashboardNamespace {
				return false
			}
			return true
		}
		mutateFn = func(vpa *vpaautoscalingv1.VerticalPodAutoscaler) *vpaautoscalingv1.VerticalPodAutoscaler {
			vpa.Spec.UpdatePolicy.UpdateMode = ptr.To(vpaautoscalingv1.UpdateModeRecreate)
			delete(vpa.Labels, resourcesv1alpha1.VPAInPlaceUpdatesMutated)
			return vpa
		}
	)

	cfg := vpaMigrationConfig{
		migrationName: "MigrateVPAUpdateModeToRecreate",
		log:           log,
		client:        c,
		listOpts:      listOpts,
		filterFn:      filterFn,
		mutateFn:      mutateFn,
	}
	if err := migrateVPA(ctx, &cfg); err != nil {
		return err
	}

	log.Info("Successfully migrated VerticalPodAutoscalers to update mode Recreate")
	return nil
}

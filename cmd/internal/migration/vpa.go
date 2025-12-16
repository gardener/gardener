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
	vpaParallelWorkersCount = 5
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
	var (
		vpaList = vpaautoscalingv1.VerticalPodAutoscalerList{}
	)

	if err := cfg.client.List(ctx, &vpaList, &cfg.listOpts); err != nil {
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
	for _, vpa := range vpaList.Items {
		task := func(ctx context.Context) error {
			if isValid := cfg.filterFn(&vpa); !isValid {
				return nil
			}

			vpaKey := client.ObjectKeyFromObject(&vpa)
			cfg.log.Info("Migrating VerticalPodAutoscaler resource",
				"vpa", vpaKey,
				"migration", cfg.migrationName,
			)

			patch := client.MergeFrom(vpa.DeepCopy())
			vpaNew := cfg.mutateFn(&vpa)
			if err := cfg.client.Patch(ctx, vpaNew, patch); err != nil {
				if apierrors.IsNotFound(err) {
					cfg.log.Info("Resource not found, skipping migration",
						"vpa", vpaKey,
						"migration", cfg.migrationName,
					)
				}
				return fmt.Errorf("failed applying '%s' migration to VerticalPodAutoscaler '%s': %w", cfg.migrationName, vpaKey, err)
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
// TODO(vitanovs): Remove the migration once the VPAInPlaceUpdates feature gates promoted to GA.
func MigrateVPAEmptyPatch(ctx context.Context, c client.Client, log logr.Logger) error {
	log.Info("Migrating VerticalPodAutoscalers")
	var (
		vpaLabelSkipShouldNotExist    = utils.MustNewRequirement(resourcesv1alpha1.VPAInPlaceUpdatesSkip, selection.DoesNotExist)
		vpaLabelMutatedShouldNotExist = utils.MustNewRequirement(resourcesv1alpha1.VPAInPlaceUpdatesMutated, selection.DoesNotExist)
		labelSelector                 = labels.NewSelector().Add(
			vpaLabelMutatedShouldNotExist,
			vpaLabelSkipShouldNotExist,
		)
		vpaListOpts = client.ListOptions{
			LabelSelector: labelSelector,
		}

		vpaValidator = func(vpa *vpaautoscalingv1.VerticalPodAutoscaler) bool {
			if vpa.Namespace == metav1.NamespaceSystem || vpa.Namespace == v1beta1constants.KubernetesDashboardNamespace {
				return false
			}

			updateMode := ptr.Deref(vpa.Spec.UpdatePolicy.UpdateMode, vpaautoscalingv1.UpdateModeRecreate)
			if updateMode != vpaautoscalingv1.UpdateModeAuto && updateMode != vpaautoscalingv1.UpdateModeRecreate {
				return false
			}

			return true
		}
		vpaMutator = func(vpa *vpaautoscalingv1.VerticalPodAutoscaler) *vpaautoscalingv1.VerticalPodAutoscaler {
			return vpa
		}
	)

	cfg := vpaMigrationConfig{
		migrationName: "MigrateVPAEmptyPatch",
		log:           log,
		client:        c,
		listOpts:      vpaListOpts,
		filterFn:      vpaValidator,
		mutateFn:      vpaMutator,
	}
	return migrateVPA(ctx, &cfg)
}

// MigrateVPAUpdateModeToRecreate applies a patch to VerticalPodAutoscaler resources that
// sets their update modes to Recreate, in order to undo the change applied by a GRM mutation webhook.
// TODO(vitanovs): Remove the migration once the VPAInPlaceUpdates feature gates promoted to GA.
func MigrateVPAUpdateModeToRecreate(ctx context.Context, c client.Client, log logr.Logger) error {
	log.Info("Migrating VerticalPodAutoscalers to update mode Recreate")

	var (
		vpaLabelSkipShouldNotExist = utils.MustNewRequirement(resourcesv1alpha1.VPAInPlaceUpdatesSkip, selection.DoesNotExist)
		vpaLabelMutatedShouldExist = utils.MustNewRequirement(resourcesv1alpha1.VPAInPlaceUpdatesMutated, selection.Exists)
		labelSelector              = labels.NewSelector().Add(
			vpaLabelMutatedShouldExist,
			vpaLabelSkipShouldNotExist,
		)
		vpaListOpts = client.ListOptions{
			LabelSelector: labelSelector,
		}

		vpaValidator = func(vpa *vpaautoscalingv1.VerticalPodAutoscaler) bool {
			if vpa.Namespace == metav1.NamespaceSystem || vpa.Namespace == v1beta1constants.KubernetesDashboardNamespace {
				return false
			}
			return true
		}
		vpaMutator = func(vpa *vpaautoscalingv1.VerticalPodAutoscaler) *vpaautoscalingv1.VerticalPodAutoscaler {
			vpa.Spec.UpdatePolicy.UpdateMode = ptr.To(vpaautoscalingv1.UpdateModeRecreate)
			delete(vpa.Labels, resourcesv1alpha1.VPAInPlaceUpdatesMutated)
			return vpa
		}
	)

	cfg := vpaMigrationConfig{
		migrationName: "MigrateVPAUpdateModeToRecreate",
		log:           log,
		client:        c,
		listOpts:      vpaListOpts,
		filterFn:      vpaValidator,
		mutateFn:      vpaMutator,
	}
	return migrateVPA(ctx, &cfg)
}

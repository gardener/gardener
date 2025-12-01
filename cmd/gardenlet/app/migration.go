// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (g *garden) runMigrations(ctx context.Context, log logr.Logger) error {
	if enabled, ok := g.config.FeatureGates["VPAInPlaceUpdates"]; enabled && ok {
		if err := g.migrateEmptyVPAPatch(ctx, log); err != nil {
			return err
		}
	}
	return nil
}

func (g *garden) migrateEmptyVPAPatch(ctx context.Context, log logr.Logger) error {
	log.Info("Migrating VerticalPodAutoscalers")

	list := vpaautoscalingv1.VerticalPodAutoscalerList{}
	if err := g.mgr.GetClient().List(ctx, &list); err != nil {
		if meta.IsNoMatchError(err) {
			log.Info("Resources kind not found, skipping migration", "kind", "VerticalPodAutoscaler")
			return nil
		}
		return fmt.Errorf("failed listing VerticalPodAutoscaler resources: %w", err)
	}

	for _, vpa := range list.Items {
		_, vpaHasSkipLabel := vpa.Labels[resourcesv1alpha1.VPAInPlaceUpdatesSkip]
		_, vpaHasMutatedLabel := vpa.Labels[resourcesv1alpha1.VPAInPlaceUpdatesMutated]
		if vpaHasSkipLabel || vpaHasMutatedLabel {
			continue
		}

		if vpa.Namespace == "kube-system" || vpa.Namespace == "kubernetes-dashboards" {
			continue
		}

		vpaKey := client.ObjectKeyFromObject(&vpa)
		log.Info("Updating VerticalPodAutoscaler resource", "vpa", vpaKey)

		patch := client.MergeFrom(vpa.DeepCopy())
		if err := g.mgr.GetClient().Patch(ctx, &vpa, patch); err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("Resource not found, skipping migration", "vpa", vpaKey)
				continue
			}
			return fmt.Errorf("failed updating VerticalPodAutoscaler '%s': %w", vpaKey, err)
		}
	}
	return nil
}

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package migration

import (
	"context"
	"fmt"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// MigrateEmptyVPAPatch performs an empty patch updates to VerticalPodAutoscaler resources,
// that are elighible to adopt the InPlaceOrRecreate update mode, but are filtered out, because of
// the Resource Manager's alwaysUpdate=false configuration.
// TODO(vitanovs): Remove the migration once the VPAInPlaceUpdates feature gates promoted to GA.
func MigrateEmptyVPAPatch(ctx context.Context, mgr manager.Manager, log logr.Logger) error {
	log.Info("Migrating VerticalPodAutoscalers")

	list := vpaautoscalingv1.VerticalPodAutoscalerList{}
	if err := mgr.GetClient().List(ctx, &list); err != nil {
		if meta.IsNoMatchError(err) {
			log.Info("Resources kind not found, skipping migration", "kind", "VerticalPodAutoscaler")
			return nil
		}
		return fmt.Errorf("failed listing VerticalPodAutoscaler resources: %w", err)
	}

	for _, vpa := range list.Items {
		vpaHasSkipLabel := metav1.HasLabel(vpa.ObjectMeta, resourcesv1alpha1.VPAInPlaceUpdatesSkip)
		vpaHasMutatedLabel := metav1.HasLabel(vpa.ObjectMeta, resourcesv1alpha1.VPAInPlaceUpdatesMutated)
		if vpaHasSkipLabel || vpaHasMutatedLabel {
			continue
		}

		if vpa.Namespace == metav1.NamespaceSystem || vpa.Namespace == v1beta1constants.KubernetesDashboardNamespace {
			continue
		}

		vpaKey := client.ObjectKeyFromObject(&vpa)
		log.Info("Updating VerticalPodAutoscaler resource", "vpa", vpaKey)

		patch := client.MergeFrom(vpa.DeepCopy())
		if err := mgr.GetClient().Patch(ctx, &vpa, patch); err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("Resource not found, skipping migration", "vpa", vpaKey)
				continue
			}
			return fmt.Errorf("failed updating VerticalPodAutoscaler '%s': %w", vpaKey, err)
		}
	}
	return nil
}

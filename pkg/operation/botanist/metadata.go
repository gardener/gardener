// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/operation/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

// MaintainShootAnnotations ensures that given deprecated Shoot annotations are maintained also
// with their new equivalent in the Shoot metadata.
func (b *Botanist) MaintainShootAnnotations(ctx context.Context) error {
	if _, err := kutil.TryUpdateShootAnnotations(ctx, b.K8sGardenClient.GardenCore(), retry.DefaultRetry, b.Shoot.Info.ObjectMeta, func(shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
		deprecatedValue, deprecatedExists := shoot.Annotations[common.GardenCreatedByDeprecated]
		_, newExists := shoot.Annotations[common.GardenCreatedBy]
		if deprecatedExists {
			if !newExists {
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, common.GardenCreatedBy, deprecatedValue)
			}
			delete(shoot.Annotations, common.GardenCreatedByDeprecated)
		}

		return shoot, nil
	}); err != nil {
		return err
	}

	return nil
}

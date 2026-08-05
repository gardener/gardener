// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secretsrotation

import (
	"context"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// CheckIfGlobalObservabilitySecretPropagatedToAllSeeds waits until the global observability secret has been synced by the
// gardener-controller-manager from the garden namespace into every seed namespace of the virtual cluster.
func CheckIfGlobalObservabilitySecretPropagatedToAllSeeds(ctx context.Context, c client.Client, lastRotationInitiationTimestamp int64) error {
	seedList := &metav1.PartialObjectMetadataList{}
	seedList.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("SeedList"))
	if err := c.List(ctx, seedList); err != nil {
		return fmt.Errorf("failed to list seeds: %w", err)
	}

	for _, seed := range seedList.Items {
		seedNamespace := gardenerutils.ComputeGardenNamespace(seed.Name)

		secretList := &metav1.PartialObjectMetadataList{}
		secretList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("SecretList"))
		secretSelector := client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleGlobalMonitoring}
		if err := c.List(ctx, secretList, client.InNamespace(seedNamespace), secretSelector); err != nil {
			return fmt.Errorf("failed to list global observability secrets in namespace %q of seed %q: %w", seedNamespace, seed.Name, err)
		}

		for _, secret := range secretList.Items {
			secretLastRotationInitiationTime, ok := secret.Labels[secretsmanager.LabelKeyLastRotationInitiationTime]

			// accept missing last-rotation-initiation-time label values, e.g., human-managed secrets.
			if !ok {
				continue
			}

			// fail empty last-rotation-initiation-time label values, e.g., the secret is being rotated for the first time.
			if secretLastRotationInitiationTime == "" {
				return fmt.Errorf("global observability secret in namespace %q of seed %q does not yet carry a last rotation initiation time", seedNamespace, seed.Name)
			}

			secretLastRotationInitiationTimestamp, err := strconv.ParseInt(secretLastRotationInitiationTime, 10, 64)
			if err != nil {
				return fmt.Errorf("error parsing last rotation initiation time of global observability secret in namespace %q of seed %q: %w", seedNamespace, seed.Name, err)
			}
			if secretLastRotationInitiationTimestamp < lastRotationInitiationTimestamp {
				return fmt.Errorf("global observability secret is not yet propagated to namespace %q of seed %q", seedNamespace, seed.Name)
			}
		}
	}

	return nil
}

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secretsrotation

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// RenewGardenSecretsInAllSeeds annotates all seeds to trigger renewal of their garden secrets.
func RenewGardenSecretsInAllSeeds(ctx context.Context, log logr.Logger, c client.Client, operationAnnotation string) error {
	seedList := &metav1.PartialObjectMetadataList{}
	seedList.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("SeedList"))
	if err := c.List(ctx, seedList); err != nil {
		return err
	}

	log.Info("Seeds requiring renewal of their secrets", v1beta1constants.GardenerOperation, operationAnnotation, "number", len(seedList.Items))

	for _, seed := range seedList.Items {
		log := log.WithValues("seed", seed.Name)
		if seed.Annotations[v1beta1constants.GardenerOperation] == operationAnnotation {
			continue
		}

		if seed.Annotations[v1beta1constants.GardenerOperation] != "" {
			return fmt.Errorf("error annotating seed %s: already annotated with \"%s: %s\"", seed.Name, v1beta1constants.GardenerOperation, seed.Annotations[v1beta1constants.GardenerOperation])
		}

		seed.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("Seed"))
		patch := client.MergeFrom(seed.DeepCopy())
		kubernetesutils.SetMetaDataAnnotation(&seed.ObjectMeta, v1beta1constants.GardenerOperation, operationAnnotation)
		if err := c.Patch(ctx, &seed, patch); err != nil {
			return fmt.Errorf("error annotating seed %s: %w", seed.Name, err)
		}
		log.Info("Successfully annotated seed to renew its secrets", v1beta1constants.GardenerOperation, operationAnnotation)
	}

	return nil
}

// CheckIfGardenSecretsRenewalCompletedInAllSeeds checks if renewal of garden secrets is completed for all seeds.
func CheckIfGardenSecretsRenewalCompletedInAllSeeds(ctx context.Context, c client.Client, operationAnnotation string, secretType string) error {
	seedList := &metav1.PartialObjectMetadataList{}
	seedList.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("SeedList"))
	if err := c.List(ctx, seedList); err != nil {
		return err
	}

	for _, seed := range seedList.Items {
		if seed.Annotations[v1beta1constants.GardenerOperation] == operationAnnotation {
			return fmt.Errorf("renewing %q secrets for seed %q is not yet completed", secretType, seed.Name)
		}
	}

	return nil
}

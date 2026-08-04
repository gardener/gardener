// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secretsrotation

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenletutils "github.com/gardener/gardener/pkg/utils/gardener/gardenlet"
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

	var tasks []flow.TaskFn
	for _, seed := range seedList.Items {
		if seed.Annotations[v1beta1constants.GardenerOperation] == operationAnnotation {
			continue
		}

		if seed.Annotations[v1beta1constants.GardenerOperation] != "" {
			return fmt.Errorf("error annotating seed %s: already annotated with \"%s: %s\"", seed.Name, v1beta1constants.GardenerOperation, seed.Annotations[v1beta1constants.GardenerOperation])
		}

		tasks = append(tasks, func(ctx context.Context) error {
			log := log.WithValues("seed", seed.Name)

			seed.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("Seed"))
			patch := client.MergeFrom(seed.DeepCopy())
			kubernetesutils.SetMetaDataAnnotation(&seed.ObjectMeta, v1beta1constants.GardenerOperation, operationAnnotation)
			if err := c.Patch(ctx, &seed, patch); err != nil {
				return fmt.Errorf("error annotating seed %s: %w", seed.Name, err)
			}
			log.Info("Successfully annotated seed to renew its secrets", v1beta1constants.GardenerOperation, operationAnnotation)
			return nil
		})
	}

	return flow.ParallelN(5, tasks...)(ctx)
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

// RenewKubeconfigInAllShootGardenlets annotates all Gardenlet objects for self-hosted shoots to trigger renewal of
// their garden cluster kubeconfig.
func RenewKubeconfigInAllShootGardenlets(ctx context.Context, log logr.Logger, c client.Client) error {
	gardenletList := &metav1.PartialObjectMetadataList{}
	gardenletList.SetGroupVersionKind(seedmanagementv1alpha1.SchemeGroupVersion.WithKind("GardenletList"))
	if err := c.List(ctx, gardenletList, client.InNamespace(v1beta1constants.GardenNamespace)); err != nil {
		return err
	}

	gardenletList.Items = slices.DeleteFunc(gardenletList.Items, func(objectMeta metav1.PartialObjectMetadata) bool {
		return !strings.HasPrefix(objectMeta.Name, gardenletutils.ResourcePrefixSelfHostedShoot)
	})

	log.Info("Gardenlets requiring renewal of their kubeconfig", "number", len(gardenletList.Items))

	var tasks []flow.TaskFn
	for _, gardenlet := range gardenletList.Items {
		if gardenlet.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRenewKubeconfig {
			continue
		}

		if gardenlet.Annotations[v1beta1constants.GardenerOperation] != "" {
			return fmt.Errorf("error annotating gardenlet %s: already annotated with \"%s: %s\"", client.ObjectKeyFromObject(&gardenlet), v1beta1constants.GardenerOperation, gardenlet.Annotations[v1beta1constants.GardenerOperation])
		}

		tasks = append(tasks, func(ctx context.Context) error {
			log := log.WithValues("gardenlet", client.ObjectKeyFromObject(&gardenlet))

			gardenlet.SetGroupVersionKind(seedmanagementv1alpha1.SchemeGroupVersion.WithKind("Gardenlet"))
			patch := client.MergeFrom(gardenlet.DeepCopy())
			kubernetesutils.SetMetaDataAnnotation(&gardenlet.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationRenewKubeconfig)
			if err := c.Patch(ctx, &gardenlet, patch); err != nil {
				return fmt.Errorf("error annotating Gardenlet %s: %w", client.ObjectKeyFromObject(&gardenlet), err)
			}
			log.Info("Successfully annotated gardenlet to renew its kubeconfig")
			return nil
		})
	}

	return flow.ParallelN(5, tasks...)(ctx)
}

// CheckIfKubeconfigRenewalCompletedInAllShootGardenlets checks if renewal of the garden cluster kubeconfig is
// completed for all Gardenlet objects for self-hosted shoots.
func CheckIfKubeconfigRenewalCompletedInAllShootGardenlets(ctx context.Context, c client.Client) error {
	gardenletList := &metav1.PartialObjectMetadataList{}
	gardenletList.SetGroupVersionKind(seedmanagementv1alpha1.SchemeGroupVersion.WithKind("GardenletList"))
	if err := c.List(ctx, gardenletList, client.InNamespace(v1beta1constants.GardenNamespace)); err != nil {
		return err
	}

	gardenletList.Items = slices.DeleteFunc(gardenletList.Items, func(objectMeta metav1.PartialObjectMetadata) bool {
		return !strings.HasPrefix(objectMeta.Name, gardenletutils.ResourcePrefixSelfHostedShoot)
	})

	for _, gardenlet := range gardenletList.Items {
		if gardenlet.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRenewKubeconfig {
			return fmt.Errorf("renewing kubeconfig for Gardenlet %s is not yet completed", client.ObjectKeyFromObject(&gardenlet))
		}
	}

	return nil
}

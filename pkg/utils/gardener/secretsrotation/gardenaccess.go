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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
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

// RenewGardenSecretsInAllSelfHostedShoots annotates all self-hosted Shoot resources (that are not also registered as
// seeds) to trigger renewal of their garden secrets.
func RenewGardenSecretsInAllSelfHostedShoots(ctx context.Context, log logr.Logger, c client.Client, operationAnnotation string) error {
	shootList := &metav1.PartialObjectMetadataList{}
	shootList.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("ShootList"))
	if err := c.List(ctx, shootList, client.MatchingLabels{v1beta1constants.ShootIsSelfHosted: "true"}); err != nil {
		return err
	}

	var shootsThatAreNoSeeds []metav1.PartialObjectMetadata
	for _, shoot := range shootList.Items {
		seed := &metav1.PartialObjectMetadata{}
		seed.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("Seed"))
		if err := c.Get(ctx, client.ObjectKey{Name: shoot.Name}, seed); err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("error checking whether self-hosted shoot %s is also registered as a seed: %w", client.ObjectKeyFromObject(&shoot), err)
			}
			shootsThatAreNoSeeds = append(shootsThatAreNoSeeds, shoot)
		}
	}

	log.Info("Self-hosted shoots requiring renewal of their secrets", v1beta1constants.GardenerOperation, operationAnnotation, "number", len(shootsThatAreNoSeeds))

	var tasks []flow.TaskFn
	for _, shoot := range shootsThatAreNoSeeds {
		existingOperations := v1beta1helper.GetShootGardenerOperations(shoot.Annotations)
		if slices.Contains(existingOperations, operationAnnotation) {
			continue
		}

		newAnnotation := strings.Join(append(existingOperations, operationAnnotation), v1beta1constants.GardenerOperationsSeparator)
		tasks = append(tasks, func(ctx context.Context) error {
			log := log.WithValues("shoot", client.ObjectKeyFromObject(&shoot))

			shoot.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot"))
			patch := client.MergeFrom(shoot.DeepCopy())
			kubernetesutils.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.GardenerOperation, newAnnotation)
			if err := c.Patch(ctx, &shoot, patch); err != nil {
				return fmt.Errorf("error annotating self-hosted shoot %s: %w", client.ObjectKeyFromObject(&shoot), err)
			}
			log.Info("Successfully annotated self-hosted shoot to renew its secrets", v1beta1constants.GardenerOperation, operationAnnotation)
			return nil
		})
	}

	return flow.ParallelN(5, tasks...)(ctx)
}

// CheckIfGardenSecretsRenewalCompletedInAllSelfHostedShoots checks if renewal of garden secrets is completed for all
// self-hosted Shoot resources (that are not also registered as seeds).
func CheckIfGardenSecretsRenewalCompletedInAllSelfHostedShoots(ctx context.Context, c client.Client, operationAnnotation string, secretType string) error {
	shootList := &metav1.PartialObjectMetadataList{}
	shootList.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("ShootList"))
	if err := c.List(ctx, shootList, client.MatchingLabels{v1beta1constants.ShootIsSelfHosted: "true"}); err != nil {
		return err
	}

	for _, shoot := range shootList.Items {
		if !slices.Contains(v1beta1helper.GetShootGardenerOperations(shoot.Annotations), operationAnnotation) {
			continue
		}

		seed := &metav1.PartialObjectMetadata{}
		seed.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("Seed"))
		if err := c.Get(ctx, client.ObjectKey{Name: shoot.Name}, seed); err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("error checking whether self-hosted shoot %s/%s is also registered as a seed: %w", shoot.Namespace, shoot.Name, err)
			}
			return fmt.Errorf("renewing %q secrets for self-hosted shoot %s/%s is not yet completed", secretType, shoot.Namespace, shoot.Name)
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

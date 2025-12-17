// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const managedResourceNameReferencedResources = "referenced-resources"

// DeployReferencedResources reads all referenced resources from the Garden cluster and writes a managed resource to the Seed cluster.
func (b *Botanist) DeployReferencedResources(ctx context.Context) error {
	unstructuredObjs, err := gardenerutils.PrepareReferencedResourcesForSeedCopy(ctx, b.GardenClient, b.Shoot.GetInfo().Spec.Resources, b.Shoot.GetInfo().Namespace, b.Shoot.ControlPlaneNamespace)
	if err != nil {
		return fmt.Errorf("failed to prepare referenced resources for seed copy: %w", err)
	}

	// Create managed resource from the slice of unstructured objects

	if err := managedresources.CreateFromUnstructured(
		ctx, b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, managedResourceNameReferencedResources,
		false, v1beta1constants.SeedResourceManagerClass, unstructuredObjs, false, nil,
	); err != nil {
		return fmt.Errorf("failed to create managed resource for referenced resources: %w", err)
	}

	// Reconcile secrets for referenced WorkloadIdentities
	if err := gardenerutils.ReconcileWorkloadIdentityReferencedResources(
		ctx, b.GardenClient, b.SeedClientSet.Client(), b.Shoot.GetInfo().Spec.Resources,
		b.Shoot.GetInfo().Namespace, b.Shoot.ControlPlaneNamespace, b.Shoot.GetInfo(),
	); err != nil {
		return fmt.Errorf("failed to reconcile referenced workload identities: %w", err)
	}

	return nil
}

// DestroyReferencedResources deletes the managed resource containing referenced resources from the Seed cluster.
func (b *Botanist) DestroyReferencedResources(ctx context.Context) error {
	if err := gardenerutils.DestroyWorkloadIdentityReferencedResources(ctx, b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace); err != nil {
		return fmt.Errorf("failed to destroy referenced workload identities: %w", err)
	}

	if err := client.IgnoreNotFound(managedresources.Delete(ctx, b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, managedResourceNameReferencedResources, false)); err != nil {
		return fmt.Errorf("failed to delete managed resource for referenced resources: %w", err)
	}

	return nil
}

// PopulateStaticManifestsFromSeedToShoot reads all Secrets in the seed's garden namespace labeled with
// gardener.cloud/purpose=shoot-static-manifest and copies them into the Shoot namespace. A ManagedResource is created
// referencing all of them.
func (b *Botanist) PopulateStaticManifestsFromSeedToShoot(ctx context.Context) error {
	var (
		managedResourceName = "static-manifests-propagated-from-seed"
		secretNamePrefix    = "static-manifests-"
		managedResource     = managedresources.NewForShoot(b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, managedResourceName, managedresources.LabelValueGardener, false)
	)

	secretListGardenNamespace := &corev1.SecretList{}
	if err := b.SeedClientSet.Client().List(ctx, secretListGardenNamespace, client.InNamespace(v1beta1constants.GardenNamespace), client.MatchingLabels{v1beta1constants.GardenerPurpose: v1beta1constants.GardenPurposeShootStaticManifest}); err != nil {
		return fmt.Errorf("failed listing secrets with static manifests in %s namespace: %w", v1beta1constants.GardenNamespace, err)
	}

	// remove Secrets whose selectors don't apply to the current Shoot
	var errs []error
	secretListGardenNamespace.Items = slices.DeleteFunc(secretListGardenNamespace.Items, func(secret corev1.Secret) bool {
		selectorJSON, ok := secret.Annotations[v1beta1constants.AnnotationStaticManifestsShootSelector]
		if !ok {
			return false
		}

		var selector metav1.LabelSelector
		if err := json.Unmarshal([]byte(selectorJSON), &selector); err != nil {
			errs = append(errs, fmt.Errorf("failed unmarshalling shoot selector %q for secret %q: %w", selectorJSON, client.ObjectKeyFromObject(&secret), err))
			return false
		}

		labelSelector, err := metav1.LabelSelectorAsSelector(&selector)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed parsing label selector %q for secret %q: %w", selectorJSON, client.ObjectKeyFromObject(&secret), err))
			return false
		}

		return !labelSelector.Matches(labels.Set(b.Shoot.GetInfo().Labels))
	})
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	secretListShootControlPlaneNamespace := &corev1.SecretList{}
	if err := b.SeedClientSet.Client().List(ctx, secretListShootControlPlaneNamespace, client.InNamespace(b.Shoot.ControlPlaneNamespace), client.MatchingLabels{v1beta1constants.GardenerPurpose: v1beta1constants.GardenPurposeShootStaticManifest}); err != nil {
		return fmt.Errorf("failed listing secrets with static manifests in %s namespace: %w", b.Shoot.ControlPlaneNamespace, err)
	}

	var tasks []flow.TaskFn

	// populate current secrets into Shoot's control plane namespace
	for _, secret := range secretListGardenNamespace.Items {
		secretInShootNamespace := secret.DeepCopy()
		secretInShootNamespace.SetName(secretNamePrefix + secret.Name)
		secretInShootNamespace.SetNamespace(b.Shoot.ControlPlaneNamespace)
		secretInShootNamespace.SetResourceVersion("")
		secretInShootNamespace.SetManagedFields(nil)

		tasks = append(tasks, func(ctx context.Context) error {
			_, err := controllerutils.GetAndCreateOrMergePatch(ctx, b.SeedClientSet.Client(), secretInShootNamespace, func() error {
				secretInShootNamespace.Immutable = ptr.To(false)
				secretInShootNamespace.Type = secret.Type
				secretInShootNamespace.Data = secret.Data
				return nil
			})
			return err
		})

		managedResource.WithSecretRef(secretInShootNamespace.Name)
	}

	// cleanup old Secrets from Shoot's control plane namespace
	for _, secret := range secretListShootControlPlaneNamespace.Items {
		if !slices.ContainsFunc(secretListGardenNamespace.Items, func(s corev1.Secret) bool {
			return secret.Name == secretNamePrefix+s.Name
		}) {
			tasks = append(tasks, func(ctx context.Context) error {
				return kubernetesutils.DeleteObject(ctx, b.SeedClientSet.Client(), &secret)
			})
		}
	}

	if err := flow.Parallel(tasks...)(ctx); err != nil {
		return fmt.Errorf("failed reconciling Secrets with static manifests in Shoot namespace: %w", err)
	}

	if len(secretListGardenNamespace.Items) == 0 {
		return managedResource.Delete(ctx)
	}
	return managedResource.Reconcile(ctx)
}

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package imagepullsecret

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/imagevector"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const managedResourceNameImagePullSecret = "image-pull-secret" // #nosec G101 -- No credential.

// Reconciler watches image pull secrets in the seed's scoped namespace (seed-<name>) on the
// garden cluster and propagates them to all extension and shoot control plane namespaces in the
// seed cluster.
type Reconciler struct {
	GardenClient client.Client
	SeedClient   client.Client
	SeedName     string
}

// Reconcile propagates an image pull secret to all target namespaces in the seed cluster.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	seedNamespace := gardenerutils.ComputeGardenNamespace(r.SeedName)

	// Fetch the secret from the seed-scoped namespace in the garden cluster.
	// gardener-controller-manager copies it there based on the annotation; the gardenlet only needs to propagate it.
	gardenSecret := &corev1.Secret{}
	if err := r.GardenClient.Get(ctx, client.ObjectKey{Namespace: seedNamespace, Name: request.Name}, gardenSecret); err != nil {
		if !apierrors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("error retrieving image pull secret from garden cluster: %w", err)
		}
		// Secret was removed from seed-<name> namespace. Clean up all copies from the seed cluster.
		log.V(1).Info("Secret is gone from seed namespace, cleaning up all copies from seed cluster")
		return reconcile.Result{}, r.deleteFromSeedCluster(ctx, request.Name)
	}

	// Sync the secret into the seed cluster's garden namespace.
	seedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gardenSecret.Name,
			Namespace: v1beta1constants.GardenNamespace,
		},
	}
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.SeedClient, seedSecret, func() error {
		seedSecret.Type = gardenSecret.Type
		seedSecret.Data = gardenSecret.Data
		seedSecret.Labels = utils.MergeStringMaps(gardenSecret.Labels, map[string]string{
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleImagePullSecret,
		})
		return nil
	}); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to sync image pull secret to seed cluster: %w", err)
	}

	// List all extension namespaces (gardener.cloud/role=extension).
	extensionNamespaces := &corev1.NamespaceList{}
	if err := r.SeedClient.List(ctx, extensionNamespaces, client.MatchingLabels{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleExtension,
	}); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to list extension namespaces: %w", err)
	}

	// List all shoot control plane namespaces (gardener.cloud/role=shoot).
	shootNamespaces := &corev1.NamespaceList{}
	if err := r.SeedClient.List(ctx, shootNamespaces, client.MatchingLabels{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot,
	}); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to list shoot control plane namespaces: %w", err)
	}

	var targetNamespaces []string
	for _, ns := range extensionNamespaces.Items {
		targetNamespaces = append(targetNamespaces, ns.Name)
	}
	var shootNamespaceNames []string
	for _, ns := range shootNamespaces.Items {
		targetNamespaces = append(targetNamespaces, ns.Name)
		shootNamespaceNames = append(shootNamespaceNames, ns.Name)
	}

	log.Info("Propagating image pull secret", "secret", seedSecret.Name, "targetNamespaces", len(targetNamespaces))

	var propagateFns []flow.TaskFn
	for _, namespace := range targetNamespaces {
		ns := namespace
		propagateFns = append(propagateFns, func(ctx context.Context) error {
			targetSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      seedSecret.Name,
					Namespace: ns,
				},
			}
			if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.SeedClient, targetSecret, func() error {
				targetSecret.Type = seedSecret.Type
				targetSecret.Data = seedSecret.Data
				targetSecret.Labels = utils.MergeStringMaps(map[string]string{
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleImagePullSecret,
				}, seedSecret.Labels)
				return nil
			}); err != nil {
				return fmt.Errorf("failed to propagate image pull secret %q to namespace %q: %w", seedSecret.Name, ns, err)
			}
			return nil
		})
	}
	if err := flow.Parallel(propagateFns...)(ctx); err != nil {
		return reconcile.Result{}, err
	}

	var managedResourceFns []flow.TaskFn
	for _, namespace := range shootNamespaceNames {
		ns := namespace
		managedResourceFns = append(managedResourceFns, func(ctx context.Context) error {
			return r.updateShootManagedResource(ctx, ns)
		})
	}
	return reconcile.Result{}, flow.Parallel(managedResourceFns...)(ctx)
}

// deleteFromSeedCluster removes the named secret from the seed cluster's garden namespace, all
// extension and shoot control-plane namespaces, and deletes the ManagedResource for each shoot
// namespace so that gardener-resource-manager cleans it up in the shoot cluster as well.
func (r *Reconciler) deleteFromSeedCluster(ctx context.Context, secretName string) error {
	if err := client.IgnoreNotFound(r.SeedClient.Delete(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: v1beta1constants.GardenNamespace},
	})); err != nil {
		return fmt.Errorf("failed to delete image pull secret %q from garden namespace: %w", secretName, err)
	}

	extensionNamespaces := &corev1.NamespaceList{}
	if err := r.SeedClient.List(ctx, extensionNamespaces, client.MatchingLabels{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleExtension,
	}); err != nil {
		return fmt.Errorf("failed to list extension namespaces: %w", err)
	}

	shootNamespaces := &corev1.NamespaceList{}
	if err := r.SeedClient.List(ctx, shootNamespaces, client.MatchingLabels{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot,
	}); err != nil {
		return fmt.Errorf("failed to list shoot namespaces: %w", err)
	}

	var fns []flow.TaskFn

	for _, ns := range extensionNamespaces.Items {
		ns := ns
		fns = append(fns, func(ctx context.Context) error {
			if err := client.IgnoreNotFound(r.SeedClient.Delete(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: ns.Name},
			})); err != nil {
				return fmt.Errorf("failed to delete image pull secret %q from namespace %q: %w", secretName, ns.Name, err)
			}
			return nil
		})
	}

	for _, ns := range shootNamespaces.Items {
		ns := ns
		fns = append(fns, func(ctx context.Context) error {
			if err := client.IgnoreNotFound(r.SeedClient.Delete(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: ns.Name},
			})); err != nil {
				return fmt.Errorf("failed to delete image pull secret %q from namespace %q: %w", secretName, ns.Name, err)
			}
			// Rebuild the ManagedResource from the remaining secrets rather than deleting it
			// entirely, to avoid removing other image pull secrets from the shoot cluster.
			if err := r.updateShootManagedResource(ctx, ns.Name); err != nil {
				return fmt.Errorf("failed to update ManagedResource in namespace %q: %w", ns.Name, err)
			}
			return nil
		})
	}

	return flow.Parallel(fns...)(ctx)
}

// updateShootManagedResource creates or updates the image-pull-secret ManagedResource in the
// given shoot CP namespace, causing gardener-resource-manager to apply the secrets to the shoot cluster.
func (r *Reconciler) updateShootManagedResource(ctx context.Context, namespace string) error {
	var secretNames []string
	for _, cred := range imagevector.AllContainerImagePullCredentials() {
		if cred.Type == "StaticSecret" && cred.SecretName != "" {
			secretNames = append(secretNames, cred.SecretName)
		}
	}
	if len(secretNames) == 0 {
		return nil
	}

	registry := managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

	for _, secretName := range secretNames {
		ns := &corev1.Secret{}
		if err := r.SeedClient.Get(ctx, client.ObjectKey{Name: secretName, Namespace: namespace}, ns); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("failed to get image pull secret %q from namespace %q: %w", secretName, namespace, err)
		}

		shootSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: metav1.NamespaceSystem,
			},
			Type: ns.Type,
			Data: ns.Data,
		}
		if err := registry.Add(shootSecret); err != nil {
			return fmt.Errorf("failed to add secret %q to registry: %w", secretName, err)
		}
	}

	serializedObjects, err := registry.SerializedObjects()
	if err != nil {
		return fmt.Errorf("failed to serialize objects for ManagedResource in namespace %q: %w", namespace, err)
	}

	if len(serializedObjects) == 0 {
		return managedresources.DeleteForShoot(ctx, r.SeedClient, namespace, managedResourceNameImagePullSecret)
	}

	return managedresources.CreateForShoot(ctx, r.SeedClient, namespace, managedResourceNameImagePullSecret, managedresources.LabelValueGardener, false, serializedObjects)
}

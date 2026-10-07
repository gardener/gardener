// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/gardener/operator"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	oldExtensionRuntimePrefix = "extension-"
	oldExtensionRuntimeSuffix = "-garden"
)

func runMigrations(ctx context.Context, c client.Client, log logr.Logger) manager.RunnableFunc {
	return func(context.Context) error {
		// TODO(timuthy): Remove migration after Gardener v1.153 has been released.
		if err := migrateExtensionManagedResources(ctx, c, log); err != nil {
			return err
		}
		// TODO(timuthy): Remove migration after Gardener v1.162 has been released.
		return DeleteStaleShootAccessSecrets(ctx, c, log)
	}
}

// DeleteStaleShootAccessSecrets deletes stale `shoot-access-*` secrets of class `shoot` in the garden namespace for
// which a valid `garden-access-*` replacement secret (same ServiceAccount, class `garden`) exists.
func DeleteStaleShootAccessSecrets(ctx context.Context, c client.Client, log logr.Logger) error {
	secretList := &corev1.SecretList{}
	if err := c.List(ctx, secretList, client.InNamespace(v1beta1constants.GardenNamespace), client.MatchingLabels{
		resourcesv1alpha1.ResourceManagerPurpose: resourcesv1alpha1.LabelPurposeTokenRequest,
		resourcesv1alpha1.ResourceManagerClass:   resourcesv1alpha1.ResourceManagerClassShoot,
	}); err != nil {
		return fmt.Errorf("failed listing shoot access secrets: %w", err)
	}

	var taskFns []flow.TaskFn
	for _, staleSecret := range secretList.Items {
		if !strings.HasPrefix(staleSecret.Name, gardenerutils.SecretNamePrefixShootAccess) {
			continue
		}

		taskFns = append(taskFns, func(ctx context.Context) error {
			return deleteStaleShootAccessSecret(ctx, c, log, staleSecret)
		})
	}

	return flow.Parallel(taskFns...)(ctx)
}

func deleteStaleShootAccessSecret(ctx context.Context, c client.Client, log logr.Logger, staleSecret corev1.Secret) error {
	suffix := strings.TrimPrefix(staleSecret.Name, gardenerutils.SecretNamePrefixShootAccess)
	replacementName := gardenerutils.SecretNamePrefixGardenAccess + suffix

	replacement := &corev1.Secret{}
	if err := c.Get(ctx, client.ObjectKey{Name: replacementName, Namespace: v1beta1constants.GardenNamespace}, replacement); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Skipping deletion of stale shoot access secret, no replacement found", "secret", client.ObjectKeyFromObject(&staleSecret), "replacement", replacementName)
			return nil
		}
		return fmt.Errorf("failed getting replacement secret %q: %w", replacementName, err)
	}

	if !isValidGardenAccessReplacement(&staleSecret, replacement) {
		log.V(1).Info("Skipping deletion of stale shoot access secret, replacement is not a valid garden access secret", "secret", client.ObjectKeyFromObject(&staleSecret), "replacement", replacementName)
		return nil
	}

	log.Info("Deleting stale shoot access secret", "secret", client.ObjectKeyFromObject(&staleSecret), "replacement", replacementName)
	if err := client.IgnoreNotFound(c.Delete(ctx, &staleSecret)); err != nil {
		return fmt.Errorf("failed deleting stale shoot access secret %q: %w", staleSecret.Name, err)
	}

	return nil
}

func isValidGardenAccessReplacement(stale, replacement *corev1.Secret) bool {
	return replacement.Labels[resourcesv1alpha1.ResourceManagerPurpose] == resourcesv1alpha1.LabelPurposeTokenRequest &&
		replacement.Labels[resourcesv1alpha1.ResourceManagerClass] == resourcesv1alpha1.ResourceManagerClassGarden &&
		replacement.Annotations[resourcesv1alpha1.ServiceAccountName] == stale.Annotations[resourcesv1alpha1.ServiceAccountName] &&
		replacement.Annotations[resourcesv1alpha1.ServiceAccountNamespace] == stale.Annotations[resourcesv1alpha1.ServiceAccountNamespace] &&
		metav1.HasAnnotation(replacement.ObjectMeta, resourcesv1alpha1.ServiceAccountTokenRenewTimestamp)
}

func migrateExtensionManagedResources(ctx context.Context, c client.Client, log logr.Logger) error {
	mrList := &resourcesv1alpha1.ManagedResourceList{}
	if err := c.List(ctx, mrList, client.InNamespace(v1beta1constants.GardenNamespace), client.MatchingLabels{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleSeedSystemComponent,
	}); err != nil {
		if meta.IsNoMatchError(err) {
			log.Info("ManagedResource CRD not found, skipping migration of extension ManagedResources")
			return nil
		}
		return fmt.Errorf("failed listing ManagedResources: %w", err)
	}

	var taskFns []flow.TaskFn
	for _, mr := range mrList.Items {
		if strings.HasPrefix(mr.Name, oldExtensionRuntimePrefix) && strings.HasSuffix(mr.Name, oldExtensionRuntimeSuffix) {
			taskFns = append(taskFns, func(ctx context.Context) error {
				return migrateExtensionManagedResource(ctx, c, log, mr)
			})
		}
	}

	return flow.Parallel(taskFns...)(ctx)
}

const oldResourceIgnoreAnnotation = "old.resources.gardener.cloud/ignore"

func migrateExtensionManagedResource(ctx context.Context, c client.Client, log logr.Logger, mr resourcesv1alpha1.ManagedResource) error {
	name := mr.Name
	extensionName := strings.TrimSuffix(strings.TrimPrefix(name, oldExtensionRuntimePrefix), oldExtensionRuntimeSuffix)
	newMRName := operator.ExtensionRuntimeManagedResourceName(extensionName)

	// Check if the currently processed ManagedResource is a false-positive, i.e. not managed by the Gardener Operator.
	// This is the case if the ManagedResource does not contain a Deployment in the expected runtime namespace.
	if !slices.ContainsFunc(mr.Status.Resources, func(r resourcesv1alpha1.ObjectReference) bool {
		return r.Kind == "Deployment" && r.Namespace == operator.ExtensionRuntimeNamespaceName(extensionName)
	}) {
		log.Info("Skipping migration of extension ManagedResource: not managed by gardener-operator", "name", name)
		return nil
	}

	log.Info("Migrating extension ManagedResource", "old", name, "new", newMRName)
	if len(mr.Spec.SecretRefs) != 1 {
		return fmt.Errorf("old ManagedResource %q has unexpected number of secret refs: %d", name, len(mr.Spec.SecretRefs))
	}

	patch := client.MergeFrom(mr.DeepCopy())

	// Check if the old ManagedResource has the ignore annotation set. If so, we will consider it for the new ManagedResource as well.
	oldMRIgnored := mr.Annotations[resourcesv1alpha1.Ignore] == "true"
	if oldMRIgnoredVal, ok := mr.Annotations[oldResourceIgnoreAnnotation]; ok {
		var err error
		oldMRIgnored, err = strconv.ParseBool(oldMRIgnoredVal)
		if err != nil {
			return fmt.Errorf("failed parsing old ignore annotation value %q for ManagedResource %q: %w", oldMRIgnoredVal, name, err)
		}
	} else {
		metav1.SetMetaDataAnnotation(&mr.ObjectMeta, oldResourceIgnoreAnnotation, strconv.FormatBool(oldMRIgnored))
	}

	metav1.SetMetaDataAnnotation(&mr.ObjectMeta, resourcesv1alpha1.Ignore, "true")
	mr.Spec.KeepObjects = new(true)
	if err := c.Patch(ctx, &mr, patch); err != nil {
		return fmt.Errorf("failed annotating old ManagedResource %q with ignore annotation: %w", name, err)
	}

	oldSecret := &corev1.Secret{}
	oldSecretName := mr.Spec.SecretRefs[0].Name
	if err := c.Get(ctx, client.ObjectKey{Name: oldSecretName, Namespace: v1beta1constants.GardenNamespace}, oldSecret); err != nil {
		return fmt.Errorf("failed getting old secret %q: %w", oldSecretName, err)
	}

	secretName, secret := managedresources.NewSecret(c, v1beta1constants.GardenNamespace, newMRName, oldSecret.Data, true)
	managedResource := managedresources.NewForSeed(c, v1beta1constants.GardenNamespace, newMRName, false).WithSecretRef(secretName)

	// Add the ignore annotation to the new ManagedResource if it was present on the old one.
	if oldMRIgnored {
		managedResource = managedResource.WithAnnotations(map[string]string{resourcesv1alpha1.Ignore: "true"})
	}

	if err := secret.Reconcile(ctx); err != nil {
		return fmt.Errorf("could not create or update secret of managed resources: %w", err)
	}

	if err := managedResource.Reconcile(ctx); err != nil {
		return fmt.Errorf("could not create or update managed resource: %w", err)
	}

	if !oldMRIgnored {
		if err := managedresources.WaitUntilHealthyAndNotProgressing(ctx, c, v1beta1constants.GardenNamespace, newMRName); err != nil {
			return fmt.Errorf("failed waiting for new ManagedResource %q to be healthy: %w", newMRName, err)
		}
	}

	if err := client.IgnoreNotFound(managedresources.DeleteForSeed(ctx, c, v1beta1constants.GardenNamespace, name)); err != nil {
		return fmt.Errorf("failed deleting old ManagedResource %q: %w", name, err)
	}

	if err := client.IgnoreNotFound(c.Delete(ctx, oldSecret)); err != nil {
		return fmt.Errorf("failed deleting old secret %q: %w", oldSecretName, err)
	}

	if err := managedresources.WaitUntilDeleted(ctx, c, v1beta1constants.GardenNamespace, name); err != nil {
		return fmt.Errorf("failed waiting for old ManagedResource %q to be deleted: %w", name, err)
	}

	log.Info("Successfully migrated extension ManagedResource", "old", name, "new", newMRName)
	return nil
}

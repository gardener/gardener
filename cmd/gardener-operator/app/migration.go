// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
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
		return migrateExtensionManagedResources(ctx, c, log)
	}
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

func migrateExtensionManagedResource(ctx context.Context, c client.Client, log logr.Logger, mr resourcesv1alpha1.ManagedResource) error {
	name := mr.Name
	extensionName := strings.TrimSuffix(strings.TrimPrefix(name, oldExtensionRuntimePrefix), oldExtensionRuntimeSuffix)
	newMRName := operator.ExtensionRuntimeManagedResourceName(extensionName)
	log.Info("Migrating extension ManagedResource", "old", name, "new", newMRName)

	patch := client.MergeFrom(mr.DeepCopy())
	metav1.SetMetaDataAnnotation(&mr.ObjectMeta, resourcesv1alpha1.Ignore, "true")
	mr.Spec.KeepObjects = new(true)
	if err := c.Patch(ctx, &mr, patch); err != nil {
		return fmt.Errorf("failed annotating old ManagedResource %q with ignore annotation: %w", name, err)
	}

	if len(mr.Spec.SecretRefs) != 1 {
		return fmt.Errorf("old ManagedResource %q has unexpected number of secret refs: %d", name, len(mr.Spec.SecretRefs))
	}

	oldSecret := &corev1.Secret{}
	oldSecretName := mr.Spec.SecretRefs[0].Name
	if err := c.Get(ctx, client.ObjectKey{Name: oldSecretName, Namespace: v1beta1constants.GardenNamespace}, oldSecret); err != nil {
		return fmt.Errorf("failed getting old secret %q: %w", oldSecretName, err)
	}

	if err := managedresources.CreateForSeed(ctx, c, v1beta1constants.GardenNamespace, newMRName, false, oldSecret.Data); err != nil {
		return fmt.Errorf("failed creating new ManagedResource %q: %w", newMRName, err)
	}

	if err := managedresources.WaitUntilHealthyAndNotProgressing(ctx, c, v1beta1constants.GardenNamespace, newMRName); err != nil {
		return fmt.Errorf("failed waiting for new ManagedResource %q to be healthy: %w", newMRName, err)
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

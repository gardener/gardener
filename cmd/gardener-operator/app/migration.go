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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

func runMigrations(ctx context.Context, reader client.Reader, c client.Client, log logr.Logger) manager.RunnableFunc {
	return func(context.Context) error {
		// TODO(timuthy): Remove migration after Gardener v1.155 has been released.
		if err := DeleteStaleShootAccessSecrets(ctx, c, log); err != nil {
			return err
		}

		// TODO(DobromirNPeev): Remove this migration once a release containing it has been published and clusters have been
		// upgraded past it.
		if err := gardenerutils.RemoveVPAInPlaceUpdatesMutatedLabel(ctx, reader, c); err != nil {
			return fmt.Errorf("failed removing vpa-in-place-updates mutated label from VerticalPodAutoscalers: %w", err)
		}

		return nil
	}
}

// DeleteStaleShootAccessSecrets deletes stale `shoot-access-*` secrets of class `shoot` in the garden namespace for
// which a valid `garden-access-*` replacement secret (same ServiceAccount, class `garden-transitional`) exists.
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
		replacement.Labels[resourcesv1alpha1.ResourceManagerClass] == resourcesv1alpha1.ResourceManagerClassGardenTransitional &&
		replacement.Annotations[resourcesv1alpha1.ServiceAccountName] == stale.Annotations[resourcesv1alpha1.ServiceAccountName] &&
		replacement.Annotations[resourcesv1alpha1.ServiceAccountNamespace] == stale.Annotations[resourcesv1alpha1.ServiceAccountNamespace] &&
		metav1.HasAnnotation(replacement.ObjectMeta, resourcesv1alpha1.ServiceAccountTokenRenewTimestamp)
}

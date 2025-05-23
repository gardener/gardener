// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secretsrotation

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// CreateNewServiceAccountSecrets creates new secrets for all service accounts in the target cluster. This should only
// be executed in the 'Preparing' phase of the service account signing key rotation operation.
func CreateNewServiceAccountSecrets(ctx context.Context, log logr.Logger, c client.Client, secretsManager secretsmanager.Interface) error {
	serviceAccountKeySecret, found := secretsManager.Get(v1beta1constants.SecretNameServiceAccountKey, secretsmanager.Current)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameServiceAccountKey)
	}
	secretNameSuffix := utils.ComputeSecretChecksum(serviceAccountKeySecret.Data)[:6]

	serviceAccountList := &corev1.ServiceAccountList{}
	if err := c.List(ctx, serviceAccountList, client.MatchingLabelsSelector{
		Selector: labels.NewSelector().Add(
			utils.MustNewRequirement(labelKeyRotationKeyName, selection.NotEquals, serviceAccountKeySecret.Name),
		)},
	); err != nil {
		return err
	}

	log.Info("ServiceAccounts requiring a new token secret", "number", len(serviceAccountList.Items))

	var (
		limiter = rate.NewLimiter(rate.Limit(rotationQPS), rotationQPS)
		taskFns []flow.TaskFn
	)

	for _, obj := range serviceAccountList.Items {
		serviceAccount := obj

		taskFns = append(taskFns, func(ctx context.Context) error {
			if len(serviceAccount.Secrets) == 0 {
				return nil
			}

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        fmt.Sprintf("%s-token-%s", serviceAccount.Name, secretNameSuffix),
					Namespace:   serviceAccount.Namespace,
					Annotations: map[string]string{corev1.ServiceAccountNameKey: serviceAccount.Name},
				},
				Type: corev1.SecretTypeServiceAccountToken,
			}

			// If the ServiceAccount already references the secret then we have already created it and added it to the
			// list of secrets in a previous reconciliation. Consequently, we can exit early here since there is nothing
			// left to be done.
			for _, secretReference := range serviceAccount.Secrets {
				if secretReference.Name == secret.Name {
					return nil
				}
			}

			// Wait until we are allowed by the limiter to not overload the kube-apiserver with too many requests.
			if err := limiter.Wait(ctx); err != nil {
				return err
			}

			if err := c.Create(ctx, secret); client.IgnoreAlreadyExists(err) != nil {
				return fmt.Errorf("error creating new ServiceAccount secret: %w", err)
			}

			timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			return retry.Until(timeoutCtx, time.Second, func(ctx context.Context) (bool, error) {
				// Make sure we have the most recent version of the service account when we reach this point (which might
				// take a while given the above limiter.Wait call - in the meantime, the object might have been changed).
				if err := c.Get(ctx, client.ObjectKeyFromObject(&serviceAccount), &serviceAccount); err != nil {
					return retry.SevereError(err)
				}

				patch := client.MergeFromWithOptions(serviceAccount.DeepCopy(), client.MergeFromWithOptimisticLock{})
				metav1.SetMetaDataLabel(&serviceAccount.ObjectMeta, labelKeyRotationKeyName, serviceAccountKeySecret.Name)
				serviceAccount.Secrets = append([]corev1.ObjectReference{{Name: secret.Name}}, serviceAccount.Secrets...)

				if err := c.Patch(ctx, &serviceAccount, patch); err != nil {
					if apierrors.IsConflict(err) {
						return retry.MinorError(err)
					}
					return retry.SevereError(err)
				}

				return retry.Ok()
			})
		})
	}

	return flow.Parallel(taskFns...)(ctx)
}

// DeleteOldServiceAccountSecrets deletes old secrets for all service accounts in the target cluster. This should only
// be executed in the 'Completing' phase of the service account signing key rotation operation.
func DeleteOldServiceAccountSecrets(ctx context.Context, log logr.Logger, c client.Client, serviceAccountLastInitiationFinishedTime time.Time) error {
	serviceAccountList := &corev1.ServiceAccountList{}
	if err := c.List(ctx, serviceAccountList); err != nil {
		return err
	}

	log.Info("ServiceAccounts requiring the cleanup of old token secrets", "number", len(serviceAccountList.Items))

	var (
		limiter = rate.NewLimiter(rate.Limit(rotationQPS), rotationQPS)
		taskFns []flow.TaskFn
	)

	for _, obj := range serviceAccountList.Items {
		serviceAccount := obj

		taskFns = append(taskFns, func(ctx context.Context) error {
			// Wait until we are allowed by the limiter to not overload the kube-apiserver with too many requests.
			if err := limiter.Wait(ctx); err != nil {
				return err
			}

			var (
				secretsToDelete  []client.Object
				remainingSecrets []corev1.ObjectReference
			)

			// In the CreateNewServiceAccountSecrets function we add a new ServiceAccount secret as the first one to the
			// .secrets[] list in the ServiceAccount resource. However, when we reach this code now, the user could have
			// already removed this secret or changed the .secrets[] list. Hence, we now check which of the secrets in
			// the list have been created before the credentials rotation completion has been triggered. We only delete
			// those and keep the rest of the list untouched to not interfere with the user's operations.
			for _, secretReference := range serviceAccount.Secrets {
				secretMeta := &metav1.PartialObjectMetadata{}
				secretMeta.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Secret"))
				if err := c.Get(ctx, client.ObjectKey{Name: secretReference.Name, Namespace: serviceAccount.Namespace}, secretMeta); err != nil {
					if !apierrors.IsNotFound(err) {
						return err
					}
					// We don't care about secrets in the list which do not exist actually - it is the responsibility of the user to clean this up.
				} else if secretMeta.CreationTimestamp.UTC().Before(serviceAccountLastInitiationFinishedTime.UTC()) {
					secretsToDelete = append(secretsToDelete, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretMeta.Name, Namespace: secretMeta.Namespace}})
					continue
				}

				remainingSecrets = append(remainingSecrets, secretReference)
			}

			if len(secretsToDelete) == 0 {
				return nil
			}

			if err := kubernetesutils.DeleteObjects(ctx, c, secretsToDelete...); err != nil {
				return fmt.Errorf("error deleting old ServiceAccount secrets: %w", err)
			}

			timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			return retry.Until(timeoutCtx, time.Second, func(ctx context.Context) (bool, error) {
				// Make sure we have the most recent version of the service account when we reach this point (which might
				// take a while given the above limiter.Wait call - in the meantime, the object might have been changed).
				// Also, when deleting above secrets, kube-controller-manager might already remove them from the service
				// account which definitely changes the object.
				if err := c.Get(ctx, client.ObjectKeyFromObject(&serviceAccount), &serviceAccount); err != nil {
					return retry.SevereError(err)
				}

				patch := client.MergeFromWithOptions(serviceAccount.DeepCopy(), client.MergeFromWithOptimisticLock{})
				delete(serviceAccount.Labels, labelKeyRotationKeyName)
				serviceAccount.Secrets = remainingSecrets

				if err := c.Patch(ctx, &serviceAccount, patch); err != nil {
					if apierrors.IsConflict(err) {
						return retry.MinorError(err)
					}
					return retry.SevereError(err)
				}

				return retry.Ok()
			})
		})
	}

	return flow.Parallel(taskFns...)(ctx)
}

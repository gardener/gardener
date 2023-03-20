// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package secrets

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/time/rate"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	labelKeyRotationKeyName = "credentials.gardener.cloud/key-name"
	rotationQPS             = 100
)

// CreateNewServiceAccountSecrets creates new secrets for all service accounts in the target cluster. This should only
// be executed in the 'Preparing' phase of the service account signing key rotation operation.
func CreateNewServiceAccountSecrets(ctx context.Context, log logr.Logger, clientSet kubernetes.Interface, secretsManager secretsmanager.Interface) error {
	serviceAccountKeySecret, found := secretsManager.Get(v1beta1constants.SecretNameServiceAccountKey, secretsmanager.Current)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameServiceAccountKey)
	}
	secretNameSuffix := utils.ComputeSecretChecksum(serviceAccountKeySecret.Data)[:6]

	serviceAccountList := &corev1.ServiceAccountList{}
	if err := clientSet.Client().List(ctx, serviceAccountList, client.MatchingLabelsSelector{
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
		log := log.WithValues("serviceAccount", client.ObjectKeyFromObject(&serviceAccount))

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

			if err := clientSet.Client().Create(ctx, secret); client.IgnoreAlreadyExists(err) != nil {
				log.Error(err, "Error creating new ServiceAccount secret")
				return err
			}

			timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			return retry.Until(timeoutCtx, time.Second, func(ctx context.Context) (bool, error) {
				// Make sure we have the most recent version of the service account when we reach this point (which might
				// take a while given the above limiter.Wait call - in the meantime, the object might have been changed).
				if err := clientSet.Client().Get(ctx, client.ObjectKeyFromObject(&serviceAccount), &serviceAccount); err != nil {
					return retry.SevereError(err)
				}

				patch := client.MergeFromWithOptions(serviceAccount.DeepCopy(), client.MergeFromWithOptimisticLock{})
				metav1.SetMetaDataLabel(&serviceAccount.ObjectMeta, labelKeyRotationKeyName, serviceAccountKeySecret.Name)
				serviceAccount.Secrets = append([]corev1.ObjectReference{{Name: secret.Name}}, serviceAccount.Secrets...)

				if err := clientSet.Client().Patch(ctx, &serviceAccount, patch); err != nil {
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
func DeleteOldServiceAccountSecrets(ctx context.Context, log logr.Logger, clientSet kubernetes.Interface, serviceAccountLastInitiationFinishedTime time.Time) error {
	serviceAccountList := &corev1.ServiceAccountList{}
	if err := clientSet.Client().List(ctx, serviceAccountList); err != nil {
		return err
	}

	log.Info("ServiceAccounts requiring the cleanup of old token secrets", "number", len(serviceAccountList.Items))

	var (
		limiter = rate.NewLimiter(rate.Limit(rotationQPS), rotationQPS)
		taskFns []flow.TaskFn
	)

	for _, obj := range serviceAccountList.Items {
		serviceAccount := obj
		log := log.WithValues("serviceAccount", client.ObjectKeyFromObject(&serviceAccount))

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
				if err := clientSet.Client().Get(ctx, client.ObjectKey{Name: secretReference.Name, Namespace: serviceAccount.Namespace}, secretMeta); err != nil {
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

			if err := kubernetesutils.DeleteObjects(ctx, clientSet.Client(), secretsToDelete...); err != nil {
				log.Error(err, "Error deleting old ServiceAccount secrets")
				return err
			}

			timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			return retry.Until(timeoutCtx, time.Second, func(ctx context.Context) (bool, error) {
				// Make sure we have the most recent version of the service account when we reach this point (which might
				// take a while given the above limiter.Wait call - in the meantime, the object might have been changed).
				// Also, when deleting above secrets, kube-controller-manager might already remove them from the service
				// account which definitely changes the object.
				if err := clientSet.Client().Get(ctx, client.ObjectKeyFromObject(&serviceAccount), &serviceAccount); err != nil {
					return retry.SevereError(err)
				}

				patch := client.MergeFromWithOptions(serviceAccount.DeepCopy(), client.MergeFromWithOptimisticLock{})
				delete(serviceAccount.Labels, labelKeyRotationKeyName)
				serviceAccount.Secrets = remainingSecrets

				if err := clientSet.Client().Patch(ctx, &serviceAccount, patch); err != nil {
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

// RewriteSecretsAddLabel patches all secrets in all namespaces in the target clusters and adds a label whose value is
// the name of the current ETCD encryption key secret. This function is useful for the ETCD encryption key secret
// rotation which requires all secrets to be rewritten to ETCD so that they become encrypted with the new key.
// After it's done, it snapshots ETCD so that we can restore backups in case we lose the cluster before the next
// incremental snapshot is taken.
func RewriteSecretsAddLabel(ctx context.Context, log logr.Logger, clientSet kubernetes.Interface, secretsManager secretsmanager.Interface) error {
	etcdEncryptionKeySecret, found := secretsManager.Get(v1beta1constants.SecretNameETCDEncryptionKey, secretsmanager.Current)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameETCDEncryptionKey)
	}

	return rewriteSecrets(
		ctx,
		log,
		clientSet,
		utils.MustNewRequirement(labelKeyRotationKeyName, selection.NotEquals, etcdEncryptionKeySecret.Name),
		func(objectMeta *metav1.ObjectMeta) {
			metav1.SetMetaDataLabel(objectMeta, labelKeyRotationKeyName, etcdEncryptionKeySecret.Name)
		},
	)
}

// SnapshotETCDAfterRewritingSecrets performs a full snapshot on ETCD after the secrets got rewritten as part of the
// ETCD encryption secret rotation. It adds an annotation to the kube-apiserver deployment after it's done so that it
// does not take another snapshot again after it succeeded once.
func SnapshotETCDAfterRewritingSecrets(ctx context.Context, runtimeClientSet kubernetes.Interface, snapshotEtcd func(ctx context.Context) error, kubeAPIServerNamespace string) error {
	// Check if we have to snapshot ETCD now that we have rewritten all secrets.
	meta := &metav1.PartialObjectMetadata{}
	meta.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))
	if err := runtimeClientSet.Client().Get(ctx, kubernetesutils.Key(kubeAPIServerNamespace, v1beta1constants.DeploymentNameKubeAPIServer), meta); err != nil {
		return err
	}

	if metav1.HasAnnotation(meta.ObjectMeta, common.AnnotationKeyEtcdSnapshotted) {
		return nil
	}

	if err := snapshotEtcd(ctx); err != nil {
		return err
	}

	// If we have hit this point then we have snapshotted ETCD successfully. Now we can mark this step as "completed"
	// (via an annotation) so that we do not trigger a snapshot again in a future reconciliation in case the current one
	// fails after this step.
	return common.PatchKubeAPIServerDeploymentMeta(ctx, runtimeClientSet, kubeAPIServerNamespace, func(meta *metav1.PartialObjectMetadata) {
		metav1.SetMetaDataAnnotation(&meta.ObjectMeta, common.AnnotationKeyEtcdSnapshotted, "true")
	})
}

// RewriteSecretsRemoveLabel patches all secrets in all namespaces in the target clusters and removes the label whose
// value is the name of the current ETCD encryption key secret. This function is useful for the ETCD encryption key
// secret rotation which requires all secrets to be rewritten to ETCD so that they become encrypted with the new key.
func RewriteSecretsRemoveLabel(ctx context.Context, log logr.Logger, runtimeClientSet, targetClientSet kubernetes.Interface, kubeAPIServerNamespace string) error {
	if err := rewriteSecrets(
		ctx,
		log,
		targetClientSet,
		utils.MustNewRequirement(labelKeyRotationKeyName, selection.Exists),
		func(objectMeta *metav1.ObjectMeta) {
			delete(objectMeta.Labels, labelKeyRotationKeyName)
		},
	); err != nil {
		return err
	}

	return common.PatchKubeAPIServerDeploymentMeta(ctx, runtimeClientSet, kubeAPIServerNamespace, func(meta *metav1.PartialObjectMetadata) {
		delete(meta.Annotations, common.AnnotationKeyEtcdSnapshotted)
	})
}

func rewriteSecrets(ctx context.Context, log logr.Logger, clientSet kubernetes.Interface, requirement labels.Requirement, mutateObjectMeta func(*metav1.ObjectMeta)) error {
	secretList := &metav1.PartialObjectMetadataList{}
	secretList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("SecretList"))
	if err := clientSet.Client().List(ctx, secretList, client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(requirement)}); err != nil {
		return err
	}

	log.Info("Secrets requiring to be rewritten after ETCD encryption key rotation", "number", len(secretList.Items))

	var (
		limiter = rate.NewLimiter(rate.Limit(rotationQPS), rotationQPS)
		taskFns []flow.TaskFn
	)

	for _, obj := range secretList.Items {
		secret := obj

		taskFns = append(taskFns, func(ctx context.Context) error {
			patch := client.StrategicMergeFrom(secret.DeepCopy())
			mutateObjectMeta(&secret.ObjectMeta)

			// Wait until we are allowed by the limiter to not overload the kube-apiserver with too many requests.
			if err := limiter.Wait(ctx); err != nil {
				return err
			}

			return clientSet.Client().Patch(ctx, &secret, patch)
		})
	}

	return flow.Parallel(taskFns...)(ctx)
}

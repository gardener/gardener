// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package genericactuator

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/extensions/pkg/controller/backupentry"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	securityv1alpha1constants "github.com/gardener/gardener/pkg/apis/security/v1alpha1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

type actuator struct {
	backupEntryDelegate BackupEntryDelegate
	client              client.Client
}

// Ensure actuator implements backupentry.Actuator interface
var _ backupentry.Actuator = (*actuator)(nil)

// AnnotationKeyCreatedByBackupEntry is a constant for the key of an annotation on the etcd-backup Secret whose value contains
// the name of the BackupEntry object that created the Secret.
const AnnotationKeyCreatedByBackupEntry = "backup.gardener.cloud/created-by"

// NewActuator creates a new Actuator that updates the status of the handled BackupEntry resources.
func NewActuator(mgr manager.Manager, backupEntryDelegate BackupEntryDelegate) backupentry.Actuator {
	return &actuator{
		backupEntryDelegate: backupEntryDelegate,
		client:              mgr.GetClient(),
	}
}

// Reconcile reconciles the update of a BackupEntry.
func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, be *extensionsv1alpha1.BackupEntry) error {
	return a.deployEtcdBackupSecret(ctx, log, be)
}

func (a *actuator) deployEtcdBackupSecret(ctx context.Context, log logr.Logger, be *extensionsv1alpha1.BackupEntry) error {
	shootTechnicalID, _ := backupentry.ExtractShootDetailsFromBackupEntryName(be.Name)

	namespace := &corev1.Namespace{}
	if err := a.client.Get(ctx, client.ObjectKey{Name: shootTechnicalID}, namespace); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("SeedNamespace for shoot not found. Avoiding etcd backup secret deployment")
			return nil
		}
		return fmt.Errorf("failed to get seed namespace: %w", err)
	}
	if namespace.DeletionTimestamp != nil {
		log.Info("SeedNamespace for shoot is being terminated. Avoiding etcd backup secret deployment")
		return nil
	}

	backupEntrySecret, err := kubernetesutils.GetSecretByReference(ctx, a.client, &be.Spec.SecretRef)
	if err != nil {
		log.Error(err, "Failed to read backup extension secret")
		return err
	}

	var (
		backupEntrySecretData = backupEntrySecret.DeepCopy().Data
		withWorkloadIdentity  = backupEntrySecret.Labels[securityv1alpha1constants.LabelPurpose] == securityv1alpha1constants.LabelPurposeWorkloadIdentityTokenRequestor
	)

	if withWorkloadIdentity {
		// When WorkloadIdentity is used as credentials, the only standard data keys are `config` and `token`.
		// Any other data key shall be set via (admission) controller, also the `token` value will be set by the
		// token-requestor controller, therefore we are not copying from the backupEntry secret.
		maps.DeleteFunc(backupEntrySecretData, func(k string, _ []byte) bool {
			return k != securityv1alpha1constants.DataKeyConfig
		})
	}

	if backupEntrySecretData == nil {
		backupEntrySecretData = map[string][]byte{}
	}
	backupEntrySecretData[v1beta1constants.DataKeyBackupBucketName] = []byte(be.Spec.BucketName)
	etcdSecretData, err := a.backupEntryDelegate.GetETCDSecretData(ctx, log, be, backupEntrySecretData)
	if err != nil {
		return err
	}

	etcdSecret := emptyEtcdBackupSecret(be.Name)

	_, err = controllerutils.GetAndCreateOrMergePatch(ctx, a.client, etcdSecret, func() error {
		if withWorkloadIdentity {
			// Preserve only the workload identity token renew timestamp annotation and reset all others.
			maps.DeleteFunc(etcdSecret.Annotations, func(k, _ string) bool {
				return k != securityv1alpha1constants.AnnotationWorkloadIdentityTokenRenewTimestamp
			})

			if workloadIdentityName, ok := backupEntrySecret.Annotations[securityv1alpha1constants.AnnotationWorkloadIdentityName]; !ok {
				return fmt.Errorf("BackupEntry is set to use workload identity but WorkloadIdentity's name is missing")
			} else {
				metav1.SetMetaDataAnnotation(&etcdSecret.ObjectMeta, securityv1alpha1constants.AnnotationWorkloadIdentityName, workloadIdentityName)
			}

			if workloadIdentityNamespace, ok := backupEntrySecret.Annotations[securityv1alpha1constants.AnnotationWorkloadIdentityNamespace]; !ok {
				return fmt.Errorf("BackupEntry is set to use workload identity but WorkloadIdentity's namespace is missing")
			} else {
				metav1.SetMetaDataAnnotation(&etcdSecret.ObjectMeta, securityv1alpha1constants.AnnotationWorkloadIdentityNamespace, workloadIdentityNamespace)
			}

			if contextObj, ok := backupEntrySecret.Annotations[securityv1alpha1constants.AnnotationWorkloadIdentityContextObject]; ok {
				metav1.SetMetaDataAnnotation(&etcdSecret.ObjectMeta, securityv1alpha1constants.AnnotationWorkloadIdentityContextObject, contextObj)
			}

			// Preserve only the workload identity token purpose label and reset all others.
			etcdSecret.Labels = map[string]string{securityv1alpha1constants.LabelPurpose: securityv1alpha1constants.LabelPurposeWorkloadIdentityTokenRequestor}

			if provider, ok := backupEntrySecret.Labels[securityv1alpha1constants.LabelWorkloadIdentityProvider]; !ok {
				return fmt.Errorf("BackupEntry is set to use workload identity but WorkloadIdentity's provider type missing")
			} else {
				etcdSecret.Labels[securityv1alpha1constants.LabelWorkloadIdentityProvider] = provider
			}

			if token, ok := etcdSecret.Data[securityv1alpha1constants.DataKeyToken]; ok {
				etcdSecretData[securityv1alpha1constants.DataKeyToken] = token
			}
		}

		metav1.SetMetaDataAnnotation(&etcdSecret.ObjectMeta, AnnotationKeyCreatedByBackupEntry, be.Name)
		etcdSecret.Data = etcdSecretData
		return nil
	},
		// The token-requestor might concurrently update the secret token key to populate the token.
		// Hence, we need to use optimistic locking here to ensure we don't accidentally overwrite the concurrent update.
		// ref https://github.com/gardener/gardener/issues/6092#issuecomment-1156244514
		controllerutils.MergeFromOption{MergeFromOption: client.MergeFromWithOptimisticLock{}},
	)
	return err
}

// Delete deletes the BackupEntry.
func (a *actuator) Delete(ctx context.Context, log logr.Logger, be *extensionsv1alpha1.BackupEntry) error {
	if err := a.deleteEtcdBackupSecret(ctx, log, be.Name); err != nil {
		return err
	}
	return a.backupEntryDelegate.Delete(ctx, log, be)
}

func (a *actuator) deleteEtcdBackupSecret(ctx context.Context, log logr.Logger, backupEntryName string) error {
	etcdSecret := emptyEtcdBackupSecret(backupEntryName)
	if err := a.client.Get(ctx, client.ObjectKeyFromObject(etcdSecret), etcdSecret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("failed to get secret %s: %w", client.ObjectKeyFromObject(etcdSecret), err)
	}

	if createdBy, ok := etcdSecret.Annotations[AnnotationKeyCreatedByBackupEntry]; ok && createdBy != backupEntryName {
		log.Info("Skipping etcd-backup Secret deletion because it was not created by the currently deleting BackupEntry", "createdBy", createdBy)
		return nil
	}

	log.Info("Deleting etcd-backup Secret")
	return kubernetesutils.DeleteObject(ctx, a.client, etcdSecret)
}

func emptyEtcdBackupSecret(backupEntryName string) *corev1.Secret {
	secretName := v1beta1constants.BackupSecretName
	if strings.HasPrefix(backupEntryName, v1beta1constants.BackupSourcePrefix) {
		secretName = fmt.Sprintf("%s-%s", v1beta1constants.BackupSourcePrefix, v1beta1constants.BackupSecretName)
	}
	shootTechnicalID, _ := backupentry.ExtractShootDetailsFromBackupEntryName(backupEntryName)

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: shootTechnicalID,
		},
	}
}

// Restore restores the BackupEntry.
func (a *actuator) Restore(ctx context.Context, log logr.Logger, be *extensionsv1alpha1.BackupEntry) error {
	return a.Reconcile(ctx, log, be)
}

// Migrate migrates the BackupEntry.
func (a *actuator) Migrate(_ context.Context, _ logr.Logger, _ *extensionsv1alpha1.BackupEntry) error {
	return nil
}

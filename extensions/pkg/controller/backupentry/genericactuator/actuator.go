// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package genericactuator

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

	"github.com/gardener/gardener/extensions/pkg/controller/backupentry"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

type actuator struct {
	backupEntryDelegate BackupEntryDelegate
	client              client.Client
}

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
		log.Error(err, "Failed to get seed namespace")
		return err
	}
	if namespace.DeletionTimestamp != nil {
		log.Info("SeedNamespace for shoot is being terminated. Avoiding etcd backup secret deployment")
		return nil
	}

	backupSecret, err := kubernetesutils.GetSecretByReference(ctx, a.client, &be.Spec.SecretRef)
	if err != nil {
		log.Error(err, "Failed to read backup extension secret")
		return err
	}

	backupSecretData := backupSecret.DeepCopy().Data
	if backupSecretData == nil {
		backupSecretData = map[string][]byte{}
	}
	backupSecretData[v1beta1constants.DataKeyBackupBucketName] = []byte(be.Spec.BucketName)
	etcdSecretData, err := a.backupEntryDelegate.GetETCDSecretData(ctx, log, be, backupSecretData)
	if err != nil {
		return err
	}

	etcdSecret := emptyEtcdBackupSecret(be.Name)

	_, err = controllerutils.GetAndCreateOrMergePatch(ctx, a.client, etcdSecret, func() error {
		metav1.SetMetaDataAnnotation(&etcdSecret.ObjectMeta, AnnotationKeyCreatedByBackupEntry, be.Name)
		etcdSecret.Data = etcdSecretData
		return nil
	})
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

// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/operation/common"
	encryptionconfiguration "github.com/gardener/gardener/pkg/operation/etcdencryption"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// ApplyEncryptionConfiguration creates or updates a secret on the Seed
// which contains the encryption configuration that is necessary to encrypt the
// Kubernetes secrets in etcd.
//
// To mitigate data loss to a certain degree, the secret is also synced to the Garden cluster.
func (b *Botanist) ApplyEncryptionConfiguration(ctx context.Context) error {
	if err := b.syncEncryptionConfigurationFromGardenCluster(ctx); err != nil {
		return err
	}
	secret, err := b.createOrUpdateEncryptionConfiguration(ctx)
	if err != nil {
		return err
	}

	return b.syncEncryptionConfigurationToGarden(ctx, secret)
}

func (b *Botanist) syncEncryptionConfigurationFromGardenCluster(ctx context.Context) error {
	gardenSecret := &corev1.Secret{}
	if err := b.K8sGardenClient.Client().Get(ctx, common.GardenEtcdEncryptionSecretKey(b.Shoot.Info.Namespace, b.Shoot.Info.Name), gardenSecret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	// Create the secret in the Shoot's seed namespace from the garden secret only if it doesn't exist to prevent overwriting.
	secret := encryptionSecretFromGardenSecret(gardenSecret, b.Shoot.SeedNamespace)
	if err := b.K8sSeedClient.Client().Create(ctx, secret); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func (b *Botanist) createOrUpdateEncryptionConfiguration(ctx context.Context) (*corev1.Secret, error) {
	var (
		secret = &corev1.Secret{ObjectMeta: kutil.ObjectMeta(b.Shoot.SeedNamespace, common.EtcdEncryptionSecretName)}
		conf   *apiserverconfigv1.EncryptionConfiguration
	)

	_, err := controllerutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), secret, func() error {
		var err error
		conf, err = encryptionconfiguration.ReadSecret(secret)
		if err != nil {
			if !encryptionconfiguration.IsConfigurationNotFoundError(err) {
				return err
			}

			b.Logger.Info("Creating new etcd encryption configuration for Shoot")
			conf, err = encryptionconfiguration.NewPassiveConfiguration(time.Now(), rand.Reader)
			if err != nil {
				return err
			}
		}

		// When firstly created, the encryption configuration secret does not have a checksum annotation yet. This annotation will
		// only be added after all shoot secrets have been rewritten. In order to allow a smooth transition from un-encrypted to encrypted
		// etcd data we first make the configuration inactive, i.e., put the `identity` provider as first list in the entry. In the next
		// reconciliation we will detect that the annotation is set and then we can make it active, i.e., moving the `identity` provider
		// to the second list item. The reason for this is that new API servers would otherwise start with an active configuration and would
		// try to decrypt secrets in the etcd store (which would fail because they are not yet encrypted). Be aware that this will only happen
		// once during the first introduction of the encryption configuration.
		firstCreationOfEncryptionConfiguration := !metav1.HasAnnotation(secret.ObjectMeta, common.EtcdEncryptionChecksumAnnotationName)

		// We allow to force the API servers to not encrypt the secrets in etcd store. This is possible by annotating the etcd-encryption-secret
		// with 'shoot.gardener.cloud/etcd-encryption-force-plaintext-secrets=true'.
		forcePlaintextSecrets := kutil.HasMetaDataAnnotation(secret, common.EtcdEncryptionForcePlaintextAnnotationName, "true")

		encrypt := !firstCreationOfEncryptionConfiguration && !forcePlaintextSecrets
		b.Logger.Infof("Setting encryption of %s to %t", common.EtcdEncryptionEncryptedResourceSecrets, encrypt)
		if err := encryptionconfiguration.SetResourceEncryption(conf, common.EtcdEncryptionEncryptedResourceSecrets, encrypt); err != nil {
			return err
		}

		checksum, err := confChecksum(conf)
		if err != nil {
			return err
		}

		func() {
			b.mutex.Lock()
			defer b.mutex.Unlock()
			b.CheckSums[common.EtcdEncryptionSecretName] = checksum
		}()

		return encryptionconfiguration.UpdateSecret(secret, conf)
	})
	if err != nil {
		return nil, err
	}

	return secret, err
}

func (b *Botanist) syncEncryptionConfigurationToGarden(ctx context.Context, controlPlaneSecret *corev1.Secret) error {
	gardenSecret := &corev1.Secret{ObjectMeta: kutil.ObjectMetaFromKey(common.GardenEtcdEncryptionSecretKey(b.Shoot.Info.Namespace, b.Shoot.Info.Name))}
	_, err := controllerutil.CreateOrUpdate(ctx, b.K8sGardenClient.Client(), gardenSecret, func() error {
		gardenSecret.OwnerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(b.Shoot.Info, gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot")),
		}
		if forcePlainTextSecrets, ok := controlPlaneSecret.Annotations[common.EtcdEncryptionForcePlaintextAnnotationName]; ok {
			kutil.SetMetaDataAnnotation(gardenSecret, common.EtcdEncryptionForcePlaintextAnnotationName, forcePlainTextSecrets)
		}
		gardenSecret.Data = controlPlaneSecret.Data
		return nil
	})
	return err
}

func confChecksum(conf *apiserverconfigv1.EncryptionConfiguration) (string, error) {
	data, err := encryptionconfiguration.Write(conf)
	if err != nil {
		return "", err
	}

	return utils.ComputeSHA256Hex(data), nil
}

// RewriteShootSecretsIfEncryptionConfigurationChanged rewrites the secrets in the Shoot if the etcd
// encryption configuration changed. Rewriting here means that a patch request is sent that forces
// the etcd to encrypt them with the new configuration.
func (b *Botanist) RewriteShootSecretsIfEncryptionConfigurationChanged(ctx context.Context) error {
	checksum := func() string {
		b.mutex.RLock()
		defer b.mutex.RUnlock()
		return b.CheckSums[common.EtcdEncryptionSecretName]
	}()

	if err := b.rewriteShootSecretsIfChecksumChanged(ctx, checksum); err != nil {
		return err
	}

	gardenSecret := &corev1.Secret{ObjectMeta: kutil.ObjectMetaFromKey(common.GardenEtcdEncryptionSecretKey(b.Shoot.Info.Namespace, b.Shoot.Info.Name))}
	_, err := controllerutil.CreateOrUpdate(ctx, b.K8sGardenClient.Client(), gardenSecret, func() error {
		kutil.SetMetaDataAnnotation(gardenSecret, common.EtcdEncryptionChecksumAnnotationName, checksum)
		return nil
	})
	return err
}

func (b *Botanist) rewriteShootSecretsIfChecksumChanged(ctx context.Context, checksum string) error {
	secret := &corev1.Secret{}
	if err := b.K8sSeedClient.Client().Get(ctx, kutil.Key(b.Shoot.SeedNamespace, common.EtcdEncryptionSecretName), secret); err != nil {
		return err
	}

	// If the etcd encryption secret in the seed already has the correct checksum annotation then we don't have to do anything.
	if secret.Annotations[common.EtcdEncryptionChecksumAnnotationName] == checksum {
		b.Logger.Infof("etcd encryption is up to date (checksum %s), no need to rewrite secrets", checksum)
		return nil
	}

	shortChecksum := kutil.TruncateLabelValue(checksum)

	// Add checksum label to all secrets in shoot so that they get rewritten now, and also so that we don't rewrite them again in
	// case this function fails for some reason.
	notCurrentChecksum, err := labels.NewRequirement(common.EtcdEncryptionChecksumLabelName, selection.NotEquals, []string{shortChecksum})
	if err != nil {
		return err
	}
	if errorList := b.updateShootLabelsForEtcdEncryption(ctx, notCurrentChecksum, func(m metav1.Object) {
		kutil.SetMetaDataLabel(m, common.EtcdEncryptionChecksumLabelName, shortChecksum)
	}); len(errorList) > 0 {
		return fmt.Errorf("could not add checksum label for all shoot secrets: %+v", errorList)
	}
	b.Logger.Info("Successfully updated all secrets in the shoot after etcd encryption config changed")

	// Remove checksum label from all secrets in shoot again.
	hasChecksumLabelKey, err := labels.NewRequirement(common.EtcdEncryptionChecksumLabelName, selection.Exists, nil)
	if err != nil {
		return err
	}
	if errorList := b.updateShootLabelsForEtcdEncryption(ctx, hasChecksumLabelKey, func(m metav1.Object) {
		delete(m.GetLabels(), common.EtcdEncryptionChecksumLabelName)
	}); len(errorList) > 0 {
		return fmt.Errorf("could not remove checksum label from all shoot secrets: %+v", errorList)
	}
	b.Logger.Info("Successfully removed all added secret labels in the shoot after etcd encryption config changed")

	// Update etcd encryption secret in seed to have the correct checksum annotation.
	oldSecret := secret.DeepCopy()
	kutil.SetMetaDataAnnotation(secret, common.EtcdEncryptionChecksumAnnotationName, checksum)
	return b.K8sSeedClient.Client().Patch(ctx, secret, client.MergeFrom(oldSecret))
}

func (b *Botanist) updateShootLabelsForEtcdEncryption(ctx context.Context, labelRequirement *labels.Requirement, mutateLabelsFunc func(m metav1.Object)) []error {
	secretList := &corev1.SecretList{}
	if err := b.K8sShootClient.Client().List(ctx, secretList, client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(*labelRequirement)}); err != nil {
		return []error{err}
	}

	var errorList []error
	for _, s := range secretList.Items {
		secretCopy := s.DeepCopy()
		mutateLabelsFunc(&s)
		patch := client.MergeFrom(secretCopy)

		if err := b.K8sShootClient.Client().Patch(ctx, &s, patch); err != nil {
			errorList = append(errorList, err)
		}
	}

	return errorList
}

func encryptionSecretFromGardenSecret(gardenSecret *corev1.Secret, seedNamespace string) *corev1.Secret {
	secret := &corev1.Secret{ObjectMeta: kutil.ObjectMeta(seedNamespace, common.EtcdEncryptionSecretName)}
	if gardenSecret.Data != nil {
		secret.Data = make(map[string][]byte)
		secret.Data[common.EtcdEncryptionSecretFileName] = gardenSecret.Data[common.EtcdEncryptionSecretFileName]
	}
	if checksum, ok := gardenSecret.Annotations[common.EtcdEncryptionChecksumAnnotationName]; ok {
		kutil.SetMetaDataAnnotation(secret, common.EtcdEncryptionChecksumAnnotationName, checksum)
	}
	if forcePlainTextSecrets, ok := gardenSecret.Annotations[common.EtcdEncryptionForcePlaintextAnnotationName]; ok {
		kutil.SetMetaDataAnnotation(secret, common.EtcdEncryptionForcePlaintextAnnotationName, forcePlainTextSecrets)
	}
	return secret
}

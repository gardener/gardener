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
	"errors"
	"fmt"
	"reflect"

	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/etcdencryption"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/infodata"
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

// GenerateEncryptionConfiguration generates new encryption configuration data or syncs it from the etcd encryption configuration secret if it already exists.
func (b *Botanist) GenerateEncryptionConfiguration(ctx context.Context) error {
	secret := &corev1.Secret{}
	if err := b.K8sSeedClient.Client().Get(ctx, kutil.Key(b.Shoot.SeedNamespace, common.EtcdEncryptionSecretName), secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		secret = nil
	}

	if b.Shoot.ETCDEncryption == nil {
		var err error
		b.Shoot.ETCDEncryption, err = generateETCDEncryption(secret)
		if err != nil {
			return err
		}
	}

	if secret != nil {
		forcePlainTextSecrets := kutil.HasMetaDataAnnotation(secret, common.EtcdEncryptionForcePlaintextAnnotationName, "true")
		b.Shoot.ETCDEncryption.SetForcePlainTextResources(forcePlainTextSecrets)
	}

	return nil
}

// PersistEncryptionConfiguration adds the encryption configuration to the ShootState.
func (b *Botanist) PersistEncryptionConfiguration(ctx context.Context) error {
	return b.persistEncryptionConfigInShootState(ctx)
}

// ApplyEncryptionConfiguration creates or updates a secret on the Seed
// which contains the encryption configuration that is necessary to encrypt the
// Kubernetes secrets in etcd.
func (b *Botanist) ApplyEncryptionConfiguration(ctx context.Context) error {
	var (
		secret = &corev1.Secret{ObjectMeta: kutil.ObjectMeta(b.Shoot.SeedNamespace, common.EtcdEncryptionSecretName)}
		conf   *apiserverconfigv1.EncryptionConfiguration
	)
	if b.Shoot.ETCDEncryption == nil {
		return errors.New("Could not find etcd encryption configuration in ShootState")
	}

	conf = etcdencryption.NewEncryptionConfiguration(b.Shoot.ETCDEncryption)
	_, err := controllerutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), secret, func() error {
		if b.Shoot.ETCDEncryption.ForcePlainTextResources {
			kutil.SetMetaDataAnnotation(secret, common.EtcdEncryptionForcePlaintextAnnotationName, "true")
		}
		return etcdencryption.UpdateSecret(secret, conf)
	})
	if err != nil {
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

	return nil
}

// RemoveOldETCDEncryptionSecretFromGardener removes the etcd encryption configuration secret from the Shoot's namespace in the garden cluster as it is no longer necessary.
// This step can be removed in the future after all secrets have been cleaned up.
func (b *Botanist) RemoveOldETCDEncryptionSecretFromGardener(ctx context.Context) error {
	etcdSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gutil.GardenEtcdEncryptionSecretName(b.Shoot.Info.Name),
			Namespace: b.Shoot.Info.Namespace,
		},
	}
	return client.IgnoreNotFound(b.K8sGardenClient.Client().Delete(ctx, etcdSecret))
}

func confChecksum(conf *apiserverconfigv1.EncryptionConfiguration) (string, error) {
	data, err := etcdencryption.Write(conf)
	if err != nil {
		return "", err
	}

	return utils.ComputeSHA256Hex(data), nil
}

// RewriteShootSecretsIfEncryptionConfigurationChanged rewrites the secrets in the Shoot if the etcd
// encryption configuration changed. Rewriting here means that a patch request is sent that forces
// the etcd to encrypt them with the new configuration.
func (b *Botanist) RewriteShootSecretsIfEncryptionConfigurationChanged(ctx context.Context) error {
	if !b.Shoot.ETCDEncryption.RewriteResources {
		return nil
	}

	checksum := func() string {
		b.mutex.RLock()
		defer b.mutex.RUnlock()
		return b.CheckSums[common.EtcdEncryptionSecretName]
	}()
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

	b.Shoot.ETCDEncryption.RewriteResources = false
	return b.persistEncryptionConfigInShootState(ctx)
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

func (b *Botanist) persistEncryptionConfigInShootState(ctx context.Context) error {
	gardenerResourceList := gardencorev1alpha1helper.GardenerResourceDataList(b.ShootState.Spec.Gardener)

	oldETCDEncryptionConfig, err := etcdencryption.GetEncryptionConfig(gardenerResourceList)
	if err != nil {
		return err
	}
	if reflect.DeepEqual(oldETCDEncryptionConfig, b.Shoot.ETCDEncryption) {
		return nil
	}

	if err := infodata.UpsertInfoData(&gardenerResourceList, common.ETCDEncryptionConfigDataName, b.Shoot.ETCDEncryption); err != nil {
		return err
	}

	return b.SaveGardenerResourcesInShootState(ctx, gardenerResourceList)
}

func generateETCDEncryption(secret *corev1.Secret) (*etcdencryption.EncryptionConfig, error) {
	encryptionConfig := &etcdencryption.EncryptionConfig{}
	if secret != nil {
		if err := encryptionConfig.AddEncryptionKeyFromSecret(secret); err != nil {
			return nil, err
		}
		return encryptionConfig, nil
	}

	if err := encryptionConfig.AddNewEncryptionKey(); err != nil {
		return nil, err
	}
	return encryptionConfig, nil
}

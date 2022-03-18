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

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/etcdencryption"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/infodata"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
)

// GenerateEncryptionConfiguration generates new encryption configuration data or syncs it from the etcd encryption configuration secret if it already exists.
func (b *Botanist) GenerateEncryptionConfiguration(ctx context.Context) error {
	secret := &corev1.Secret{}
	if err := b.K8sSeedClient.Client().Get(ctx, kutil.Key(b.Shoot.SeedNamespace, kubeapiserver.SecretNameEtcdEncryption), secret); err != nil {
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
		secret = &corev1.Secret{ObjectMeta: kutil.ObjectMeta(b.Shoot.SeedNamespace, kubeapiserver.SecretNameEtcdEncryption)}
		conf   *apiserverconfigv1.EncryptionConfiguration
	)
	if b.Shoot.ETCDEncryption == nil {
		return errors.New("could not find etcd encryption configuration in ShootState")
	}

	conf = etcdencryption.NewEncryptionConfiguration(b.Shoot.ETCDEncryption)
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, b.K8sSeedClient.Client(), secret, func() error {
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

	b.StoreCheckSum(kubeapiserver.SecretNameEtcdEncryption, checksum)

	return nil
}

func confChecksum(conf *apiserverconfigv1.EncryptionConfiguration) (string, error) {
	data, err := etcdencryption.Write(conf)
	if err != nil {
		return "", err
	}

	return utils.ComputeSHA256Hex(data), nil
}

func (b *Botanist) persistEncryptionConfigInShootState(ctx context.Context) error {
	return b.SaveGardenerResourceDataInShootState(ctx, func(gardenerResourceData *[]gardencorev1alpha1.GardenerResourceData) error {
		gardenerResourceList := gardencorev1alpha1helper.GardenerResourceDataList(*gardenerResourceData)
		if err := infodata.UpsertInfoData(&gardenerResourceList, common.ETCDEncryptionConfigDataName, b.Shoot.ETCDEncryption); err != nil {
			return err
		}

		*gardenerResourceData = gardenerResourceList
		return nil
	})
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

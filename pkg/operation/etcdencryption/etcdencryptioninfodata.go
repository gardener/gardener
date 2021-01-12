/*
 * Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *       http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 *
 */

package etcdencryption

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"

	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/infodata"
	corev1 "k8s.io/api/core/v1"
)

// EncryptionKey holds the key and its name used to encrypt resources in ETCD.
type EncryptionKey struct {
	Key  string
	Name string
}

// EncryptionConfig holds a list of keys and information whether resources should be forcefully persisted in plain text and rewritten if the configuration changes.
type EncryptionConfig struct {
	EncryptionKeys          []EncryptionKey
	ForcePlainTextResources bool
	RewriteResources        bool
}

// TypeVersion implements InfoData
func (e *EncryptionConfig) TypeVersion() infodata.TypeVersion {
	return ETCDEncryptionDataType
}

// Marshal ETCDEncryption InfoData
func (e *EncryptionConfig) Marshal() ([]byte, error) {
	encryptionKeysData := make([]EncryptionKeyData, len(e.EncryptionKeys))
	for i, encryptionKey := range e.EncryptionKeys {
		encryptionKeysData[i].Key = encryptionKey.Key
		encryptionKeysData[i].Name = encryptionKey.Name
	}
	return json.Marshal(&EncryptionConfigData{EncryptionKeys: encryptionKeysData, ForcePlainTextResources: e.ForcePlainTextResources, RewriteResources: e.RewriteResources})
}

// NewEncryptionConfig creates a new ETCDEncryptionKey from a given key and name
func NewEncryptionConfig(keys []EncryptionKey, forcePlainTextResources, rewriteResources bool) (*EncryptionConfig, error) {
	return &EncryptionConfig{keys, forcePlainTextResources, rewriteResources}, nil
}

// AddEncryptionKeyFromSecret gets the active etcd encryption key from the secret object and adds it to the ETCDEncryptionConfig.
// TODO: this function can be removed in a future version when all the encryption configurations have been synced to the ShootState.
func (e *EncryptionConfig) AddEncryptionKeyFromSecret(secret *corev1.Secret) error {
	conf, err := ReadSecret(secret)
	if err != nil {
		return err
	}
	name, key, err := GetSecretKeyForResources(conf, common.EtcdEncryptionEncryptedResourceSecrets)
	if err != nil {
		return err
	}

	etcdKey := EncryptionKey{
		Key:  key,
		Name: name,
	}
	e.EncryptionKeys = append(e.EncryptionKeys, etcdKey)
	return nil
}

// AddNewEncryptionKey generates a new etcd encryption key and adds it to the ETCDEncryptionConfig.
func (e *EncryptionConfig) AddNewEncryptionKey() error {
	key, err := NewEncryptionKey(time.Now(), rand.Reader)
	if err != nil {
		return err
	}
	etcdKey := EncryptionKey{
		Key:  key.Secret,
		Name: key.Name,
	}
	e.EncryptionKeys = append(e.EncryptionKeys, etcdKey)
	return nil
}

// SetForcePlainTextResources sets whether resources should be encrypted or not.
// If the configuration changes RewriteResource is set to true.
func (e *EncryptionConfig) SetForcePlainTextResources(forcePlainTextResources bool) {
	if e.ForcePlainTextResources != forcePlainTextResources {
		e.RewriteResources = true
	}
	e.ForcePlainTextResources = forcePlainTextResources
}

// GetEncryptionConfig retrieves the ETCDEncryptionConfig from the gardenerResourceDataList.
func GetEncryptionConfig(gardenerResourceDataList gardencorev1alpha1helper.GardenerResourceDataList) (*EncryptionConfig, error) {
	infoData, err := infodata.GetInfoData(gardenerResourceDataList, common.ETCDEncryptionConfigDataName)
	if err != nil {
		return nil, err
	}
	if infoData == nil {
		return nil, nil
	}

	encryptionConfig, ok := infoData.(*EncryptionConfig)
	if !ok {
		return nil, fmt.Errorf("could not convert GardenerResourceData entry %s to ETCDEncryptionConfig", common.ETCDEncryptionConfigDataName)
	}
	return encryptionConfig, nil
}

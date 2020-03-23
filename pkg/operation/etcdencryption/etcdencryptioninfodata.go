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

package encryptionconfiguration

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

// ETCDEncryptionKey holds the the key and its name used to encrypt resources in ETCD.
type ETCDEncryptionKey struct {
	Key  string
	Name string
}

// ETCDEncryptionConfig holds information whether a key is active or not and whether resources should be forcefully kept in plain text
type ETCDEncryptionConfig struct {
	EncryptionKeys          []ETCDEncryptionKey
	ForcePlainTextResources bool
	RewriteResources        bool
}

// TypeVersion implements InfoData
func (e *ETCDEncryptionConfig) TypeVersion() infodata.TypeVersion {
	return ETCDEncryptionDataType
}

// Marshal ETCDEncryption InfoData
func (e *ETCDEncryptionConfig) Marshal() ([]byte, error) {
	encryptionKeysData := make([]ETCDEncryptionKeyData, len(e.EncryptionKeys))
	for i, encryptionKey := range e.EncryptionKeys {
		encryptionKeysData[i].Key = encryptionKey.Key
		encryptionKeysData[i].Name = encryptionKey.Name
	}
	return json.Marshal(&ETCDEncryptionConfigData{EncryptionKeys: encryptionKeysData, ForcePlainTextResources: e.ForcePlainTextResources, RewriteResources: e.RewriteResources})
}

// NewETCDEncryption creates a new ETCDEncryptionKey from a given key and name
func NewETCDEncryption(keys []ETCDEncryptionKey, forcePlainTextResources, rewriteResources bool) (*ETCDEncryptionConfig, error) {
	return &ETCDEncryptionConfig{keys, forcePlainTextResources, rewriteResources}, nil
}

// AddEncryptionKeyFromSecret gets the active etcd encryption key from the secret object and adds it to the ETCDEncryptionConfig.
func (e *ETCDEncryptionConfig) AddEncryptionKeyFromSecret(secret *corev1.Secret) error {
	conf, err := ReadSecret(secret)
	if err != nil {
		return err
	}
	name, key, err := GetSecretKeyForResources(conf, common.EtcdEncryptionEncryptedResourceSecrets)
	if err != nil {
		return err
	}

	etcdKey := ETCDEncryptionKey{
		Key:  key,
		Name: name,
	}
	e.EncryptionKeys = append(e.EncryptionKeys, etcdKey)
	return nil
}

// AddNewEncryptionKey generates a new etcd encryption key and adds it to the ETCDEncryptionConfig.
func (e *ETCDEncryptionConfig) AddNewEncryptionKey() error {
	key, err := NewEncryptionKey(time.Now(), rand.Reader)
	if err != nil {
		return err
	}
	etcdKey := ETCDEncryptionKey{
		Key:  key.Secret,
		Name: key.Name,
	}
	e.EncryptionKeys = append(e.EncryptionKeys, etcdKey)
	return nil
}

// SetForcePlainTextResources sets whether resources should be encrypted or not.
// If the configuration changes RewriteResource is set to true.
func (e *ETCDEncryptionConfig) SetForcePlainTextResources(forcePlainTextResources bool) {
	if e.ForcePlainTextResources != forcePlainTextResources {
		e.RewriteResources = true
	}
	e.ForcePlainTextResources = forcePlainTextResources
}

// GetETCDEncryptionConfig retrieves the ETCDEncryptionConfig from the gardenerResourceDataList.
func GetETCDEncryptionConfig(gardenerResourceDataList gardencorev1alpha1helper.GardenerResourceDataList) (*ETCDEncryptionConfig, error) {
	infoData, err := infodata.GetInfoData(gardenerResourceDataList, common.ETCDSecretsEncryptionConfigDataName)
	if err != nil {
		return nil, err
	}
	if infoData == nil {
		return nil, nil
	}

	encryptionConfig, ok := infoData.(*ETCDEncryptionConfig)
	if !ok {
		return nil, fmt.Errorf("could not convert GardenerResourceData entry %s to ETCDEncryptionConfig", common.ETCDSecretsEncryptionConfigDataName)
	}
	return encryptionConfig, nil
}

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
	"encoding/json"
	"fmt"

	"github.com/gardener/gardener/pkg/utils/infodata"
)

// ETCDEncryptionDataType is the type used to denote an ETCDKeyData structure in the ShootState
const ETCDEncryptionDataType = infodata.TypeVersion("etcdEncryption")

// ETCDEncryptionStateDataType is the type used to denote an ETCDEncryptionStateData structure in the ShootState
//const ETCDEncryptionStateDataType = infodata.TypeVersion("etcdEncryptionStateData")

func init() {
	infodata.Register(ETCDEncryptionDataType, Unmarshal)
}

// EncryptionKeyData holds the key and its name used to encrypt resources in ETCD
type EncryptionKeyData struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// EncryptionConfigData holds a list of keys and information whether resources should be forcefully persisted in plain text and rewritten if the configuration changes.
type EncryptionConfigData struct {
	EncryptionKeys          []EncryptionKeyData `json:"encryptionKeys"`
	ForcePlainTextResources bool                `json:"forcePlainTextResources"`
	RewriteResources        bool                `json:"rewriteResources"`
}

// Unmarshal unmarshals an ETCDKeyData json.
func Unmarshal(bytes []byte) (infodata.InfoData, error) {
	if bytes == nil {
		return nil, fmt.Errorf("no data given")
	}
	data := &EncryptionConfigData{}
	err := json.Unmarshal(bytes, data)
	if err != nil {
		return nil, err
	}

	encryptionKeys := make([]EncryptionKey, len(data.EncryptionKeys))
	for i, encryptionKey := range data.EncryptionKeys {
		encryptionKeys[i].Key = encryptionKey.Key
		encryptionKeys[i].Name = encryptionKey.Name
	}

	return NewEncryptionConfig(encryptionKeys, data.ForcePlainTextResources, data.RewriteResources)
}

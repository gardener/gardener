// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"encoding/json"
	"fmt"

	"github.com/gardener/gardener/pkg/utils/infodata"
)

// PrivateKeyDataType is the type used to denote an PrivateKeyJSONData structure in the ShootState
const PrivateKeyDataType = infodata.TypeVersion("privateKey")

func init() {
	infodata.Register(PrivateKeyDataType, UnmarshalPrivateKey)
}

// PrivateKeyJSONData is the json representation of PrivateKeyInfoData used to store private key in the ShootState
type PrivateKeyJSONData struct {
	PrivateKey []byte `json:"privateKey"`
}

// UnmarshalPrivateKey unmarshals an PrivateKeyJSONData into an PrivateKeyInfoData.
func UnmarshalPrivateKey(bytes []byte) (infodata.InfoData, error) {
	if bytes == nil {
		return nil, fmt.Errorf("no data given")
	}
	data := &PrivateKeyJSONData{}
	err := json.Unmarshal(bytes, data)
	if err != nil {
		return nil, err
	}

	return NewPrivateKeyInfoData(data.PrivateKey), nil
}

// PrivateKeyInfoData holds the data of a private key.
type PrivateKeyInfoData struct {
	PrivateKey []byte
}

// TypeVersion implements InfoData
func (r *PrivateKeyInfoData) TypeVersion() infodata.TypeVersion {
	return PrivateKeyDataType
}

// Marshal implements InfoData
func (r *PrivateKeyInfoData) Marshal() ([]byte, error) {
	return json.Marshal(&PrivateKeyJSONData{r.PrivateKey})
}

// NewPrivateKeyInfoData creates a new PrivateKeyInfoData struct
func NewPrivateKeyInfoData(privateKey []byte) *PrivateKeyInfoData {
	return &PrivateKeyInfoData{privateKey}
}

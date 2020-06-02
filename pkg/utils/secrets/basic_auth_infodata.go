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

// BasicAuthDataType is the type used to denote an BasicAuthJSONData structure in the ShootState
const BasicAuthDataType = infodata.TypeVersion("basicAuth")

func init() {
	infodata.Register(BasicAuthDataType, UnmarshalBasicAuth)
}

// BasicAuthJSONData is the json representation of BasicAuthInfoData used to store BasicAuth metadata in the ShootState
type BasicAuthJSONData struct {
	Password string `json:"password"`
}

// UnmarshalBasicAuth unmarshals an BasicAuthJSONData into a BasicAuthInfoData struct.
func UnmarshalBasicAuth(bytes []byte) (infodata.InfoData, error) {
	if bytes == nil {
		return nil, fmt.Errorf("no data given")
	}
	data := &BasicAuthJSONData{}
	err := json.Unmarshal(bytes, data)
	if err != nil {
		return nil, err
	}

	return NewBasicAuthInfoData(data.Password), nil
}

// BasicAuthInfoData holds the password used for basic authentication.
type BasicAuthInfoData struct {
	Password string
}

// TypeVersion implements InfoData
func (b *BasicAuthInfoData) TypeVersion() infodata.TypeVersion {
	return BasicAuthDataType
}

// Marshal implements InfoData
func (b *BasicAuthInfoData) Marshal() ([]byte, error) {
	return json.Marshal(&BasicAuthJSONData{b.Password})
}

// NewBasicAuthInfoData creates a new BasicAuthInfoData struct with the given password.
func NewBasicAuthInfoData(password string) infodata.InfoData {
	return &BasicAuthInfoData{password}
}

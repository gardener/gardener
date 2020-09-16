// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

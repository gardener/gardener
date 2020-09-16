// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

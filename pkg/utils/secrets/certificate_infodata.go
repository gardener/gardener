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

// CertificateDataType is the type used to denote an CertificateJSONData structure in the ShootState
const CertificateDataType = infodata.TypeVersion("certificate")

func init() {
	infodata.Register(CertificateDataType, UnmarshalCert)
}

// CertificateJSONData is the json representation of CertificateInfoData used to store Certificate metadata in the ShootState
type CertificateJSONData struct {
	PrivateKey  []byte `json:"privateKey"`
	Certificate []byte `json:"certificate"`
}

// UnmarshalCert unmarshals an CertificateJSONData into a CertificateInfoData.
func UnmarshalCert(bytes []byte) (infodata.InfoData, error) {
	if bytes == nil {
		return nil, fmt.Errorf("no data given")
	}
	data := &CertificateJSONData{}
	err := json.Unmarshal(bytes, data)
	if err != nil {
		return nil, err
	}

	return NewCertificateInfoData(data.PrivateKey, data.Certificate), nil
}

// CertificateInfoData holds a certificate's private key data and certificate data.
type CertificateInfoData struct {
	PrivateKey  []byte
	Certificate []byte
}

// TypeVersion implements InfoData
func (c *CertificateInfoData) TypeVersion() infodata.TypeVersion {
	return CertificateDataType
}

// Marshal implements InfoData
func (c *CertificateInfoData) Marshal() ([]byte, error) {
	return json.Marshal(&CertificateJSONData{c.PrivateKey, c.Certificate})
}

// NewCertificateInfoData creates a new CertificateInfoData struct
func NewCertificateInfoData(privateKey, certificate []byte) *CertificateInfoData {
	return &CertificateInfoData{privateKey, certificate}
}

// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets_test

import (
	"fmt"

	"github.com/gardener/gardener/pkg/utils"

	. "github.com/gardener/gardener/pkg/utils/secrets"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Certificate InfoData", func() {
	var (
		certificateJSON     = []byte(fmt.Sprintf(`{"privateKey":"%s","certificate":"%s"}`, utils.EncodeBase64([]byte("foo")), utils.EncodeBase64([]byte("bar"))))
		certificateInfoData = &CertificateInfoData{
			PrivateKey:  []byte("foo"),
			Certificate: []byte("bar"),
		}
	)

	Describe("#UnmarshalCert", func() {
		It("should properly unmarshal CertificateJSONData into CertificateInfoData", func() {
			infoData, err := UnmarshalCert(certificateJSON)
			Expect(err).NotTo(HaveOccurred())
			Expect(infoData).To(Equal(certificateInfoData))
		})
	})

	Describe("#Marshal", func() {
		It("should properly marshal CertificateInfoData into CertificateJSONData", func() {
			data, err := certificateInfoData.Marshal()
			Expect(err).NotTo(HaveOccurred())
			Expect(data).To(Equal(certificateJSON))
		})
	})

	Describe("#TypeVersion", func() {
		It("should return the correct TypeVersion", func() {
			typeVersion := certificateInfoData.TypeVersion()
			Expect(typeVersion).To(Equal(CertificateDataType))
		})
	})

	Describe("#NewCertificateInfoData", func() {
		It("should return new CertificateInfoData from the passed private key and certificate", func() {
			newCertificateInfoData := NewCertificateInfoData([]byte("foo"), []byte("bar"))
			Expect(newCertificateInfoData).To(Equal(certificateInfoData))
		})
	})
})

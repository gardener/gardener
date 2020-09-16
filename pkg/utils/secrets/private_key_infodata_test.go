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

var _ = Describe("PrivateKey InfoData", func() {
	var (
		privateKeyJSON     = []byte(fmt.Sprintf(`{"privateKey":"%s"}`, utils.EncodeBase64([]byte("foo"))))
		privateKeyInfoData = &PrivateKeyInfoData{
			PrivateKey: []byte("foo"),
		}
	)

	Describe("#UnmarshalPrivateKey", func() {
		It("should properly unmarshal PrivateKeyJSONData into PrivateKeyInfoData", func() {
			infoData, err := UnmarshalPrivateKey(privateKeyJSON)
			Expect(err).NotTo(HaveOccurred())
			Expect(infoData).To(Equal(privateKeyInfoData))
		})
	})

	Describe("#Marshal", func() {
		It("should properly marshal PrivateKeyInfoData into PrivateKeyJSONData", func() {
			data, err := privateKeyInfoData.Marshal()
			Expect(err).NotTo(HaveOccurred())
			Expect(data).To(Equal(privateKeyJSON))
		})
	})
})

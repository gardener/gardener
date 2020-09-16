// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package etcdencryption_test

import (
	. "github.com/gardener/gardener/pkg/operation/etcdencryption"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ETCD Encryption Data", func() {
	var (
		etcdEncryptionDataJson = []byte("{\"encryptionKeys\":[{\"key\":\"foo\",\"name\":\"bar\"}],\"forcePlainTextResources\":true,\"rewriteResources\":true}")
		etcdEncryptionConfig   = &EncryptionConfig{
			EncryptionKeys: []EncryptionKey{
				{
					Key:  "foo",
					Name: "bar",
				},
			},
			ForcePlainTextResources: true,
			RewriteResources:        true,
		}
	)
	It("should correctly unmarshal EncryptionConfigData struct into EncryptionConfig", func() {
		data, err := Unmarshal([]byte(etcdEncryptionDataJson))
		Expect(err).NotTo(HaveOccurred())

		etcdKey, ok := data.(*EncryptionConfig)
		Expect(ok).To(BeTrue())
		Expect(etcdKey).To(Equal(etcdEncryptionConfig))
	})
})

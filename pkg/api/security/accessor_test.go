// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package security_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/api/security"
	gardensecurity "github.com/gardener/gardener/pkg/apis/security"
)

var _ = Describe("Accessor", func() {
	Describe("#Accessor", func() {
		It("Should succeed to create an accessor", func() {
			credentialsBinding := &gardensecurity.CredentialsBinding{}
			credentialsBindingAccessor, err := security.Accessor(credentialsBinding)
			Expect(err).ToNot(HaveOccurred())
			Expect(credentialsBinding).To(Equal(credentialsBindingAccessor))
		})
	})
})

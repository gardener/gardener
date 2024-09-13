// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package security_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

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

		It("Should fail to create an accessor because of the missing implementation", func() {
			secret := &corev1.Secret{}
			_, err := security.Accessor(secret)
			Expect(err).To(HaveOccurred())
		})
	})
})

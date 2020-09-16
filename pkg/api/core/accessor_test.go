// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"github.com/gardener/gardener/pkg/apis/core"

	. "github.com/gardener/gardener/pkg/api/core"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Accessor", func() {
	Describe("#Accessor", func() {
		It("Should succeed to create an accessor", func() {
			shoot := &core.Shoot{}
			shootAcessor, err := Accessor(shoot)
			Expect(err).To(Not(HaveOccurred()))
			Expect(shoot).To(Equal(shootAcessor))
		})

		It("Should fail to create an accessor because of the missing implementation", func() {
			secretBinding := &corev1.Secret{}
			_, err := Accessor(secretBinding)
			Expect(err).To(HaveOccurred())
		})
	})
})

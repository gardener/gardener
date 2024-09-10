// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	. "github.com/gardener/gardener/pkg/api/core"
	"github.com/gardener/gardener/pkg/apis/core"
)

var _ = Describe("Accessor", func() {
	Describe("#Accessor", func() {
		It("Should succeed to create an accessor", func() {
			shoot := &core.Shoot{}
			shootAccessor, err := Accessor(shoot)
			Expect(err).To(Not(HaveOccurred()))
			Expect(shoot).To(Equal(shootAccessor))
		})

		It("Should fail to create an accessor because of the missing implementation", func() {
			secret := &corev1.Secret{}
			_, err := Accessor(secret)
			Expect(err).To(HaveOccurred())
		})
	})
})

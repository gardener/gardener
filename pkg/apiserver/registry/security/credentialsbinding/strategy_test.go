// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package credentialsbinding_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/apis/security"
	credentialsbindingregistry "github.com/gardener/gardener/pkg/apiserver/registry/security/credentialsbinding"
)

var _ = Describe("Strategy", func() {
	var credentialsBinding *security.CredentialsBinding

	BeforeEach(func() {
		credentialsBinding = &security.CredentialsBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "profile",
				Namespace: "garden",
			},
		}
	})

	Describe("#PrepareForCreate", func() {
		It("should set the name if not set", func() {
			credentialsBinding.SetName("")

			credentialsbindingregistry.Strategy.PrepareForCreate(context.TODO(), credentialsBinding)

			Expect(credentialsBinding.GetName()).NotTo(BeEmpty())
		})

		It("should set name with generateName as prefix", func() {
			genName := "prefix-"
			credentialsBinding.GenerateName = genName
			credentialsBinding.Name = ""

			credentialsbindingregistry.Strategy.PrepareForCreate(context.TODO(), credentialsBinding)

			Expect(credentialsBinding.GetGenerateName()).To(Equal(genName))
			Expect(credentialsBinding.GetName()).To(HavePrefix(genName))
		})

		It("should not overwrite already set name", func() {
			credentialsBinding.SetName("bar")

			credentialsbindingregistry.Strategy.PrepareForCreate(context.TODO(), credentialsBinding)

			Expect(credentialsBinding.GetName()).To(Equal("bar"))
		})
	})
})

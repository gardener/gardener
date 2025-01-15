// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package workloadidentity_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/gardener/gardener/pkg/apis/security"
	"github.com/gardener/gardener/pkg/apiserver/registry/security/workloadidentity"
)

var _ = Describe("Workload Identity Strategy Test", func() {

	var (
		issuerURL string
		wi        *security.WorkloadIdentity
		s         rest.RESTCreateUpdateStrategy
		ctx       context.Context
	)

	BeforeEach(func() {
		wi = &security.WorkloadIdentity{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "garden",
			},
		}
		issuerURL = "https://issuer.gardener.cloud.local"
		s = workloadidentity.NewStrategy(&issuerURL)
		ctx = context.Background()
	})

	Describe("#PrepareForCreate", func() {
		It("should set UID", func() {
			Expect(wi.GetUID()).To(BeEmpty())
			s.PrepareForCreate(ctx, wi)

			Expect(wi.GetUID()).ToNot(BeEmpty())
		})

		It("should set name without generateName", func() {
			Expect(wi.GetName()).To(BeEmpty())
			Expect(wi.GetGenerateName()).To(BeEmpty())

			s.PrepareForCreate(ctx, wi)

			Expect(wi.GetGenerateName()).To(BeEmpty())
			Expect(wi.GetName()).ToNot(BeEmpty())
		})

		It("should set name with generateName as prefix", func() {
			genName := "prefix-"
			wi.GenerateName = genName
			Expect(wi.GetName()).To(BeEmpty())
			Expect(wi.GetGenerateName()).To(Equal(genName))

			s.PrepareForCreate(ctx, wi)

			Expect(wi.GetGenerateName()).To(Equal(genName))
			name := wi.GetName()
			Expect(name).ToNot(BeEmpty())
			Expect(name).To(HavePrefix(genName))
		})

		It("should not overwrite already set name", func() {
			name := "name"
			wi.Name = name
			Expect(wi.GetName()).To(Equal(name))

			s.PrepareForCreate(ctx, wi)
			Expect(wi.GetName()).To(Equal(name))
		})

		It("should set status.sub value", func() {
			wi.Name = "name"
			uid := "52c48341-ce0f-4400-a902-e665ba443c78"
			wi.UID = types.UID(uid)
			Expect(wi.Status.Sub).To(BeEmpty())
			s.PrepareForCreate(ctx, wi)

			Expect(wi.Status.Sub).To(Equal("gardener.cloud:workloadidentity:garden:name:" + uid))
		})

		It("should not overwrite already set status.sub value", func() {
			wi.Name = "name"
			uid := "52c48341-ce0f-4400-a902-e665ba443c78"
			wi.UID = types.UID(uid)
			wi.Status.Sub = "foo"

			s.PrepareForCreate(ctx, wi)

			Expect(wi.Status.Sub).To(Equal("foo"))
		})

		It("should set status.issuer value", func() {
			s.PrepareForCreate(ctx, wi)
			Expect(*wi.Status.Issuer).To(Equal(issuerURL))
		})

		It("should reset status.issuer value", func() {
			presetIssuer := issuerURL + ".preset"
			wi.Status.Issuer = &presetIssuer

			s.PrepareForCreate(ctx, wi)
			Expect(*wi.Status.Issuer).To(Equal(issuerURL))
			Expect(*wi.Status.Issuer).ToNot(Equal(presetIssuer))
		})

		It("should overwrite status.issuer value", func() {
			newIssuerURL := "new" + issuerURL
			newStrategy := workloadidentity.NewStrategy(&newIssuerURL)

			s.PrepareForCreate(ctx, wi)
			Expect(*wi.Status.Issuer).To(Equal(issuerURL))

			newStrategy.PrepareForCreate(ctx, wi)
			Expect(*wi.Status.Issuer).To(Equal(newIssuerURL))
		})

		It("should unset status.issuer field", func() {
			wi.Status.Issuer = new(string)
			*wi.Status.Issuer = "some-issuer"
			newStrategy := workloadidentity.NewStrategy(nil)

			newStrategy.PrepareForCreate(ctx, wi)
			Expect(wi.Status.Issuer).To(BeNil())
		})
	})

	Describe("#PrepareForUpdate", func() {
		It("should set status.issuer value", func() {
			s.PrepareForUpdate(ctx, wi, nil)
			Expect(*wi.Status.Issuer).To(Equal(issuerURL))
		})

		It("should reset status.issuer value", func() {
			presetIssuer := issuerURL + ".preset"
			wi.Status.Issuer = &presetIssuer

			s.PrepareForUpdate(ctx, wi, nil)
			Expect(*wi.Status.Issuer).To(Equal(issuerURL))
			Expect(*wi.Status.Issuer).ToNot(Equal(presetIssuer))
		})

		It("should overwrite status.issuer value", func() {
			newIssuerURL := issuerURL + ".new"
			newStrategy := workloadidentity.NewStrategy(&newIssuerURL)

			s.PrepareForUpdate(ctx, wi, nil)
			Expect(*wi.Status.Issuer).To(Equal(issuerURL))

			newStrategy.PrepareForUpdate(ctx, wi, nil)
			Expect(*wi.Status.Issuer).To(Equal(newIssuerURL))
		})

		It("should update status.issuer value", func() {
			oldTokenIssuer := issuerURL + ".old"
			wi.Status.Issuer = &oldTokenIssuer

			s.PrepareForUpdate(ctx, wi, nil)
			Expect(*wi.Status.Issuer).To(Equal(issuerURL))
		})

		It("should unset status.issuer field", func() {
			wi.Status.Issuer = new(string)
			*wi.Status.Issuer = "some-issuer"
			newStrategy := workloadidentity.NewStrategy(nil)

			newStrategy.PrepareForUpdate(ctx, wi, nil)
			Expect(wi.Status.Issuer).To(BeNil())
		})
	})
})

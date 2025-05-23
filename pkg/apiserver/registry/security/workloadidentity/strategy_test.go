// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package workloadidentity_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/gardener/gardener/pkg/apis/security"
	"github.com/gardener/gardener/pkg/apiserver/registry/security/workloadidentity"
)

var _ = Describe("WorkloadIdentity Strategy Test", func() {

	var (
		wi  *security.WorkloadIdentity
		s   = workloadidentity.Strategy
		ctx context.Context
	)

	BeforeEach(func() {
		wi = &security.WorkloadIdentity{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "garden",
			},
		}
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
	})
})

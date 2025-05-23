// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/v1"
)

var _ = Describe("Conversion", func() {
	var scheme *runtime.Scheme

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(SchemeBuilder.AddToScheme(scheme)).ToNot(HaveOccurred())
	})

	Describe("ControllerDeployment conversion", func() {
		Describe("convert from v1 to internal", func() {
			var (
				in  *ControllerDeployment
				out *core.ControllerDeployment
			)

			BeforeEach(func() {
				in = &ControllerDeployment{}
				out = &core.ControllerDeployment{}
			})

			Context("helm type with rawChart", func() {
				BeforeEach(func() {
					in.Helm = &HelmControllerDeployment{
						RawChart: []byte("foo"),
						Values: &apiextensionsv1.JSON{
							Raw: []byte(`{"foo":["bar","baz"]}`),
						},
					}
				})

				It("should keep helm deployment", func() {
					Expect(scheme.Convert(in, out, nil)).To(Succeed())

					Expect(out.Type).To(BeEmpty(), "type is empty for non-custom type")
					Expect(out.ProviderConfig).To(BeNil(), "providerConfig is empty for non-custom type")
					Expect(out.Helm).To(Equal(&core.HelmControllerDeployment{
						RawChart: []byte("foo"),
						Values: &apiextensionsv1.JSON{
							Raw: []byte(`{"foo":["bar","baz"]}`),
						},
					}))
				})
			})

			Context("helm type with ociRepository", func() {
				BeforeEach(func() {
					in.Helm = &HelmControllerDeployment{
						OCIRepository: &OCIRepository{
							Repository: ptr.To("url"),
							Tag:        ptr.To("1.0.0"),
							Digest:     ptr.To("sha256:foo"),
						},
						Values: &apiextensionsv1.JSON{
							Raw: []byte(`{"foo":["bar","baz"]}`),
						},
					}
				})

				It("should keep helm deployment", func() {
					Expect(scheme.Convert(in, out, nil)).To(Succeed())

					Expect(out.Type).To(BeEmpty(), "type is empty for non-custom type")
					Expect(out.ProviderConfig).To(BeNil(), "providerConfig is empty for non-custom type")
					Expect(out.Helm).To(Equal(&core.HelmControllerDeployment{
						OCIRepository: &core.OCIRepository{
							Repository: ptr.To("url"),
							Tag:        ptr.To("1.0.0"),
							Digest:     ptr.To("sha256:foo"),
						},
						Values: &apiextensionsv1.JSON{
							Raw: []byte(`{"foo":["bar","baz"]}`),
						},
					}))
				})
			})

			Context("custom type", func() {
				BeforeEach(func() {
					metav1.SetMetaDataAnnotation(&in.ObjectMeta, MigrationControllerDeploymentType, "custom")
					metav1.SetMetaDataAnnotation(&in.ObjectMeta, MigrationControllerDeploymentProviderConfig, `{"foo":"bar"}`)
				})

				It("should convert type and providerConfig annotations to legacy structure", func() {
					Expect(scheme.Convert(in, out, nil)).To(Succeed())

					Expect(out.Annotations).NotTo(HaveKey(MigrationControllerDeploymentType))
					Expect(out.Annotations).NotTo(HaveKey(MigrationControllerDeploymentProviderConfig))
					Expect(out.Type).To(Equal("custom"))
					Expect(out.ProviderConfig).To(Equal(&runtime.Unknown{
						ContentType: "application/json",
						Raw:         []byte(`{"foo":"bar"}`),
					}))
					Expect(out.Helm).To(BeNil())
				})
			})
		})

		Describe("convert from internal to v1", func() {
			var (
				in  *core.ControllerDeployment
				out *ControllerDeployment
			)

			BeforeEach(func() {
				in = &core.ControllerDeployment{}
				out = &ControllerDeployment{}
			})

			Context("helm type", func() {
				BeforeEach(func() {
					in.Helm = &core.HelmControllerDeployment{
						RawChart: []byte("foo"),
						Values: &apiextensionsv1.JSON{
							Raw: []byte(`{"foo":["bar","baz"]}`),
						},
					}
				})

				It("should keep helm deployment", func() {
					Expect(scheme.Convert(in, out, nil)).To(Succeed())

					Expect(out.Helm).To(Equal(&HelmControllerDeployment{
						RawChart: []byte("foo"),
						Values: &apiextensionsv1.JSON{
							Raw: []byte(`{"foo":["bar","baz"]}`),
						},
					}))
				})
			})

			Context("custom type", func() {
				BeforeEach(func() {
					in.Type = "custom"
					in.ProviderConfig = &runtime.Unknown{
						ContentType: "application/json",
						Raw:         []byte(`{"foo":"bar"}`),
					}
				})

				It("should convert type and providerConfig to annotations", func() {
					Expect(scheme.Convert(in, out, nil)).To(Succeed())

					Expect(out.Annotations).To(HaveKeyWithValue(MigrationControllerDeploymentType, "custom"))
					Expect(out.Annotations).To(HaveKeyWithValue(MigrationControllerDeploymentProviderConfig, `{"foo":"bar"}`))
					Expect(out.Helm).To(BeNil())
				})
			})
		})
	})
})

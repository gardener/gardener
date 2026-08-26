// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package imagevector_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/yaml"

	. "github.com/gardener/gardener/pkg/utils/imagevector"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("validation", func() {
	var (
		imageVector           func(string, *string, *string, *string, string, string) ImageVector
		componentImageVectors func(string, ImageVector) ComponentImageVectors
	)

	BeforeEach(func() {
		imageVector = func(name string, ref, repository, tag *string, runtimeVersion, targetVersion string) ImageVector {
			return ImageVector{
				{
					Name:           name,
					Ref:            ref,
					Repository:     repository,
					Tag:            tag,
					RuntimeVersion: new(runtimeVersion),
					TargetVersion:  new(targetVersion),
				},
			}
		}

		componentImageVectors = func(name string, imageVector ImageVector) ComponentImageVectors {
			vector := struct {
				Images ImageVector `json:"images" yaml:"images"`
			}{
				Images: imageVector,
			}

			buf, err := yaml.Marshal(vector)
			Expect(err).NotTo(HaveOccurred())

			return ComponentImageVectors{
				name: string(buf),
			}
		}
	})

	Describe("#ValidateImageVector", func() {
		It("should allow valid image vectors", func() {
			errorList := ValidateImageVector(imageVector("test-image1", nil, new("test-repo"), new("test-tag"), ">= 1.6, < 1.8", ">= 1.8"), field.NewPath("images"))

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid invalid image vectors", func() {
			errorList := ValidateImageVector(imageVector("", nil, nil, new(""), "", "!@#"), field.NewPath("images"))

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("images[0].name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("images[0].ref/repository"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("images[0].tag"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("images[0].runtimeVersion"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("images[0].targetVersion"),
				})),
			))
		})

		It("should forbid empty ref", func() {
			Expect(ValidateImageVector(imageVector("foo", new(""), nil, nil, ">= 1.6", "< 1.8"), field.NewPath("images"))).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("images[0].ref"),
					"Detail": Equal("ref must not be empty if specified"),
				})),
			))
		})

		It("should forbid empty repository", func() {
			Expect(ValidateImageVector(imageVector("foo", nil, new(""), nil, ">= 1.6", "< 1.8"), field.NewPath("images"))).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("images[0].repository"),
					"Detail": Equal("repository must not be empty if specified"),
				})),
			))
		})

		It("should forbid specifying repository/tag when ref is set", func() {
			Expect(ValidateImageVector(imageVector("foo", new("ref"), new("repo"), new("tag"), ">= 1.6", "< 1.8"), field.NewPath("images"))).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("images[0].repository"),
					"Detail": Equal("cannot specify repository when ref is set"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("images[0].tag"),
					"Detail": Equal("cannot specify tag when ref is set"),
				})),
			))
		})
	})

	Describe("#ValidateComponentImageVectors", func() {
		It("should allow valid component image vectors", func() {
			errorList := ValidateComponentImageVectors(componentImageVectors("test-component1", imageVector("test-image1", nil, new("test-repo"), new("test-tag"), ">= 1.6, < 1.8", ">= 1.8")), field.NewPath("components"))

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid invalid component image vectors", func() {
			errorList := ValidateComponentImageVectors(componentImageVectors("", ImageVector{{}}), field.NewPath("components"))

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("components[].name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("components[].imageVectorOverwrite"),
				})),
			))
		})
	})

	Describe("#ValidateCABundle", func() {
		var (
			fldPath      *field.Path
			validCertPEM string
		)

		BeforeEach(func() {
			fldPath = field.NewPath("caBundle")

			ca, err := (&secretsutils.CertificateSecretConfig{
				Name:       "test-ca",
				CommonName: "TestCA",
				CertType:   secretsutils.CACert,
			}).GenerateCertificate()
			Expect(err).NotTo(HaveOccurred())
			validCertPEM = string(ca.SecretData()[secretsutils.DataKeyCertificateCA])
		})

		It("should allow nil", func() {
			Expect(ValidateCABundle(nil, fldPath)).To(BeEmpty())
			Expect(ValidateCABundle(&CABundle{Inline: nil}, fldPath)).To(BeEmpty())
		})

		It("should allow a valid CABundle", func() {
			Expect(ValidateCABundle(&CABundle{Inline: &validCertPEM}, fldPath)).To(BeEmpty())
		})

		It("should forbid non-PEM inline", func() {
			inline := "not-a-certificate"
			errs := ValidateCABundle(&CABundle{Inline: &inline}, fldPath)
			Expect(errs).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("caBundle.inline"),
				"Detail": ContainSubstring("not a valid PEM-encoded certificate"),
			}))))
		})

		It("should fail when inline certificate is already expired", func() {
			DeferCleanup(test.WithVar(&secretsutils.Clock, testclock.NewFakeClock(time.Now().Add(-8*24*time.Hour))))

			cert, err := (&secretsutils.CertificateSecretConfig{
				Name:       "test",
				CommonName: "test",
				CertType:   secretsutils.CACert,
				Validity:   new(24 * time.Hour),
			}).GenerateCertificate()
			Expect(err).NotTo(HaveOccurred())
			expired := string(cert.CertificatePEM)

			Expect(ValidateCABundle(&CABundle{Inline: &expired}, fldPath)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("caBundle.inline"),
					"Detail": Equal("certificate has expired"),
				})),
			))
		})
	})
})

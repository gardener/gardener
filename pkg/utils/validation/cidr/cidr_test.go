// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cidr_test

import (
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"

	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/pkg/utils/validation/cidr"
)

var _ = Describe("cidr", func() {
	Context("IPv4", func() {
		var (
			ipFamily          string
			invalidGardenCIDR = "invalid_cidr"
			validGardenCIDR   = "10.0.0.0/8"
			path              = field.NewPath("foo")
		)

		BeforeEach(func() {
			ipFamily = IPFamilyIPv4
		})

		Describe("NewCIDR", func() {
			It("should return a non-nil value", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr).ToNot(BeNil())
			})

		})

		Describe("GetCIDR", func() {
			It("should return a correct address", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr.GetCIDR()).To(Equal(validGardenCIDR))
			})
		})

		Describe("GetIPNet", func() {
			It("should return a correct IPNet", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				_, expected, _ := net.ParseCIDR(validGardenCIDR)

				actual := cdr.GetIPNet()

				Expect(actual).ToNot(BeNil())
				Expect(actual).To(Equal(expected))
			})

			It("should return an empty IPNet", func() {
				cdr := NewCIDR(invalidGardenCIDR, path)

				Expect(cdr.GetIPNet()).To(BeNil())
			})
		})

		Describe("GetFieldPath", func() {
			It("should return a correct FieldPath", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				actual := cdr.GetFieldPath()

				Expect(actual).ToNot(BeNil())
				Expect(actual).To(Equal(path))
			})

			It("should return a nil FieldPath", func() {
				cdr := NewCIDR(validGardenCIDR, nil)

				Expect(cdr.GetFieldPath()).To(BeNil())
			})
		})

		Describe("Parse", func() {
			It("should return a correct FieldPath", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr.Parse()).To(BeTrue())
			})

			It("should return a nil FieldPath", func() {
				cdr := NewCIDR(invalidGardenCIDR, path)

				Expect(cdr.Parse()).To(BeFalse())
			})
		})

		Describe("ValidateNotOverlap", func() {
			It("should not be a subset", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				other := NewCIDR(string("2.2.2.2/32"), path)

				Expect(cdr.ValidateNotOverlap(other)).To(BeEmpty())
			})

			It("should ignore nil values", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr.ValidateNotOverlap(nil)).To(BeEmpty())
			})

			It("should ignore when parse error", func() {
				cdr := NewCIDR(invalidGardenCIDR, path)
				other := NewCIDR(string("2.2.2.2/32"), path)

				Expect(cdr.ValidateNotOverlap(other)).To(BeEmpty())
			})

			It("should return a nil FieldPath", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				badCIDR := string("10.0.0.1/32")
				badPath := field.NewPath("bad")
				other := NewCIDR(badCIDR, badPath)

				Expect(cdr.ValidateNotOverlap(other)).To(ConsistOfFields(Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(badPath.String()),
					"BadValue": Equal(badCIDR),
					"Detail":   Equal(`must not overlap with "foo" ("10.0.0.0/8")`),
				}))
			})

			It("should return an error if CIDRs are the same", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				badPath := field.NewPath("bad")
				bad := NewCIDR(validGardenCIDR, badPath)

				Expect(cdr.ValidateNotOverlap(bad)).To(ConsistOfFields(Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(badPath.String()),
					"BadValue": Equal(validGardenCIDR),
					"Detail":   Equal(`must not overlap with "foo" ("10.0.0.0/8")`),
				}))
			})

			It("should return an error if CIDRs overlap", func() {
				cdr := NewCIDR("10.1.0.0/16", path)
				badCIDR := "10.0.0.0/8"
				badPath := field.NewPath("bad")
				bad := NewCIDR(badCIDR, badPath)

				Expect(cdr.ValidateNotOverlap(bad)).To(ConsistOfFields(Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(badPath.String()),
					"BadValue": Equal(badCIDR),
					"Detail":   Equal(`must not overlap with "foo" ("10.1.0.0/16")`),
				}))
			})
		})

		Describe("ValidateParse", func() {
			It("should parse without errors", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr.ValidateParse()).To(BeEmpty())
			})

			It("should return a nil FieldPath", func() {
				cdr := NewCIDR(invalidGardenCIDR, path)

				Expect(cdr.ValidateParse()).To(ConsistOfFields(Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(path.String()),
					"BadValue": Equal(invalidGardenCIDR),
					"Detail":   Equal(`invalid CIDR address: invalid_cidr`),
				}))
			})
		})

		Describe("ValidateIPFamily", func() {
			It("should not return an error for CIDR that matches IP family", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr.ValidateIPFamily(ipFamily)).To(BeEmpty())
			})

			It("should not return an error if parsing failed", func() {
				cdr := NewCIDR(invalidGardenCIDR, path)

				Expect(cdr.ValidateIPFamily(ipFamily)).To(BeEmpty())
			})

			It("should return an error for CIDR that doesn't match IP family", func() {
				cdr := NewCIDR("2001:db8:11::/48", path)

				Expect(cdr.ValidateIPFamily(ipFamily)).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal(path.String()),
					"Detail": Equal(`must be a valid IPv4 address`),
				}))
			})
		})

		Describe("ValidateSubset", func() {
			It("should be a subset", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				other := NewCIDR(string("10.0.0.1/32"), field.NewPath("other"))

				Expect(cdr.ValidateSubset(other)).To(BeEmpty())
			})

			It("should ignore nil values", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr.ValidateSubset(nil)).To(BeEmpty())
			})

			It("should ignore parse errors", func() {
				cdr := NewCIDR(invalidGardenCIDR, path)
				other := NewCIDR(string("10.0.0.1/32"), field.NewPath("other"))

				Expect(cdr.ValidateSubset(other)).To(BeEmpty())
			})

			It("should not be a subset", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				other := NewCIDR(string("10.0.0.1/32"), field.NewPath("bad"))

				Expect(other.ValidateSubset(cdr)).To(ConsistOfFields(Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(path.String()),
					"BadValue": Equal(validGardenCIDR),
					"Detail":   Equal(`must be a subset of "bad" ("10.0.0.1/32")`),
				}))
			})

			It("superset subnet should not be a subset", func() {
				valid := NewCIDR(string("10.0.0.0/24"), field.NewPath("valid"))
				other := NewCIDR(validGardenCIDR, path)

				Expect(valid.ValidateSubset(other)).To(ConsistOfFields(Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(path.String()),
					"BadValue": Equal(other.GetIPNet().String()),
					"Detail":   Equal(`must be a subset of "valid" ("10.0.0.0/24")`),
				}))
			})
		})

		Describe("ValidateOverlap", func() {
			It("should return an error on disjoint subnets", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				badPath := field.NewPath("bad")
				badCIDR := "11.0.0.0/8"
				bad := NewCIDR(badCIDR, badPath)
				Expect(cdr.ValidateOverlap(bad)).To(ConsistOfFields(Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(badPath.String()),
					"BadValue": Equal(badCIDR),
					"Detail":   Equal(`must overlap with "foo" ("10.0.0.0/8")`),
				}))
			})

			It("should return no errors if cidr is subset", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				other := NewCIDR(string("10.5.0.0/16"), field.NewPath("other"))
				Expect(cdr.ValidateOverlap(other)).To(BeEmpty())
			})

			It("should return no errors if cidr is superset", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				other := NewCIDR(string("10.5.0.0/16"), field.NewPath("other"))
				Expect(other.ValidateOverlap(cdr)).To(BeEmpty())
			})
		})

		Describe("ValidateMaxSize", func() {
			It("should return no errors if cidr is same size than limit", func() {
				goodPath := field.NewPath("good")
				goodCIDR := "11.12.0.0/16"
				good := NewCIDR(goodCIDR, goodPath)

				Expect(good.ValidateMaxSize(16)).To(BeEmpty())
			})

			It("should return no errors if cidr is smaller than limit", func() {
				goodPath := field.NewPath("good")
				goodCIDR := "11.12.0.0/17"
				good := NewCIDR(goodCIDR, goodPath)

				Expect(good.ValidateMaxSize(16)).To(BeEmpty())
			})

			It("should return an error if cidr is larger than limit", func() {
				badPath := field.NewPath("bad")
				badCIDR := "11.12.0.0/15"
				bad := NewCIDR(badCIDR, badPath)

				Expect(bad.ValidateMaxSize(16)).To(ConsistOfFields(Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(badPath.String()),
					"BadValue": Equal(badCIDR),
					"Detail":   Equal(`cannot be larger than /16`),
				}))
			})
		})

		Describe("IsIPv4", func() {
			It("should return true on a valid IPv4 cidr", func() {
				goodPath := field.NewPath("good")
				goodCIDR := "10.100.0.0/16"
				good := NewCIDR(goodCIDR, goodPath)

				Expect(good.Parse()).To(BeTrue())
				Expect(good.IsIPv4()).To(BeTrue())
				Expect(good.IsIPv6()).To(BeFalse())
			})

			It("should return false on a malformed IPv4 cidr", func() {
				badPath := field.NewPath("bad")
				badCIDR := "10.100.0.0/123"
				bad := NewCIDR(badCIDR, badPath)

				Expect(bad.Parse()).To(BeFalse())
				Expect(bad.IsIPv4()).To(BeFalse())
				Expect(bad.IsIPv6()).To(BeFalse())
			})

			It("should return false on a valid IPv6 cidr", func() {
				wrongPath := field.NewPath("wrong")
				wrongCIDR := "2001:0db8::/16"
				wrong := NewCIDR(wrongCIDR, wrongPath)

				Expect(wrong.Parse()).To(BeTrue())
				Expect(wrong.IsIPv4()).To(BeFalse())
				Expect(wrong.IsIPv6()).To(BeTrue())
			})
		})
	})

	Context("IPv6", func() {
		var (
			ipFamily          string
			invalidGardenCIDR = "invalid_cidr"
			validGardenCIDR   = "2001:0db8:85a3::/104"
			path              = field.NewPath("foo")
		)

		BeforeEach(func() {
			ipFamily = IPFamilyIPv6
		})

		Describe("NewCIDR", func() {
			It("should return a non-nil value", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr).ToNot(BeNil())
			})

		})

		Describe("GetCIDR", func() {
			It("should return a correct address", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr.GetCIDR()).To(Equal(validGardenCIDR))
			})
		})

		Describe("GetIPNet", func() {
			It("should return a correct IPNet", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				_, expected, _ := net.ParseCIDR(validGardenCIDR)

				actual := cdr.GetIPNet()

				Expect(actual).ToNot(BeNil())
				Expect(actual).To(Equal(expected))
			})

			It("should return an empty IPNet", func() {
				cdr := NewCIDR(invalidGardenCIDR, path)

				Expect(cdr.GetIPNet()).To(BeNil())
			})
		})

		Describe("GetFieldPath", func() {
			It("should return a correct FieldPath", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				actual := cdr.GetFieldPath()

				Expect(actual).ToNot(BeNil())
				Expect(actual).To(Equal(path))
			})

			It("should return a nil FieldPath", func() {
				cdr := NewCIDR(validGardenCIDR, nil)

				Expect(cdr.GetFieldPath()).To(BeNil())
			})
		})

		Describe("Parse", func() {
			It("should return a correct FieldPath", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr.Parse()).To(BeTrue())
			})

			It("should return a nil FieldPath", func() {
				cdr := NewCIDR(invalidGardenCIDR, path)

				Expect(cdr.Parse()).To(BeFalse())
			})
		})

		Describe("ValidateNotOverlap", func() {
			It("should not be a subset", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				other := NewCIDR(string("3001:0db8:85a3::1/128"), path)

				Expect(cdr.ValidateNotOverlap(other)).To(BeEmpty())
			})

			It("should ignore nil values", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr.ValidateNotOverlap(nil)).To(BeEmpty())
			})

			It("should ignore when parse error", func() {
				cdr := NewCIDR(invalidGardenCIDR, path)
				other := NewCIDR(string("3001:0db8:85a3::1/128"), path)

				Expect(cdr.ValidateNotOverlap(other)).To(BeEmpty())
			})

			It("should return a nil FieldPath", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				badCIDR := string("2001:0db8:85a3::1/128")
				badPath := field.NewPath("bad")
				other := NewCIDR(badCIDR, badPath)

				Expect(cdr.ValidateNotOverlap(other)).To(ConsistOfFields(Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(badPath.String()),
					"BadValue": Equal(badCIDR),
					"Detail":   Equal(`must not overlap with "foo" ("2001:0db8:85a3::/104")`),
				}))
			})

			It("should return an error if CIDRs overlap", func() {
				cdr := NewCIDR("2001:0db8::/16", path)
				badCIDR := validGardenCIDR
				badPath := field.NewPath("bad")
				other := NewCIDR(badCIDR, badPath)

				Expect(cdr.ValidateNotOverlap(other)).To(ConsistOfFields(Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(badPath.String()),
					"BadValue": Equal(badCIDR),
					"Detail":   Equal(`must not overlap with "foo" ("2001:0db8::/16")`),
				}))
			})
		})

		Describe("ValidateParse", func() {
			It("should parse without errors", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr.ValidateParse()).To(BeEmpty())
			})

			It("should return a nil FieldPath", func() {
				cdr := NewCIDR(invalidGardenCIDR, path)

				Expect(cdr.ValidateParse()).To(ConsistOfFields(Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(path.String()),
					"BadValue": Equal(invalidGardenCIDR),
					"Detail":   Equal(`invalid CIDR address: invalid_cidr`),
				}))
			})
		})

		Describe("ValidateIPFamily", func() {
			It("should not return an error for CIDR that matches IP family", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr.ValidateIPFamily(ipFamily)).To(BeEmpty())
			})

			It("should not return an error if parsing failed", func() {
				cdr := NewCIDR(invalidGardenCIDR, path)

				Expect(cdr.ValidateIPFamily(ipFamily)).To(BeEmpty())
			})

			It("should return an error for CIDR that doesn't match IP family", func() {
				cdr := NewCIDR("10.1.0.0/16", path)

				Expect(cdr.ValidateIPFamily(ipFamily)).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal(path.String()),
					"Detail": Equal(`must be a valid IPv6 address`),
				}))
			})
		})

		Describe("ValidateSubset", func() {
			It("should be a subset", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				other := NewCIDR(string("2001:0db8:85a3::1/128"), field.NewPath("other"))

				Expect(cdr.ValidateSubset(other)).To(BeEmpty())
			})

			It("should ignore nil values", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr.ValidateSubset(nil)).To(BeEmpty())
			})

			It("should ignore parse errors", func() {
				cdr := NewCIDR(invalidGardenCIDR, path)
				other := NewCIDR(string("2001:0db8:85a3::1/128"), field.NewPath("other"))

				Expect(cdr.ValidateSubset(other)).To(BeEmpty())
			})

			It("should not be a subset", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				other := NewCIDR(string("2001:0db8:85a3::1/128"), field.NewPath("bad"))

				Expect(other.ValidateSubset(cdr)).To(ConsistOfFields(Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(path.String()),
					"BadValue": Equal(validGardenCIDR),
					"Detail":   Equal(`must be a subset of "bad" ("2001:0db8:85a3::1/128")`),
				}))
			})

			It("superset subnet should not be a subset", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				other := NewCIDR(string("2001:0db8:85a3::/128"), field.NewPath("bad"))

				Expect(other.ValidateSubset(cdr)).To(ConsistOfFields(Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(path.String()),
					"BadValue": Equal(validGardenCIDR),
					"Detail":   Equal(`must be a subset of "bad" ("2001:0db8:85a3::/128")`),
				}))
			})
		})

		Describe("ValidateOverlap", func() {
			It("should return an error on disjoint subnets", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				badPath := field.NewPath("bad")
				badCIDR := "2002::/32"
				bad := NewCIDR(badCIDR, badPath)
				Expect(cdr.ValidateOverlap(bad)).To(ConsistOfFields(Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(badPath.String()),
					"BadValue": Equal(badCIDR),
					"Detail":   Equal(`must overlap with "foo" ("2001:0db8:85a3::/104")`),
				}))
			})

			It("should return no errors if cidr is subset", func() {
				cdr := NewCIDR("2001:0db8::/32", path)
				other := NewCIDR(validGardenCIDR, field.NewPath("other"))
				Expect(cdr.ValidateOverlap(other)).To(BeEmpty())
			})

			It("should return no errors if cidr is superset", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				other := NewCIDR("2001:0db8::/32", field.NewPath("other"))
				Expect(cdr.ValidateOverlap(other)).To(BeEmpty())
			})
		})

		Describe("ValidateMaxSize", func() {
			It("should return no errors if cidr is same size as limit", func() {
				goodPath := field.NewPath("good")
				goodCIDR := "2001:1db8::/32"
				good := NewCIDR(goodCIDR, goodPath)

				Expect(good.ValidateMaxSize(32)).To(BeEmpty())
			})

			It("should return no errors if cidr is smaller than limit", func() {
				goodPath := field.NewPath("good")
				goodCIDR := "2001:1db8::/33"
				good := NewCIDR(goodCIDR, goodPath)

				Expect(good.ValidateMaxSize(32)).To(BeEmpty())
			})

			It("should return an error if cidr is larger than limit", func() {
				badPath := field.NewPath("bad")
				badCIDR := "2001:1db8::/31"
				bad := NewCIDR(badCIDR, badPath)

				Expect(bad.ValidateMaxSize(32)).To(ConsistOfFields(Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(badPath.String()),
					"BadValue": Equal(badCIDR),
					"Detail":   Equal(`cannot be larger than /32`),
				}))
			})
		})

		Describe("IsIPv6", func() {
			It("should return true on a valid IPv6 cidr", func() {
				goodPath := field.NewPath("good")
				goodCIDR := "2001:0db8::/16"
				good := NewCIDR(goodCIDR, goodPath)

				Expect(good.Parse()).To(BeTrue())
				Expect(good.IsIPv4()).To(BeFalse())
				Expect(good.IsIPv6()).To(BeTrue())
			})

			It("should return false on a malformed IPv6 cidr", func() {
				badPath := field.NewPath("bad")
				badCIDR := "2001:0db8:xxyy:/64"
				bad := NewCIDR(badCIDR, badPath)

				Expect(bad.Parse()).To(BeFalse())
				Expect(bad.IsIPv4()).To(BeFalse())
				Expect(bad.IsIPv6()).To(BeFalse())
			})

			It("should return false on a valid IPv4 cidr", func() {
				wrongPath := field.NewPath("wrong")
				wrongCIDR := "10.100.0.0/16"
				wrong := NewCIDR(wrongCIDR, wrongPath)

				Expect(wrong.Parse()).To(BeTrue())
				Expect(wrong.IsIPv4()).To(BeTrue())
				Expect(wrong.IsIPv6()).To(BeFalse())
			})
		})
	})
})

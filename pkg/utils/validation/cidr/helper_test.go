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

var _ = Describe("#ValidateCIDRIPFamily", func() {
	var (
		ipV4CIDRs, ipV6CIDRs []CIDR
	)

	BeforeEach(func() {
		path := field.NewPath("cidrs")

		ipV4CIDRs = []CIDR{
			NewCIDR("10.0.1.0/24", path.Index(0)),
			NewCIDR("101.2.0.0/16", path.Index(1)),
		}
		ipV6CIDRs = []CIDR{
			NewCIDR("2001:db8:1::/48", path.Index(2)),
			NewCIDR("2001:db8:2:3::/64", path.Index(3)),
		}
	})

	It("should not return errors if all CIDRs match the ipFamily", func() {
		Expect(ValidateCIDRIPFamily(ipV4CIDRs, "IPv4")).To(BeEmpty())
		Expect(ValidateCIDRIPFamily(ipV6CIDRs, "IPv6")).To(BeEmpty())
	})

	It("should skip nil values in CIDRs", func() {
		Expect(ValidateCIDRIPFamily(append(append([]CIDR{nil}, ipV4CIDRs...), nil), "IPv4")).To(BeEmpty())
		Expect(ValidateCIDRIPFamily(append(append([]CIDR{nil}, ipV6CIDRs...), nil), "IPv6")).To(BeEmpty())
	})

	It("should return errors if all CIDRs don't match ipFamily", func() {
		Expect(ValidateCIDRIPFamily(ipV6CIDRs, "IPv4")).To(ConsistOfFields(
			Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("cidrs[2]"),
				"Detail": Equal("must be a valid IPv4 address"),
			},
			Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("cidrs[3]"),
				"Detail": Equal("must be a valid IPv4 address"),
			},
		))

		Expect(ValidateCIDRIPFamily(ipV4CIDRs, "IPv6")).To(ConsistOfFields(
			Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("cidrs[0]"),
				"Detail": Equal("must be a valid IPv6 address"),
			},
			Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("cidrs[1]"),
				"Detail": Equal("must be a valid IPv6 address"),
			},
		))
	})

	It("should return errors if some CIDRs don't match ipFamily", func() {
		mixedCIDRs := append(ipV4CIDRs, ipV6CIDRs...)
		Expect(ValidateCIDRIPFamily(mixedCIDRs, "IPv4")).To(ConsistOfFields(
			Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("cidrs[2]"),
				"Detail": Equal("must be a valid IPv4 address"),
			},
			Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("cidrs[3]"),
				"Detail": Equal("must be a valid IPv4 address"),
			},
		))

		Expect(ValidateCIDRIPFamily(mixedCIDRs, "IPv6")).To(ConsistOfFields(
			Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("cidrs[0]"),
				"Detail": Equal("must be a valid IPv6 address"),
			},
			Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("cidrs[1]"),
				"Detail": Equal("must be a valid IPv6 address"),
			},
		))
	})
})

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
				other := NewCIDR("2.2.2.2/32", path)

				Expect(cdr.ValidateNotOverlap(other)).To(BeEmpty())
			})

			It("should ignore nil values", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr.ValidateNotOverlap(nil)).To(BeEmpty())
			})

			It("should ignore when parse error", func() {
				cdr := NewCIDR(invalidGardenCIDR, path)
				other := NewCIDR("2.2.2.2/32", path)

				Expect(cdr.ValidateNotOverlap(other)).To(BeEmpty())
			})

			It("should return a nil FieldPath", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				badCIDR := "10.0.0.1/32"
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
				other := NewCIDR("10.0.0.1/32", field.NewPath("other"))

				Expect(cdr.ValidateSubset(other)).To(BeEmpty())
			})

			It("should ignore nil values", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr.ValidateSubset(nil)).To(BeEmpty())
			})

			It("should ignore parse errors", func() {
				cdr := NewCIDR(invalidGardenCIDR, path)
				other := NewCIDR("10.0.0.1/32", field.NewPath("other"))

				Expect(cdr.ValidateSubset(other)).To(BeEmpty())
			})

			It("should not be a subset", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				other := NewCIDR("10.0.0.1/32", field.NewPath("bad"))

				Expect(other.ValidateSubset(cdr)).To(ConsistOfFields(Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(path.String()),
					"BadValue": Equal(validGardenCIDR),
					"Detail":   Equal(`must be a subset of "bad" ("10.0.0.1/32")`),
				}))
			})

			It("superset subnet should not be a subset", func() {
				valid := NewCIDR("10.0.0.0/24", field.NewPath("valid"))
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
				other := NewCIDR("10.5.0.0/1", field.NewPath("other"))
				Expect(cdr.ValidateOverlap(other)).To(BeEmpty())
			})

			It("should return no errors if cidr is superset", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				other := NewCIDR("10.5.0.0/16", field.NewPath("other"))
				Expect(other.ValidateOverlap(cdr)).To(BeEmpty())
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
				other := NewCIDR("3001:0db8:85a3::1/128", path)

				Expect(cdr.ValidateNotOverlap(other)).To(BeEmpty())
			})

			It("should ignore nil values", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr.ValidateNotOverlap(nil)).To(BeEmpty())
			})

			It("should ignore when parse error", func() {
				cdr := NewCIDR(invalidGardenCIDR, path)
				other := NewCIDR("3001:0db8:85a3::1/128", path)

				Expect(cdr.ValidateNotOverlap(other)).To(BeEmpty())
			})

			It("should return a nil FieldPath", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				badCIDR := "2001:0db8:85a3::1/128"
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
				other := NewCIDR("2001:0db8:85a3::1/128", field.NewPath("other"))

				Expect(cdr.ValidateSubset(other)).To(BeEmpty())
			})

			It("should ignore nil values", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr.ValidateSubset(nil)).To(BeEmpty())
			})

			It("should ignore parse errors", func() {
				cdr := NewCIDR(invalidGardenCIDR, path)
				other := NewCIDR("2001:0db8:85a3::1/128", field.NewPath("other"))

				Expect(cdr.ValidateSubset(other)).To(BeEmpty())
			})

			It("should not be a subset", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				other := NewCIDR("2001:0db8:85a3::1/128", field.NewPath("bad"))

				Expect(other.ValidateSubset(cdr)).To(ConsistOfFields(Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(path.String()),
					"BadValue": Equal(validGardenCIDR),
					"Detail":   Equal(`must be a subset of "bad" ("2001:0db8:85a3::1/128")`),
				}))
			})

			It("superset subnet should not be a subset", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				other := NewCIDR("2001:0db8:85a3::/128", field.NewPath("bad"))

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
	})
})

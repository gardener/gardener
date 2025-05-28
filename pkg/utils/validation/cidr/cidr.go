// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cidr

import (
	"fmt"
	"net"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	// IPFamilyIPv4 is the IPv4 IP family.
	IPFamilyIPv4 string = "IPv4"
	// IPFamilyIPv6 is the IPv6 IP family.
	IPFamilyIPv6 string = "IPv6"
)

// CIDR contains CIDR and Path information
type CIDR interface {
	// GetCIDR returns the provided CIDR
	GetCIDR() string
	// GetFieldPath returns the fieldpath
	GetFieldPath() *field.Path
	// GetIPNet optionally returns the IPNet of the CIDR
	GetIPNet() *net.IPNet
	// Parse checks if CIDR parses
	Parse() bool
	// ValidateNotOverlap returns errors if subsets overlap with CIDR. This is the inverse operation of ValidateOverlap.
	ValidateNotOverlap(subsets ...CIDR) field.ErrorList
	// ValidateParse returns errors CIDR can't be parsed.
	ValidateParse() field.ErrorList
	// ValidateIPFamily returns error if IPFamily does not match CIDR.
	ValidateIPFamily(ipFamily string) field.ErrorList
	// ValidateSubset returns errors if subsets is not a subset.
	ValidateSubset(subsets ...CIDR) field.ErrorList
	// LastIPInRange returns the last IP in the CIDR range.
	LastIPInRange() net.IP
	// ValidateOverlap returns errors if the subnets do not overlap with CIDR.
	ValidateOverlap(subsets ...CIDR) field.ErrorList
	// ValidateMaxSize returns errors if the subnet is larger than the given bits. e.g. /15 is larger than /16
	ValidateMaxSize(bits int) field.ErrorList
	// IsIPv4 returns true if the CIDR is a valid v4 CIDR, false otherwise.
	IsIPv4() bool
	// IsIPv6 returns true if the CIDR is a valid v6 CIDR, false otherwise.
	IsIPv6() bool
}

type cidrPath struct {
	cidr       string
	fieldPath  *field.Path
	net        *net.IPNet
	ParseError error
}

// NewCIDR creates a new instance of cidrPath
func NewCIDR(c string, f *field.Path) CIDR {
	_, ipNet, err := net.ParseCIDR(c)
	return &cidrPath{c, f, ipNet, err}
}

func (c *cidrPath) ValidateSubset(subsets ...CIDR) field.ErrorList {
	allErrs := field.ErrorList{}
	if c.ParseError != nil {
		return allErrs
	}

	for _, subset := range subsets {
		if subset == nil || c == subset || !subset.Parse() {
			continue
		}

		if !c.net.Contains(subset.GetIPNet().IP) || !c.net.Contains(subset.LastIPInRange()) {
			allErrs = append(allErrs, field.Invalid(subset.GetFieldPath(), subset.GetCIDR(), fmt.Sprintf("must be a subset of %q (%q)", c.fieldPath.String(), c.cidr)))
		}
	}

	return allErrs
}

func (c *cidrPath) ValidateOverlap(subsets ...CIDR) field.ErrorList {
	allErrs := field.ErrorList{}
	if c.ParseError != nil {
		return allErrs
	}

	for _, subset := range subsets {
		if subset == nil || c == subset || !subset.Parse() {
			continue
		}

		// continue if CIDRs overlap.
		if c.net.Contains(subset.GetIPNet().IP) || subset.GetIPNet().Contains(c.net.IP) {
			continue
		}

		allErrs = append(allErrs, field.Invalid(subset.GetFieldPath(), subset.GetCIDR(), fmt.Sprintf("must overlap with %q (%q)", c.fieldPath.String(), c.cidr)))
	}

	return allErrs
}

func (c *cidrPath) ValidateNotOverlap(subsets ...CIDR) field.ErrorList {
	allErrs := field.ErrorList{}
	if c.ParseError != nil {
		return allErrs
	}

	for _, subset := range subsets {
		if subset == nil || c == subset || !subset.Parse() {
			continue
		}

		// continue if CIDRs do not overlap.
		if !c.net.Contains(subset.GetIPNet().IP) && !subset.GetIPNet().Contains(c.net.IP) {
			continue
		}

		allErrs = append(allErrs, field.Invalid(subset.GetFieldPath(), subset.GetCIDR(), fmt.Sprintf("must not overlap with %q (%q)", c.fieldPath.String(), c.cidr)))
	}

	return allErrs
}

func (c *cidrPath) ValidateParse() field.ErrorList {
	allErrs := field.ErrorList{}

	if c.ParseError != nil {
		allErrs = append(allErrs, field.Invalid(c.fieldPath, c.cidr, c.ParseError.Error()))
	}

	return allErrs
}

// ValidateIPFamily returns error if IPFamily does not match CIDR.
func (c *cidrPath) ValidateIPFamily(ipFamily string) field.ErrorList {
	allErrs := field.ErrorList{}

	if c.ParseError != nil {
		return allErrs
	}

	switch ipFamily {
	case IPFamilyIPv4:
		if c.net.IP.To4() == nil {
			allErrs = append(allErrs, field.Invalid(c.fieldPath, c.net.IP.String(), "must be a valid IPv4 address"))
		}
	case IPFamilyIPv6:
		if c.net.IP.To4() != nil {
			allErrs = append(allErrs, field.Invalid(c.fieldPath, c.net.IP.String(), "must be a valid IPv6 address"))
		}
	}

	return allErrs
}

// ValidateMaxSize returns an error if CIDR size is larger than given bits. e.g. /15 is larger than /16
func (c *cidrPath) ValidateMaxSize(bits int) field.ErrorList {
	allErrs := field.ErrorList{}

	if c.ParseError != nil {
		return allErrs
	}
	cidrBits, _ := c.net.Mask.Size()

	if cidrBits < bits {
		allErrs = append(allErrs, field.Invalid(c.fieldPath, c.net.String(), fmt.Sprintf("cannot be larger than /%d", bits)))
	}

	return allErrs
}

func (c *cidrPath) Parse() (success bool) {
	return c.ParseError == nil
}

func (c *cidrPath) GetIPNet() *net.IPNet {
	return c.net
}

func (c *cidrPath) GetFieldPath() *field.Path {
	return c.fieldPath
}

func (c *cidrPath) GetCIDR() string {
	return c.cidr
}

func (c *cidrPath) LastIPInRange() net.IP {
	var buf, res net.IP

	for _, b := range c.GetIPNet().Mask {
		buf = append(buf, ^b)
	}

	for i := range c.GetIPNet().IP {
		res = append(res, c.GetIPNet().IP[i]|buf[i])
	}

	return res
}

func (c *cidrPath) IsIPv4() bool {
	if c.ParseError == nil && len(c.ValidateIPFamily(IPFamilyIPv4)) == 0 {
		return true
	}
	return false
}

func (c *cidrPath) IsIPv6() bool {
	if c.ParseError == nil && len(c.ValidateIPFamily(IPFamilyIPv6)) == 0 {
		return true
	}
	return false
}

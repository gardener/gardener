// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"fmt"
	"net/netip"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/api/core/helper"
	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencoreinstall "github.com/gardener/gardener/pkg/apis/core/install"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var (
	gardenCoreScheme = runtime.NewScheme()

	shootAllowedNodesCIDRIPv4 = netip.MustParsePrefix("10.0.0.0/16")
	// Technically, there are other CIDRs that are subnets of the kind pod network (fd00:10:1::/48) that don't overlap
	// with the seed pod CIDR (fd00:10:1::/56), but we restrict the allowed CIDRs to a single /56 prefix for simplicity.
	shootAllowedNodesCIDRIPv6 = netip.MustParsePrefix("fd00:10:1:100::/56")

	// For self-hosted shoots without managed infrastructure (GinD), nodes are plain Docker containers on the Docker
	// bridge network instead of KinD pods, so they explicitly use the CIDR range of the kind Docker network.
	shootAllowedNodesCIDRIPv4GinD = netip.MustParsePrefix("172.18.0.0/24")
	shootAllowedNodesCIDRIPv6GinD = netip.MustParsePrefix("fd00:10::/64")
)

func init() {
	utilruntime.Must(gardencoreinstall.AddToScheme(gardenCoreScheme))
}

type shootValidator struct{}

// NewShootValidator returns a new instance of a Shoot validator.
func NewShootValidator() extensionswebhook.Validator {
	return &shootValidator{}
}

// Validate validates the given Shoot object.
func (s *shootValidator) Validate(_ context.Context, newObj, oldObj client.Object) error {
	newShoot, ok := newObj.(*core.Shoot)
	if !ok {
		return fmt.Errorf("expected Shoot, but got %T", newObj)
	}
	var oldShoot *core.Shoot
	if oldObj != nil {
		oldShoot, ok = oldObj.(*core.Shoot)
		if !ok {
			return fmt.Errorf("expected Shoot, but got %T", oldObj)
		}
	}

	if newShoot.DeletionTimestamp != nil {
		return nil
	}

	allErrs := field.ErrorList{}
	if !helper.IsWorkerless(newShoot) {
		allErrs = append(allErrs, s.ValidateShootNodesCIDR(newShoot, oldShoot)...)
	}

	return allErrs.ToAggregate()
}

func (s *shootValidator) ValidateShootNodesCIDR(newShoot, oldShoot *core.Shoot) field.ErrorList {
	allErrs := field.ErrorList{}

	// only validate shoot nodes CIDR on creation or if it is changed
	if oldShoot != nil && apiequality.Semantic.DeepEqual(oldShoot.Spec.Networking.Nodes, newShoot.Spec.Networking.Nodes) {
		return allErrs
	}

	nodesPath := field.NewPath("spec.networking.nodes")
	if newShoot.Spec.Networking.Nodes == nil {
		allErrs = append(allErrs, field.Required(nodesPath, "nodes CIDR must not be empty"))
		return allErrs
	}

	cidr, err := netip.ParsePrefix(*newShoot.Spec.Networking.Nodes)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(nodesPath, *newShoot.Spec.Networking.Nodes, "nodes CIDR must be a valid CIDR"))
		return allErrs
	}

	allowedIPv4, allowedIPv6 := shootAllowedNodesCIDRIPv4, shootAllowedNodesCIDRIPv6
	if s.isSelfHostedWithoutManagedInfrastructure(newShoot) {
		allowedIPv4, allowedIPv6 = shootAllowedNodesCIDRIPv4GinD, shootAllowedNodesCIDRIPv6GinD
	}

	if cidr.Addr().Is4() {
		if !isSubnet(allowedIPv4, cidr) {
			allErrs = append(allErrs, field.Invalid(nodesPath, *newShoot.Spec.Networking.Nodes, fmt.Sprintf("nodes CIDR must be a subnet of %s", allowedIPv4.String())))
		}
	} else {
		if !isSubnet(allowedIPv6, cidr) {
			allErrs = append(allErrs, field.Invalid(nodesPath, *newShoot.Spec.Networking.Nodes, fmt.Sprintf("nodes CIDR must be a subnet of %s", allowedIPv6.String())))
		}
	}

	return allErrs
}

func (s *shootValidator) isSelfHostedWithoutManagedInfrastructure(shoot *core.Shoot) bool {
	v1beta1Shoot := &gardencorev1beta1.Shoot{}
	if err := gardenCoreScheme.Convert(shoot, v1beta1Shoot, nil); err != nil {
		return false
	}
	return v1beta1helper.IsShootSelfHosted(v1beta1Shoot.Spec.Provider.Workers) && !v1beta1helper.HasManagedInfrastructure(v1beta1Shoot)
}

// isSubnet returns true if sub is a subnet of net. Both net and sub must be valid netip.Prefix.
// The function assumes that net and sub are of the same IP family (both IPv4 or both IPv6).
func isSubnet(net, sub netip.Prefix) bool {
	// A is a subnet of B if:
	// - B's prefix length is shorter or equal to A's (B is a broader network).
	// - B contains the canonical starting IP address of A.
	return net.Bits() <= sub.Bits() && net.Contains(sub.Masked().Addr())
}

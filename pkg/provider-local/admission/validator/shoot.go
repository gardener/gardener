// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"fmt"
	"net/netip"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
)

// We require that local shoots specify a nodes CIDR within the following ranges. This ensures that:
//   - the nodes CIDR is a subnet of the kind pod network (cluster CIDR) so that machine pods get IPs from the pod
//     network and are treated as internal traffic by kube-proxy
//   - the nodes CIDR is disjoint with the seed/runtime pod CIDR (calico default IPPool)
var (
	shootAllowedNodesCIDRIPv4 = netip.MustParsePrefix("10.0.0.0/16")
	// Technically, there are other CIDRs that are subnets of the kind pod network (fd00:10:1::/48) that don't overlap
	// with the seed pod CIDR (fd00:10:1::/56), but we restrict the allowed CIDRs to a single /56 prefix for simplicity.
	shootAllowedNodesCIDRIPv6 = netip.MustParsePrefix("fd00:10:1:100::/56")
)

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

	if cidr.Addr().Is4() {
		if !shootAllowedNodesCIDRIPv4.Contains(cidr.Addr()) {
			allErrs = append(allErrs, field.Invalid(nodesPath, *newShoot.Spec.Networking.Nodes, fmt.Sprintf("nodes CIDR must be a subnet of %s", shootAllowedNodesCIDRIPv4.String())))
		}
	} else {
		if !shootAllowedNodesCIDRIPv6.Contains(cidr.Addr()) {
			allErrs = append(allErrs, field.Invalid(nodesPath, *newShoot.Spec.Networking.Nodes, fmt.Sprintf("nodes CIDR must be a subnet of %s", shootAllowedNodesCIDRIPv6.String())))
		}
	}

	return allErrs
}

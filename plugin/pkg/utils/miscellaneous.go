// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"
)

// SkipVerification is a common function to skip object verification during admission
func SkipVerification(operation admission.Operation, metadata metav1.ObjectMeta) bool {
	return operation == admission.Update && metadata.DeletionTimestamp != nil
}

// ListShootsUsingSeed lists all shoots referencing the given seed name.
func ListShootsUsingSeed(seedName string, shootLister gardencorev1beta1listers.ShootLister) ([]*gardencorev1beta1.Shoot, error) {
	allShoots, err := shootLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	var shoots []*gardencorev1beta1.Shoot
	for _, shoot := range allShoots {
		if IsSeedUsedByShoot(seedName, shoot) {
			shoots = append(shoots, shoot)
		}
	}
	return shoots, nil
}

// IsSeedUsedByAnyShoot checks whether there is any shoot referencing the given seed name.
func IsSeedUsedByAnyShoot(seedName string, shootLister gardencorev1beta1listers.ShootLister) (bool, error) {
	shoots, err := ListShootsUsingSeed(seedName, shootLister)
	if err != nil {
		return false, err
	}

	return len(shoots) > 0, nil
}

// IsSeedUsedByShoot checks whether the shoot references the given seed name.
func IsSeedUsedByShoot(seedName string, shoot *gardencorev1beta1.Shoot) bool {
	if shoot.Spec.SeedName != nil && *shoot.Spec.SeedName == seedName {
		return true
	}
	if shoot.Status.SeedName != nil && *shoot.Status.SeedName == seedName {
		return true
	}
	return false
}

// NewAttributesWithName returns admission.Attributes with the given name and all other attributes kept same.
func NewAttributesWithName(a admission.Attributes, name string) admission.Attributes {
	return admission.NewAttributesRecord(a.GetObject(),
		a.GetOldObject(),
		a.GetKind(),
		a.GetNamespace(),
		name,
		a.GetResource(),
		a.GetSubresource(),
		a.GetOperation(),
		a.GetOperationOptions(),
		a.IsDryRun(),
		a.GetUserInfo())
}

// ValidateZoneRemovalFromSeeds returns an error when zones are removed from the old seed while it is still in use by
// shoots.
func ValidateZoneRemovalFromSeeds(oldSeedSpec, newSeedSpec *core.SeedSpec, seedName string, shootLister gardencorev1beta1listers.ShootLister, kind string) error {
	if removedZones := sets.New(oldSeedSpec.Provider.Zones...).Difference(sets.New(newSeedSpec.Provider.Zones...)); removedZones.Len() > 0 {
		if isUsed, err := IsSeedUsedByAnyShoot(seedName, shootLister); err != nil {
			return apierrors.NewInternalError(err)
		} else if isUsed {
			return apierrors.NewForbidden(core.Resource(kind), seedName, fmt.Errorf("cannot remove zones %v from %s %s as there are Shoots scheduled to this Seed", sets.List(removedZones), kind, seedName))
		}
	}

	return nil
}

// ValidateSeedNetworksUpdateWithShoots returns an error when seed networks are changed so that they are incompatible
// with shoots scheduled on this seed.
func ValidateSeedNetworksUpdateWithShoots(oldSeedSpec, newSeedSpec *core.SeedSpec, seedName string, shootLister gardencorev1beta1listers.ShootLister, kind string) error {
	// for now, we only need to validate changes to the VPN network, the other seed networks are immutable.
	if apiequality.Semantic.DeepEqual(oldSeedSpec.Networks.VPN, newSeedSpec.Networks.VPN) {
		return nil
	}

	shoots, err := ListShootsUsingSeed(seedName, shootLister)
	if err != nil {
		return apierrors.NewInternalError(err)
	}

	for _, shoot := range shoots {
		if allErrs := cidrvalidation.ValidateNetworkDisjointedness(
			nil,
			shoot.Spec.Networking.Nodes,
			shoot.Spec.Networking.Pods,
			shoot.Spec.Networking.Services,
			newSeedSpec.Networks.Nodes,
			newSeedSpec.Networks.VPN,
			newSeedSpec.Networks.Pods,
			newSeedSpec.Networks.Services,
			v1beta1helper.IsWorkerless(shoot),
		); len(allErrs) > 0 {
			return apierrors.NewForbidden(core.Resource(kind), seedName, fmt.Errorf("cannot update networks of %s %s as they overlap with Shoot %s: %v", kind, seedName, client.ObjectKeyFromObject(shoot), allErrs.ToAggregate()))
		}
	}

	return nil
}

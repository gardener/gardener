// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
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

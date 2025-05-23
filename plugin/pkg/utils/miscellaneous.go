// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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

// IsSeedUsedByShoot checks whether there is a shoot cluster referencing the provided seed name
func IsSeedUsedByShoot(seedName string, shoots []*gardencorev1beta1.Shoot) bool {
	for _, shoot := range shoots {
		if shoot.Spec.SeedName != nil && *shoot.Spec.SeedName == seedName {
			return true
		}
		if shoot.Status.SeedName != nil && *shoot.Status.SeedName == seedName {
			return true
		}
	}
	return false
}

// GetFilteredShootList returns shoots returned by the shootLister filtered via the predicateFn.
func GetFilteredShootList(shootLister gardencorev1beta1listers.ShootLister, predicateFn func(*gardencorev1beta1.Shoot) bool) ([]*gardencorev1beta1.Shoot, error) {
	var matchingShoots []*gardencorev1beta1.Shoot

	shoots, err := shootLister.List(labels.Everything())
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to list shoots: %w", err))
	}

	for _, shoot := range shoots {
		if predicateFn(shoot) {
			matchingShoots = append(matchingShoots, shoot)
		}
	}
	return matchingShoots, nil
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
		shoots, err := shootLister.List(labels.Everything())
		if err != nil {
			return err
		}

		if IsSeedUsedByShoot(seedName, shoots) {
			return apierrors.NewForbidden(core.Resource(kind), seedName, fmt.Errorf("cannot remove zones %v from %s %s as there are Shoots scheduled to this Seed", sets.List(removedZones), kind, seedName))
		}
	}

	return nil
}

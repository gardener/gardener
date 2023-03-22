// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/internalversion"
)

// SkipVerification is a common function to skip object verification during admission
func SkipVerification(operation admission.Operation, metadata metav1.ObjectMeta) bool {
	return operation == admission.Update && metadata.DeletionTimestamp != nil
}

// IsSeedUsedByShoot checks whether there is a shoot cluster referencing the provided seed name
func IsSeedUsedByShoot(seedName string, shoots []*core.Shoot) bool {
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
func GetFilteredShootList(shootLister gardencorelisters.ShootLister, predicateFn func(*core.Shoot) bool) ([]*core.Shoot, error) {
	var matchingShoots []*core.Shoot
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
func ValidateZoneRemovalFromSeeds(oldSeedSpec, newSeedSpec *core.SeedSpec, seedName string, shootLister gardencorelisters.ShootLister, kind string) error {
	if removedZones := sets.New[string](oldSeedSpec.Provider.Zones...).Difference(sets.New[string](newSeedSpec.Provider.Zones...)); removedZones.Len() > 0 {
		shootList, err := GetFilteredShootList(shootLister, func(shoot *core.Shoot) bool {
			return pointer.StringDeref(shoot.Spec.SeedName, "") == seedName
		})
		if err != nil {
			return err
		}

		if len(shootList) > 0 {
			return apierrors.NewForbidden(core.Resource(kind), seedName, fmt.Errorf("cannot remove zones %v from %s %s as there are %d Shoots scheduled to this Seed", sets.List(removedZones), kind, seedName, len(shootList)))
		}
	}

	return nil
}

// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seed

import (
	"context"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	"github.com/gardener/gardener/pkg/apis/core/validation"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage/names"
)

// Strategy defines the strategy for storing seeds.
type Strategy struct {
	runtime.ObjectTyper
	names.NameGenerator

	CloudProfiles rest.StandardStorage
}

// NewStrategy defines the storage strategy for Seeds.
func NewStrategy(cloudProfiles rest.StandardStorage) Strategy {
	return Strategy{api.Scheme, names.SimpleNameGenerator, cloudProfiles}
}

// NamespaceScoped returns true if the object must be within a namespace.
func (Strategy) NamespaceScoped() bool {
	return false
}

// PrepareForCreate mutates some fields in the object before it's created.
func (s Strategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	seed := obj.(*core.Seed)

	seed.Generation = 1
	seed.Status = core.SeedStatus{}
}

// PrepareForUpdate is invoked on update before validation to normalize
// the object.  For example: remove fields that are not to be persisted,
// sort order-insensitive list fields, etc.  This should not remove fields
// whose presence would be considered a validation error.
func (s Strategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newSeed := obj.(*core.Seed)
	oldSeed := old.(*core.Seed)
	newSeed.Status = oldSeed.Status

	if !apiequality.Semantic.DeepEqual(oldSeed.Spec, newSeed.Spec) {
		newSeed.Generation = oldSeed.Generation + 1
	}

	migrateSettings(newSeed, oldSeed)
}

// Validate validates the given object.
func (Strategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	seed := obj.(*core.Seed)
	return validation.ValidateSeed(seed)
}

// Canonicalize allows an object to be mutated into a canonical form. This
// ensures that code that operates on these objects can rely on the common
// form for things like comparison.  Canonicalize is invoked after
// validation has succeeded but before the object has been persisted.
// This method may mutate the object.
func (Strategy) Canonicalize(obj runtime.Object) {
}

// AllowCreateOnUpdate returns true if the object can be created by a PUT.
func (Strategy) AllowCreateOnUpdate() bool {
	return false
}

// AllowUnconditionalUpdate returns true if the object can be updated
// unconditionally (irrespective of the latest resource version), when
// there is no resource version specified in the object.
func (Strategy) AllowUnconditionalUpdate() bool {
	return true
}

// ValidateUpdate validates the update on the given old and new object.
func (Strategy) ValidateUpdate(ctx context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldSeed, newSeed := oldObj.(*core.Seed), newObj.(*core.Seed)
	return validation.ValidateSeedUpdate(newSeed, oldSeed)
}

// StatusStrategy defines the strategy for storing seeds statuses.
type StatusStrategy struct {
	Strategy
}

// NewStatusStrategy defines the storage strategy for the status subresource of Seeds.
func NewStatusStrategy(cloudProfiles rest.StandardStorage) StatusStrategy {
	return StatusStrategy{NewStrategy(cloudProfiles)}
}

// PrepareForUpdate is invoked on update before validation to normalize
// the object.  For example: remove fields that are not to be persisted,
// sort order-insensitive list fields, etc.  This should not remove fields
// whose presence would be considered a validation error.
func (s StatusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newSeed := obj.(*core.Seed)
	oldSeed := old.(*core.Seed)
	newSeed.Spec = oldSeed.Spec
}

// ValidateUpdate validates the update on the given old and new object.
func (StatusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateSeedStatusUpdate(obj.(*core.Seed), old.(*core.Seed))
}

func migrateSettings(newSeed, oldSeed *core.Seed) {
	var (
		taintsToAdd    []string
		taintsToRemove []string
	)

	// excess capacity reservation
	if !helper.TaintsHave(oldSeed.Spec.Taints, core.DeprecatedSeedTaintDisableCapacityReservation) && helper.TaintsHave(newSeed.Spec.Taints, core.DeprecatedSeedTaintDisableCapacityReservation) {
		if newSeed.Spec.Settings == nil {
			newSeed.Spec.Settings = &core.SeedSettings{}
		}
		newSeed.Spec.Settings.ExcessCapacityReservation = &core.SeedSettingExcessCapacityReservation{Enabled: false}
	} else if helper.TaintsHave(oldSeed.Spec.Taints, core.DeprecatedSeedTaintDisableCapacityReservation) && !helper.TaintsHave(newSeed.Spec.Taints, core.DeprecatedSeedTaintDisableCapacityReservation) {
		if newSeed.Spec.Settings == nil {
			newSeed.Spec.Settings = &core.SeedSettings{}
		}
		newSeed.Spec.Settings.ExcessCapacityReservation = &core.SeedSettingExcessCapacityReservation{Enabled: true}
	}

	if !helper.SeedSettingExcessCapacityReservationEnabled(oldSeed.Spec.Settings) && helper.SeedSettingExcessCapacityReservationEnabled(newSeed.Spec.Settings) {
		taintsToRemove = append(taintsToRemove, core.DeprecatedSeedTaintDisableCapacityReservation)
	} else if helper.SeedSettingExcessCapacityReservationEnabled(oldSeed.Spec.Settings) && !helper.SeedSettingExcessCapacityReservationEnabled(newSeed.Spec.Settings) {
		taintsToAdd = append(taintsToAdd, core.DeprecatedSeedTaintDisableCapacityReservation)
	}

	// scheduling visibility
	if !helper.TaintsHave(oldSeed.Spec.Taints, core.DeprecatedSeedTaintInvisible) && helper.TaintsHave(newSeed.Spec.Taints, core.DeprecatedSeedTaintInvisible) {
		if newSeed.Spec.Settings == nil {
			newSeed.Spec.Settings = &core.SeedSettings{}
		}
		newSeed.Spec.Settings.Scheduling = &core.SeedSettingScheduling{Visible: false}
	} else if helper.TaintsHave(oldSeed.Spec.Taints, core.DeprecatedSeedTaintInvisible) && !helper.TaintsHave(newSeed.Spec.Taints, core.DeprecatedSeedTaintInvisible) {
		if newSeed.Spec.Settings == nil {
			newSeed.Spec.Settings = &core.SeedSettings{}
		}
		newSeed.Spec.Settings.Scheduling = &core.SeedSettingScheduling{Visible: true}
	}

	if !helper.SeedSettingSchedulingVisible(oldSeed.Spec.Settings) && helper.SeedSettingSchedulingVisible(newSeed.Spec.Settings) {
		taintsToRemove = append(taintsToRemove, core.DeprecatedSeedTaintInvisible)
	} else if helper.SeedSettingSchedulingVisible(oldSeed.Spec.Settings) && !helper.SeedSettingSchedulingVisible(newSeed.Spec.Settings) {
		taintsToAdd = append(taintsToAdd, core.DeprecatedSeedTaintInvisible)
	}

	// disable dns
	if !helper.TaintsHave(oldSeed.Spec.Taints, core.DeprecatedSeedTaintDisableDNS) && helper.TaintsHave(newSeed.Spec.Taints, core.DeprecatedSeedTaintDisableDNS) {
		if newSeed.Spec.Settings == nil {
			newSeed.Spec.Settings = &core.SeedSettings{}
		}
		newSeed.Spec.Settings.ShootDNS = &core.SeedSettingShootDNS{Enabled: false}
	} else if helper.TaintsHave(oldSeed.Spec.Taints, core.DeprecatedSeedTaintDisableDNS) && !helper.TaintsHave(newSeed.Spec.Taints, core.DeprecatedSeedTaintDisableDNS) {
		if newSeed.Spec.Settings == nil {
			newSeed.Spec.Settings = &core.SeedSettings{}
		}
		newSeed.Spec.Settings.ShootDNS = &core.SeedSettingShootDNS{Enabled: true}
	}

	if !helper.SeedSettingShootDNSEnabled(oldSeed.Spec.Settings) && helper.SeedSettingShootDNSEnabled(newSeed.Spec.Settings) {
		taintsToRemove = append(taintsToRemove, core.DeprecatedSeedTaintDisableDNS)
	} else if helper.SeedSettingShootDNSEnabled(oldSeed.Spec.Settings) && !helper.SeedSettingShootDNSEnabled(newSeed.Spec.Settings) {
		taintsToAdd = append(taintsToAdd, core.DeprecatedSeedTaintDisableDNS)
	}

	// add taints
	for _, taint := range taintsToAdd {
		addTaint := true
		for _, t := range newSeed.Spec.Taints {
			if t.Key == taint {
				addTaint = false
				break
			}
		}
		if addTaint {
			newSeed.Spec.Taints = append(newSeed.Spec.Taints, core.SeedTaint{
				Key: taint,
			})
		}
	}

	// remove taints
	for _, taint := range taintsToRemove {
		for i := len(newSeed.Spec.Taints) - 1; i >= 0; i-- {
			if newSeed.Spec.Taints[i].Key == taint {
				newSeed.Spec.Taints = append(newSeed.Spec.Taints[:i], newSeed.Spec.Taints[i+1:]...)
			}
		}
	}
}

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
	"fmt"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/garden"
	"github.com/gardener/gardener/pkg/apis/garden/helper"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/validation"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
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

// Migrate finds the cloud profile or the provider type. It is needed for migration of old
// `garden.sapcloud.io` and new `core.gardener.cloud` API groups.
func (s Strategy) Migrate(ctx context.Context, obj runtime.Object) error {
	migrate := func(obj runtime.Object) error {
		seed := obj.(*garden.Seed)

		switch {
		case len(seed.Spec.Cloud.Profile) > 0:
			cloudProfileObj, err := s.CloudProfiles.Get(ctx, seed.Spec.Cloud.Profile, &metav1.GetOptions{})
			if err != nil {
				return err
			}
			cloudProfile := cloudProfileObj.(*garden.CloudProfile)

			providerType, err := helper.DetermineCloudProviderInProfile(cloudProfile.Spec)
			if err != nil {
				return err
			}
			seed.Spec.Provider.Type = string(providerType)

		case len(seed.Spec.Provider.Type) > 0:
			cloudProfileListObj, err := s.CloudProfiles.List(ctx, &metainternalversion.ListOptions{})
			if err != nil {
				return err
			}
			cloudProfileList := cloudProfileListObj.(*garden.CloudProfileList)

			for _, cloudProfile := range cloudProfileList.Items {
				providerType, err := helper.DetermineCloudProviderInProfile(cloudProfile.Spec)
				if err != nil {
					return err
				}

				if string(providerType) == seed.Spec.Provider.Type {
					seed.Spec.Cloud.Profile = cloudProfile.Name
					return nil
				}
			}

			return fmt.Errorf("could not find a cloud profile for this provider type %q", seed.Spec.Provider.Type)
		}

		return nil
	}

	if meta.IsListType(obj) {
		return meta.EachListItem(obj, migrate)
	}
	return migrate(obj)
}

// NamespaceScoped returns true if the object must be within a namespace.
func (Strategy) NamespaceScoped() bool {
	return false
}

// PrepareForCreate mutates some fields in the object before it's created.
func (s Strategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	utilruntime.HandleError(s.Migrate(ctx, obj))

	seed := obj.(*garden.Seed)

	seed.Generation = 1
	seed.Status = garden.SeedStatus{}

	finalizers := sets.NewString(seed.Finalizers...)
	if !finalizers.Has(gardenv1beta1.GardenerName) {
		finalizers.Insert(gardenv1beta1.GardenerName)
	}
	seed.Finalizers = finalizers.UnsortedList()
}

// PrepareForUpdate is invoked on update before validation to normalize
// the object.  For example: remove fields that are not to be persisted,
// sort order-insensitive list fields, etc.  This should not remove fields
// whose presence would be considered a validation error.
func (s Strategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	utilruntime.HandleError(s.Migrate(ctx, obj))
	utilruntime.HandleError(s.Migrate(ctx, old))

	newSeed := obj.(*garden.Seed)
	oldSeed := old.(*garden.Seed)
	newSeed.Status = oldSeed.Status

	if !apiequality.Semantic.DeepEqual(oldSeed.Spec, newSeed.Spec) {
		newSeed.Generation = oldSeed.Generation + 1
	}
}

// Validate validates the given object.
func (Strategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	seed := obj.(*garden.Seed)
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
	oldSeed, newSeed := oldObj.(*garden.Seed), newObj.(*garden.Seed)
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
	utilruntime.HandleError(s.Migrate(ctx, obj))
	utilruntime.HandleError(s.Migrate(ctx, old))

	newSeed := obj.(*garden.Seed)
	oldSeed := old.(*garden.Seed)
	newSeed.Spec = oldSeed.Spec
}

// ValidateUpdate validates the update on the given old and new object.
func (StatusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateSeedStatusUpdate(obj.(*garden.Seed), old.(*garden.Seed))
}

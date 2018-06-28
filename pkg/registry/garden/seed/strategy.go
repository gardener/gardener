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
	"github.com/gardener/gardener/pkg/apis/garden"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/validation"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"
)

type seedStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy defines the storage strategy for Seeds.
var Strategy = seedStrategy{api.Scheme, names.SimpleNameGenerator}

func (seedStrategy) NamespaceScoped() bool {
	return false
}

func (seedStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	seed := obj.(*garden.Seed)

	seed.Generation = 1
	seed.Status = garden.SeedStatus{}

	finalizers := sets.NewString(seed.Finalizers...)
	if !finalizers.Has(gardenv1beta1.GardenerName) {
		finalizers.Insert(gardenv1beta1.GardenerName)
	}
	seed.Finalizers = finalizers.UnsortedList()
}

func (seedStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newSeed := obj.(*garden.Seed)
	oldSeed := old.(*garden.Seed)
	newSeed.Status = oldSeed.Status

	if !apiequality.Semantic.DeepEqual(oldSeed.Spec, newSeed.Spec) {
		newSeed.Generation = oldSeed.Generation + 1
	}
}

func (seedStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	seed := obj.(*garden.Seed)
	return validation.ValidateSeed(seed)
}

func (seedStrategy) Canonicalize(obj runtime.Object) {
}

func (seedStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (seedStrategy) AllowUnconditionalUpdate() bool {
	return true
}

func (seedStrategy) ValidateUpdate(ctx context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldSeed, newSeed := oldObj.(*garden.Seed), newObj.(*garden.Seed)
	return validation.ValidateSeedUpdate(newSeed, oldSeed)
}

type seedStatusStrategy struct {
	seedStrategy
}

// StatusStrategy defines the storage strategy for the status subresource of Seeds.
var StatusStrategy = seedStatusStrategy{Strategy}

func (seedStatusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newSeed := obj.(*garden.Seed)
	oldSeed := old.(*garden.Seed)
	newSeed.Spec = oldSeed.Spec
}

func (seedStatusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateSeedStatusUpdate(obj.(*garden.Seed), old.(*garden.Seed))
}

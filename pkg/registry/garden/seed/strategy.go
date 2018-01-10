// Copyright 2018 The Gardener Authors.
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
	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/garden"
	"github.com/gardener/gardener/pkg/apis/garden/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
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

func (seedStrategy) PrepareForCreate(ctx genericapirequest.Context, obj runtime.Object) {
	_ = obj.(*garden.Seed)
}

func (seedStrategy) Validate(ctx genericapirequest.Context, obj runtime.Object) field.ErrorList {
	seed := obj.(*garden.Seed)
	return validation.ValidateSeed(seed)
}

func (seedStrategy) Canonicalize(obj runtime.Object) {
}

func (seedStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (seedStrategy) PrepareForUpdate(ctx genericapirequest.Context, newObj, oldObj runtime.Object) {
	_ = oldObj.(*garden.Seed)
	_ = newObj.(*garden.Seed)
}

func (seedStrategy) AllowUnconditionalUpdate() bool {
	return true
}

func (seedStrategy) ValidateUpdate(ctx genericapirequest.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldSeed, newSeed := oldObj.(*garden.Seed), newObj.(*garden.Seed)
	return validation.ValidateSeedUpdate(newSeed, oldSeed)
}

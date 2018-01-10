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

package crosssecretbinding

import (
	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/garden"
	"github.com/gardener/gardener/pkg/apis/garden/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/storage/names"
)

type crossSecretBindingStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy defines the storage strategy for CrossSecretBindings.
var Strategy = crossSecretBindingStrategy{api.Scheme, names.SimpleNameGenerator}

func (crossSecretBindingStrategy) NamespaceScoped() bool {
	return true
}

func (crossSecretBindingStrategy) PrepareForCreate(ctx genericapirequest.Context, obj runtime.Object) {
	_ = obj.(*garden.CrossSecretBinding)
}

func (crossSecretBindingStrategy) Validate(ctx genericapirequest.Context, obj runtime.Object) field.ErrorList {
	binding := obj.(*garden.CrossSecretBinding)
	return validation.ValidateCrossSecretBinding(binding)
}

func (crossSecretBindingStrategy) Canonicalize(obj runtime.Object) {
}

func (crossSecretBindingStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (crossSecretBindingStrategy) PrepareForUpdate(ctx genericapirequest.Context, newObj, oldObj runtime.Object) {
	_ = oldObj.(*garden.CrossSecretBinding)
	_ = newObj.(*garden.CrossSecretBinding)
}

func (crossSecretBindingStrategy) AllowUnconditionalUpdate() bool {
	return true
}

func (crossSecretBindingStrategy) ValidateUpdate(ctx genericapirequest.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldBinding, newBinding := oldObj.(*garden.CrossSecretBinding), newObj.(*garden.CrossSecretBinding)
	return validation.ValidateCrossSecretBindingUpdate(newBinding, oldBinding)
}

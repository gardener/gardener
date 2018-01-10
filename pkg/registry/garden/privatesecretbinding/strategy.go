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

package privatesecretbinding

import (
	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/garden"
	"github.com/gardener/gardener/pkg/apis/garden/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/storage/names"
)

type privateSecretBindingStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy defines the storage strategy for PrivateSecretBindings.
var Strategy = privateSecretBindingStrategy{api.Scheme, names.SimpleNameGenerator}

func (privateSecretBindingStrategy) NamespaceScoped() bool {
	return true
}

func (privateSecretBindingStrategy) PrepareForCreate(ctx genericapirequest.Context, obj runtime.Object) {
	_ = obj.(*garden.PrivateSecretBinding)
}

func (privateSecretBindingStrategy) Validate(ctx genericapirequest.Context, obj runtime.Object) field.ErrorList {
	binding := obj.(*garden.PrivateSecretBinding)
	return validation.ValidatePrivateSecretBinding(binding)
}

func (privateSecretBindingStrategy) Canonicalize(obj runtime.Object) {
}

func (privateSecretBindingStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (privateSecretBindingStrategy) PrepareForUpdate(ctx genericapirequest.Context, newObj, oldObj runtime.Object) {
	_ = oldObj.(*garden.PrivateSecretBinding)
	_ = newObj.(*garden.PrivateSecretBinding)
}

func (privateSecretBindingStrategy) AllowUnconditionalUpdate() bool {
	return true
}

func (privateSecretBindingStrategy) ValidateUpdate(ctx genericapirequest.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldBinding, newBinding := oldObj.(*garden.PrivateSecretBinding), newObj.(*garden.PrivateSecretBinding)
	return validation.ValidatePrivateSecretBindingUpdate(newBinding, oldBinding)
}

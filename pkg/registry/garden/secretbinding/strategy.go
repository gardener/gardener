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

package secretbinding

import (
	"context"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/garden"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"
)

type secretBindingStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy defines the storage strategy for SecretBindings.
var Strategy = secretBindingStrategy{api.Scheme, names.SimpleNameGenerator}

func (secretBindingStrategy) NamespaceScoped() bool {
	return true
}

func (secretBindingStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	binding := obj.(*garden.SecretBinding)

	finalizers := sets.NewString(binding.Finalizers...)
	if !finalizers.Has(gardenv1beta1.GardenerName) {
		finalizers.Insert(gardenv1beta1.GardenerName)
	}
	binding.Finalizers = finalizers.UnsortedList()
}

func (secretBindingStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	binding := obj.(*garden.SecretBinding)
	return validation.ValidateSecretBinding(binding)
}

func (secretBindingStrategy) Canonicalize(obj runtime.Object) {
}

func (secretBindingStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (secretBindingStrategy) PrepareForUpdate(ctx context.Context, newObj, oldObj runtime.Object) {
	_ = oldObj.(*garden.SecretBinding)
	_ = newObj.(*garden.SecretBinding)
}

func (secretBindingStrategy) AllowUnconditionalUpdate() bool {
	return true
}

func (secretBindingStrategy) ValidateUpdate(ctx context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldBinding, newBinding := oldObj.(*garden.SecretBinding), newObj.(*garden.SecretBinding)
	return validation.ValidateSecretBindingUpdate(newBinding, oldBinding)
}

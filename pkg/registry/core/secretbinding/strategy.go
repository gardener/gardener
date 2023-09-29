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

package secretbinding

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/validation"
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

func (secretBindingStrategy) PrepareForCreate(_ context.Context, _ runtime.Object) {
}

func (secretBindingStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	binding := obj.(*core.SecretBinding)
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validation.ValidateSecretBinding(binding)...)
	allErrs = append(allErrs, validation.ValidateSecretBindingProvider(binding.Provider)...)
	return allErrs
}

func (secretBindingStrategy) Canonicalize(_ runtime.Object) {
}

func (secretBindingStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (secretBindingStrategy) PrepareForUpdate(_ context.Context, newObj, oldObj runtime.Object) {
	_ = oldObj.(*core.SecretBinding)
	_ = newObj.(*core.SecretBinding)
}

func (secretBindingStrategy) AllowUnconditionalUpdate() bool {
	return true
}

func (secretBindingStrategy) ValidateUpdate(_ context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldBinding, newBinding := oldObj.(*core.SecretBinding), newObj.(*core.SecretBinding)
	return validation.ValidateSecretBindingUpdate(newBinding, oldBinding)
}

// WarningsOnCreate returns warnings to the client performing a create.
func (secretBindingStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (secretBindingStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

func (s secretBindingStrategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	binding := obj.(*core.SecretBinding)

	if binding.GetName() == "" {
		binding.SetName(s.GenerateName(binding.GetGenerateName()))
	}
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

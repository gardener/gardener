// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secretbinding

import (
	"context"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/validation"

	"k8s.io/apimachinery/pkg/runtime"
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
}

func (secretBindingStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	binding := obj.(*core.SecretBinding)
	return validation.ValidateSecretBinding(binding)
}

func (secretBindingStrategy) Canonicalize(obj runtime.Object) {
}

func (secretBindingStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (secretBindingStrategy) PrepareForUpdate(ctx context.Context, newObj, oldObj runtime.Object) {
	_ = oldObj.(*core.SecretBinding)
	_ = newObj.(*core.SecretBinding)
}

func (secretBindingStrategy) AllowUnconditionalUpdate() bool {
	return true
}

func (secretBindingStrategy) ValidateUpdate(ctx context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldBinding, newBinding := oldObj.(*core.SecretBinding), newObj.(*core.SecretBinding)
	return validation.ValidateSecretBindingUpdate(newBinding, oldBinding)
}

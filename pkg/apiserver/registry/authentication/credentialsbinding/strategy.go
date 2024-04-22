// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package credentialsbinding

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/authentication"
	"github.com/gardener/gardener/pkg/apis/authentication/validation"
)

type credentialsBindingStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy defines the storage strategy for CredentialsBindings.
var Strategy = credentialsBindingStrategy{api.Scheme, names.SimpleNameGenerator}

func (credentialsBindingStrategy) NamespaceScoped() bool {
	return true
}

func (credentialsBindingStrategy) PrepareForCreate(_ context.Context, _ runtime.Object) {

}

func (credentialsBindingStrategy) PrepareForUpdate(_ context.Context, _, _ runtime.Object) {

}

func (credentialsBindingStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	credentialsbinding := obj.(*authentication.CredentialsBinding)
	return validation.ValidateCredentialsBinding(credentialsbinding)
}

func (credentialsBindingStrategy) Canonicalize(_ runtime.Object) {
}

func (credentialsBindingStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (credentialsBindingStrategy) ValidateUpdate(_ context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	newCredentialsBinding := newObj.(*authentication.CredentialsBinding)
	oldCredentialsBinding := oldObj.(*authentication.CredentialsBinding)
	return validation.ValidateCredentialsBindingUpdate(newCredentialsBinding, oldCredentialsBinding)
}

func (credentialsBindingStrategy) AllowUnconditionalUpdate() bool {
	return true
}

// WarningsOnCreate returns warnings to the client performing a create.
func (credentialsBindingStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (credentialsBindingStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

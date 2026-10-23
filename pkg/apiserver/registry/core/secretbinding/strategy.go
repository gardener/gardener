// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package secretbinding

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/api/core/validation"
	"github.com/gardener/gardener/pkg/apis/core"
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

func (secretBindingStrategy) AllowCreateOnUpdate(_ context.Context) bool {
	return false
}

func (secretBindingStrategy) PrepareForUpdate(_ context.Context, newObj, oldObj runtime.Object) {
	_ = oldObj.(*core.SecretBinding)
	_ = newObj.(*core.SecretBinding)
}

func (secretBindingStrategy) AllowUnconditionalUpdate(_ context.Context) bool {
	return true
}

func (secretBindingStrategy) ValidateUpdate(_ context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldBinding, newBinding := oldObj.(*core.SecretBinding), newObj.(*core.SecretBinding)
	return validation.ValidateSecretBindingUpdate(newBinding, oldBinding)
}

const secretBindingDeprecationWarning = "SecretBinding is deprecated in favour of CredentialsBinding." +
	" For migration instructions, see: https://github.com/gardener/gardener/blob/master/docs/usage/shoot-operations/secretbinding-to-credentialsbinding-migration.md"

// WarningsOnCreate returns warnings to the client performing a create.
func (secretBindingStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return []string{secretBindingDeprecationWarning}
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (secretBindingStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return []string{secretBindingDeprecationWarning}
}

// ToSelectableFields returns a field set that represents the object.
func ToSelectableFields(secretBinding *core.SecretBinding) fields.Set {
	// The purpose of allocation with a given number of elements is to reduce
	// amount of allocations needed to create the fields.Set. If you add any
	// field here or the number of object-meta related fields changes, this should
	// be adjusted.
	secretBindingSpecificFieldsSet := make(fields.Set, 2)
	secretBindingSpecificFieldsSet[core.SecretBindingSecretRefName] = secretBinding.SecretRef.Name
	secretBindingSpecificFieldsSet[core.SecretBindingSecretRefNamespace] = secretBinding.SecretRef.Namespace
	return generic.AddObjectMetaFieldsSet(secretBindingSpecificFieldsSet, &secretBinding.ObjectMeta, true)
}

// GetAttrs returns labels and fields of a given object for filtering purposes.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	secretBinding, ok := obj.(*core.SecretBinding)
	if !ok {
		return nil, nil, fmt.Errorf("not a SecretBinding")
	}
	return secretBinding.Labels, ToSelectableFields(secretBinding), nil
}

// MatchSecretBinding returns a generic matcher for a given label and field selector.
func MatchSecretBinding(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}

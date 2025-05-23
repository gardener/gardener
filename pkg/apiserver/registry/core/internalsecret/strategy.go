// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package internalsecret

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/validation"
)

// strategy implements behavior for InternalSecret objects
type strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy is the default logic that applies when creating and updating InternalSecret
// objects via the REST API.
var Strategy = strategy{api.Scheme, names.SimpleNameGenerator}

var _ = rest.RESTCreateStrategy(Strategy)

var _ = rest.RESTUpdateStrategy(Strategy)

func (strategy) NamespaceScoped() bool {
	return true
}

func (strategy) PrepareForCreate(_ context.Context, _ runtime.Object) {
}

func (strategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateInternalSecret(obj.(*core.InternalSecret))
}

// WarningsOnCreate returns warnings for the creation of the given object.
func (strategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string { return nil }

func (strategy) Canonicalize(_ runtime.Object) {
}

func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newInternalSecret := obj.(*core.InternalSecret)
	oldInternalSecret := old.(*core.InternalSecret)

	// this is weird, but consistent with what the validatedUpdate function used to do.
	if len(newInternalSecret.Type) == 0 {
		newInternalSecret.Type = oldInternalSecret.Type
	}
}

func (strategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateInternalSecretUpdate(obj.(*core.InternalSecret), old.(*core.InternalSecret))
}

// WarningsOnUpdate returns warnings for the given update.
func (strategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

func (strategy) AllowUnconditionalUpdate() bool {
	return true
}

// GetAttrs returns labels and fields of a given object for filtering purposes.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	secret, ok := obj.(*core.InternalSecret)
	if !ok {
		return nil, nil, fmt.Errorf("not an InternalSecret")
	}
	return labels.Set(secret.Labels), ToSelectableFields(secret), nil
}

// MatchInternalSecret returns a generic matcher for a given label and field selector.
func MatchInternalSecret(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:       label,
		Field:       field,
		GetAttrs:    GetAttrs,
		IndexFields: []string{core.InternalSecretType},
	}
}

// TypeTriggerFunc returns type of given object.
func TypeTriggerFunc(obj runtime.Object) string {
	secret, ok := obj.(*core.InternalSecret)
	if !ok {
		return ""
	}

	return getType(secret)
}

// ToSelectableFields returns a field set that represents the object
func ToSelectableFields(obj *core.InternalSecret) fields.Set {
	secretSpecificFieldsSet := fields.Set{
		"type": getType(obj),
	}
	return generic.AddObjectMetaFieldsSet(secretSpecificFieldsSet, &obj.ObjectMeta, true)
}

func getType(secret *core.InternalSecret) string {
	return string(secret.Type)
}

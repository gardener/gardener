// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

func (strategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	secret := obj.(*core.InternalSecret)
	dropDisabledFields(secret, nil)
}

func (strategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateInternalSecret(obj.(*core.InternalSecret))
}

// WarningsOnCreate returns warnings for the creation of the given object.
func (strategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string { return nil }

func (strategy) Canonicalize(obj runtime.Object) {
}

func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newInternalSecret := obj.(*core.InternalSecret)
	oldInternalSecret := old.(*core.InternalSecret)

	// this is weird, but consistent with what the validatedUpdate function used to do.
	if len(newInternalSecret.Type) == 0 {
		newInternalSecret.Type = oldInternalSecret.Type
	}

	dropDisabledFields(newInternalSecret, oldInternalSecret)
}

func (strategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateInternalSecretUpdate(obj.(*core.InternalSecret), old.(*core.InternalSecret))
}

// WarningsOnUpdate returns warnings for the given update.
func (strategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
}

func dropDisabledFields(secret *core.InternalSecret, oldInternalSecret *core.InternalSecret) {
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

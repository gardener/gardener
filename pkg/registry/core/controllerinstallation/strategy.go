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

package controllerinstallation

import (
	"context"
	"fmt"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/validation"
)

type controllerInstallationStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy defines the storage strategy for ControllerInstallations.
var Strategy = controllerInstallationStrategy{api.Scheme, names.SimpleNameGenerator}

func (controllerInstallationStrategy) NamespaceScoped() bool {
	return false
}

func (controllerInstallationStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	controllerInstallation := obj.(*core.ControllerInstallation)

	controllerInstallation.Generation = 1
	controllerInstallation.Status = core.ControllerInstallationStatus{}
}

func (controllerInstallationStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newControllerInstallation := obj.(*core.ControllerInstallation)
	oldControllerInstallation := old.(*core.ControllerInstallation)
	newControllerInstallation.Status = oldControllerInstallation.Status

	if mustIncreaseGeneration(oldControllerInstallation, newControllerInstallation) {
		newControllerInstallation.Generation = oldControllerInstallation.Generation + 1
	}
}

func mustIncreaseGeneration(oldControllerInstallation, newControllerInstallation *core.ControllerInstallation) bool {
	// The ControllerInstallation specification changes.
	if !apiequality.Semantic.DeepEqual(oldControllerInstallation.Spec, newControllerInstallation.Spec) {
		return true
	}

	// The deletion timestamp was set.
	if oldControllerInstallation.DeletionTimestamp == nil && newControllerInstallation.DeletionTimestamp != nil {
		return true
	}

	return false
}

func (controllerInstallationStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	controllerInstallation := obj.(*core.ControllerInstallation)
	return validation.ValidateControllerInstallation(controllerInstallation)
}

func (controllerInstallationStrategy) Canonicalize(obj runtime.Object) {
}

func (controllerInstallationStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (controllerInstallationStrategy) ValidateUpdate(ctx context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	newControllerInstallation := newObj.(*core.ControllerInstallation)
	oldControllerInstallation := oldObj.(*core.ControllerInstallation)
	return validation.ValidateControllerInstallationUpdate(newControllerInstallation, oldControllerInstallation)
}

func (controllerInstallationStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// WarningsOnCreate returns warnings to the client performing a create.
func (controllerInstallationStrategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (controllerInstallationStrategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
}

type controllerInstallationStatusStrategy struct {
	controllerInstallationStrategy
}

// StatusStrategy defines the storage strategy for the status subresource of ControllerInstallations.
var StatusStrategy = controllerInstallationStatusStrategy{Strategy}

func (controllerInstallationStatusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newControllerInstallation := obj.(*core.ControllerInstallation)
	oldControllerInstallation := old.(*core.ControllerInstallation)
	newControllerInstallation.Spec = oldControllerInstallation.Spec
}

func (controllerInstallationStatusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateControllerInstallationStatusUpdate(obj.(*core.ControllerInstallation).Status, old.(*core.ControllerInstallation).Status)
}

// ToSelectableFields returns a field set that represents the object
func ToSelectableFields(controllerInstallation *core.ControllerInstallation) fields.Set {
	// The purpose of allocation with a given number of elements is to reduce
	// amount of allocations needed to create the fields.Set. If you add any
	// field here or the number of object-meta related fields changes, this should
	// be adjusted.
	controllerInstallationSpecificFieldsSet := make(fields.Set, 3)
	controllerInstallationSpecificFieldsSet[core.RegistrationRefName] = controllerInstallation.Spec.RegistrationRef.Name
	controllerInstallationSpecificFieldsSet[core.SeedRefName] = controllerInstallation.Spec.SeedRef.Name
	return generic.AddObjectMetaFieldsSet(controllerInstallationSpecificFieldsSet, &controllerInstallation.ObjectMeta, false)
}

// GetAttrs returns labels and fields of a given object for filtering purposes.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	controllerInstallation, ok := obj.(*core.ControllerInstallation)
	if !ok {
		return nil, nil, fmt.Errorf("not a ControllerInstallation")
	}
	return controllerInstallation.ObjectMeta.Labels, ToSelectableFields(controllerInstallation), nil
}

// MatchControllerInstallation returns a generic matcher for a given label and field selector.
func MatchControllerInstallation(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:       label,
		Field:       field,
		GetAttrs:    GetAttrs,
		IndexFields: []string{core.SeedRefName, core.RegistrationRefName},
	}
}

// SeedRefNameIndexFunc returns spec.seedRef.name of given ControllerInstallation.
func SeedRefNameIndexFunc(obj interface{}) ([]string, error) {
	controllerInstallation, ok := obj.(*core.ControllerInstallation)
	if !ok {
		return nil, fmt.Errorf("expected *core.ControllerInstallation but got %T", obj)
	}

	return []string{controllerInstallation.Spec.SeedRef.Name}, nil
}

// RegistrationRefNameIndexFunc returns spec.registrationRef.name of given ControllerInstallation.
func RegistrationRefNameIndexFunc(obj interface{}) ([]string, error) {
	controllerInstallation, ok := obj.(*core.ControllerInstallation)
	if !ok {
		return nil, fmt.Errorf("expected *core.ControllerInstallation but got %T", obj)
	}

	return []string{controllerInstallation.Spec.RegistrationRef.Name}, nil
}

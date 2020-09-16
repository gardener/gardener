// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerregistration

import (
	"context"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/validation"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"
)

type controllerRegistrationStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy defines the storage strategy for ControllerRegistrations.
var Strategy = controllerRegistrationStrategy{api.Scheme, names.SimpleNameGenerator}

func (controllerRegistrationStrategy) NamespaceScoped() bool {
	return false
}

func (controllerRegistrationStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	controllerRegistration := obj.(*core.ControllerRegistration)

	controllerRegistration.Generation = 1
}

func (controllerRegistrationStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newControllerRegistration := obj.(*core.ControllerRegistration)
	oldControllerRegistration := old.(*core.ControllerRegistration)

	if mustIncreaseGeneration(oldControllerRegistration, newControllerRegistration) {
		newControllerRegistration.Generation = oldControllerRegistration.Generation + 1
	}
}

func mustIncreaseGeneration(oldControllerRegistration, newControllerRegistration *core.ControllerRegistration) bool {
	// The ControllerRegistration specification changes.
	if !apiequality.Semantic.DeepEqual(oldControllerRegistration.Spec, newControllerRegistration.Spec) {
		return true
	}

	// The deletion timestamp was set.
	if oldControllerRegistration.DeletionTimestamp == nil && newControllerRegistration.DeletionTimestamp != nil {
		return true
	}

	return false
}

func (controllerRegistrationStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	controllerRegistration := obj.(*core.ControllerRegistration)
	return validation.ValidateControllerRegistration(controllerRegistration)
}

func (controllerRegistrationStrategy) Canonicalize(obj runtime.Object) {
}

func (controllerRegistrationStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (controllerRegistrationStrategy) ValidateUpdate(ctx context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	newControllerRegistration := newObj.(*core.ControllerRegistration)
	oldControllerRegistration := oldObj.(*core.ControllerRegistration)
	return validation.ValidateControllerRegistrationUpdate(newControllerRegistration, oldControllerRegistration)
}

func (controllerRegistrationStrategy) AllowUnconditionalUpdate() bool {
	return false
}

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

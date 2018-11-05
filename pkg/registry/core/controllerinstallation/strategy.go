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

package controllerinstallation

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

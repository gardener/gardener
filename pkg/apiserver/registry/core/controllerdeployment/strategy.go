// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerdeployment

import (
	"context"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/validation"
)

type controllerDeploymentStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy defines the storage strategy for ControllerDeployments.
var Strategy = controllerDeploymentStrategy{api.Scheme, names.SimpleNameGenerator}

func (controllerDeploymentStrategy) NamespaceScoped() bool {
	return false
}

func (controllerDeploymentStrategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	controllerDeployment := obj.(*core.ControllerDeployment)

	controllerDeployment.Generation = 1
}

func (controllerDeploymentStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newControllerDeployment := obj.(*core.ControllerDeployment)
	oldControllerDeployment := old.(*core.ControllerDeployment)

	if mustIncreaseGeneration(oldControllerDeployment, newControllerDeployment) {
		newControllerDeployment.Generation = oldControllerDeployment.Generation + 1
	}
}

func mustIncreaseGeneration(oldControllerDeployment, newControllerDeployment *core.ControllerDeployment) bool {
	// The specification changes.
	if !apiequality.Semantic.DeepEqual(oldControllerDeployment.ProviderConfig, newControllerDeployment.ProviderConfig) ||
		!apiequality.Semantic.DeepEqual(oldControllerDeployment.Type, newControllerDeployment.Type) ||
		!apiequality.Semantic.DeepEqual(oldControllerDeployment.Helm, newControllerDeployment.Helm) {
		return true
	}

	// The deletion timestamp was set.
	if oldControllerDeployment.DeletionTimestamp == nil && newControllerDeployment.DeletionTimestamp != nil {
		return true
	}

	return false
}

func (controllerDeploymentStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	controllerDeployment := obj.(*core.ControllerDeployment)
	return validation.ValidateControllerDeployment(controllerDeployment)
}

func (controllerDeploymentStrategy) Canonicalize(_ runtime.Object) {
}

func (controllerDeploymentStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (controllerDeploymentStrategy) ValidateUpdate(_ context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	newControllerDeployment := newObj.(*core.ControllerDeployment)
	oldControllerDeployment := oldObj.(*core.ControllerDeployment)
	return validation.ValidateControllerDeploymentUpdate(newControllerDeployment, oldControllerDeployment)
}

func (controllerDeploymentStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// WarningsOnCreate returns warnings to the client performing a create.
func (controllerDeploymentStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (controllerDeploymentStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

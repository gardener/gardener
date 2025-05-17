// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerregistration

import (
	"context"
	"slices"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/validation"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
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

func (controllerRegistrationStrategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	controllerRegistration := obj.(*core.ControllerRegistration)

	controllerRegistration.Generation = 1
}

func (controllerRegistrationStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newControllerRegistration := obj.(*core.ControllerRegistration)
	oldControllerRegistration := old.(*core.ControllerRegistration)

	if mustIncreaseGeneration(oldControllerRegistration, newControllerRegistration) {
		newControllerRegistration.Generation = oldControllerRegistration.Generation + 1
	}

	handleAutoEnabledResources(oldControllerRegistration, newControllerRegistration)
}

func handleAutoEnabledResources(oldControllerRegistration, newControllerRegistration *core.ControllerRegistration) {
	resourceKindTypeToResource := map[string]core.ControllerResource{}
	for _, resource := range oldControllerRegistration.Spec.Resources {
		resourceKindTypeToResource[gardenerutils.ExtensionsID(resource.Kind, resource.Type)] = resource
	}

	for i, resource := range newControllerRegistration.Spec.Resources {
		var (
			oldResource core.ControllerResource
			ok          bool
		)

		if oldResource, ok = resourceKindTypeToResource[gardenerutils.ExtensionsID(resource.Kind, resource.Type)]; !ok {
			continue
		}

		// When globallyEnabled was set from true to false, the shoot type must be removed from the autoEnable list.
		// Don't do anything if globallyEnabled was not set before, as this means a conflict and field AutoEnable should take precedence in this case.
		if oldResource.GloballyEnabled != nil && *oldResource.GloballyEnabled && resource.GloballyEnabled != nil && !*resource.GloballyEnabled {
			newControllerRegistration.Spec.Resources[i].AutoEnable = slices.DeleteFunc(resource.AutoEnable, func(clusterType core.ClusterType) bool {
				return clusterType == core.ClusterTypeShoot
			})
		}
	}
}

func mustIncreaseGeneration(oldControllerRegistration, newControllerRegistration *core.ControllerRegistration) bool {
	// The specification changes.
	if !apiequality.Semantic.DeepEqual(oldControllerRegistration.Spec, newControllerRegistration.Spec) {
		return true
	}

	// The deletion timestamp was set.
	if oldControllerRegistration.DeletionTimestamp == nil && newControllerRegistration.DeletionTimestamp != nil {
		return true
	}

	return false
}

func (controllerRegistrationStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	controllerRegistration := obj.(*core.ControllerRegistration)
	return validation.ValidateControllerRegistration(controllerRegistration)
}

func (controllerRegistrationStrategy) Canonicalize(_ runtime.Object) {
}

func (controllerRegistrationStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (controllerRegistrationStrategy) ValidateUpdate(_ context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	newControllerRegistration := newObj.(*core.ControllerRegistration)
	oldControllerRegistration := oldObj.(*core.ControllerRegistration)
	return validation.ValidateControllerRegistrationUpdate(newControllerRegistration, oldControllerRegistration)
}

func (controllerRegistrationStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// WarningsOnCreate returns warnings to the client performing a create.
func (controllerRegistrationStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (controllerRegistrationStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

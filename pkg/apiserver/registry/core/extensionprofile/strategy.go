// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package extensionprofile

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/api/core/validation"
	"github.com/gardener/gardener/pkg/apis/core"
)

type extensionProfileStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy defines the storage strategy for ExtensionProfiles.
var Strategy = extensionProfileStrategy{api.Scheme, names.SimpleNameGenerator}

func (extensionProfileStrategy) NamespaceScoped() bool {
	return false
}

func (extensionProfileStrategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	extensionProfile := obj.(*core.ExtensionProfile)
	extensionProfile.Status = core.ExtensionProfileStatus{}
}

func (extensionProfileStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	extensionProfile := obj.(*core.ExtensionProfile)
	return validation.ValidateExtensionProfile(extensionProfile)
}

func (extensionProfileStrategy) Canonicalize(_ runtime.Object) {
}

func (extensionProfileStrategy) AllowCreateOnUpdate(_ context.Context) bool {
	return false
}

func (extensionProfileStrategy) PrepareForUpdate(_ context.Context, newObj, oldObj runtime.Object) {
	oldExtensionProfile := oldObj.(*core.ExtensionProfile)
	newExtensionProfile := newObj.(*core.ExtensionProfile)

	newExtensionProfile.Status = oldExtensionProfile.Status // can only be changed by extensionprofiles/status subresource.
}

func (extensionProfileStrategy) AllowUnconditionalUpdate(_ context.Context) bool {
	return true
}

func (extensionProfileStrategy) ValidateUpdate(_ context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldProfile, newProfile := oldObj.(*core.ExtensionProfile), newObj.(*core.ExtensionProfile)
	return validation.ValidateExtensionProfileUpdate(newProfile, oldProfile)
}

// WarningsOnCreate returns warnings to the client performing a create.
func (extensionProfileStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (extensionProfileStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

type extensionProfileStatusStrategy struct {
	extensionProfileStrategy
}

// StatusStrategy defines the storage strategy for the status subresource of ExtensionProfiles.
var StatusStrategy = extensionProfileStatusStrategy{Strategy}

func (extensionProfileStatusStrategy) PrepareForUpdate(_ context.Context, newObj, oldObj runtime.Object) {
	oldExtensionProfile, newExtensionProfile := oldObj.(*core.ExtensionProfile), newObj.(*core.ExtensionProfile)
	newExtensionProfile.Spec = oldExtensionProfile.Spec
}

func (extensionProfileStatusStrategy) ValidateUpdate(_ context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldExtensionProfile, newExtensionProfile := oldObj.(*core.ExtensionProfile), newObj.(*core.ExtensionProfile)
	return validation.ValidateExtensionProfileStatusUpdate(newExtensionProfile, oldExtensionProfile)
}

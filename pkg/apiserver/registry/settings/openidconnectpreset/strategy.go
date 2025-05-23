// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package openidconnectpreset

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/settings"
	"github.com/gardener/gardener/pkg/apis/settings/validation"
)

type oidcPresetStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy defines the storage strategy for openidconnectpresets.
var Strategy = oidcPresetStrategy{api.Scheme, names.SimpleNameGenerator}

func (oidcPresetStrategy) NamespaceScoped() bool {
	return true
}

func (oidcPresetStrategy) PrepareForCreate(_ context.Context, _ runtime.Object) {

}

func (oidcPresetStrategy) PrepareForUpdate(_ context.Context, _, _ runtime.Object) {

}

func (oidcPresetStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	oidcpreset := obj.(*settings.OpenIDConnectPreset)
	return validation.ValidateOpenIDConnectPreset(oidcpreset)
}

func (oidcPresetStrategy) Canonicalize(_ runtime.Object) {
}

func (oidcPresetStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (oidcPresetStrategy) ValidateUpdate(_ context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	newOIDCPreset := newObj.(*settings.OpenIDConnectPreset)
	oldOIDCPreset := oldObj.(*settings.OpenIDConnectPreset)
	return validation.ValidateOpenIDConnectPresetUpdate(newOIDCPreset, oldOIDCPreset)
}

func (oidcPresetStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// WarningsOnCreate returns warnings to the client performing a create.
func (oidcPresetStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (oidcPresetStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

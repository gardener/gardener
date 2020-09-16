// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package openidconnectpreset

import (
	"context"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/settings"
	"github.com/gardener/gardener/pkg/apis/settings/validation"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"
)

type oidcPresetStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy defines the storage strategy for openidconnectpresetss.
var Strategy = oidcPresetStrategy{api.Scheme, names.SimpleNameGenerator}

func (oidcPresetStrategy) NamespaceScoped() bool {
	return true
}

func (oidcPresetStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {

}

func (oidcPresetStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {

}

func (oidcPresetStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	oidcpreset := obj.(*settings.OpenIDConnectPreset)
	return validation.ValidateOpenIDConnectPreset(oidcpreset)
}

func (oidcPresetStrategy) Canonicalize(obj runtime.Object) {
}

func (oidcPresetStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (oidcPresetStrategy) ValidateUpdate(ctx context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	newOIDCPreset := newObj.(*settings.OpenIDConnectPreset)
	oldOIDCPreset := oldObj.(*settings.OpenIDConnectPreset)
	return validation.ValidateOpenIDConnectPresetUpdate(newOIDCPreset, oldOIDCPreset)
}

func (oidcPresetStrategy) AllowUnconditionalUpdate() bool {
	return false
}

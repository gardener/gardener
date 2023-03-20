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

// WarningsOnCreate returns warnings to the client performing a create.
func (oidcPresetStrategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (oidcPresetStrategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
}

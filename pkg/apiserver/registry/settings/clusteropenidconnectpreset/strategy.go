// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package clusteropenidconnectpreset

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/settings"
	"github.com/gardener/gardener/pkg/apis/settings/validation"
)

type clusterOIDCPresetStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy defines the storage strategy for clusteropenidconnectpresets.
var Strategy = clusterOIDCPresetStrategy{api.Scheme, names.SimpleNameGenerator}

func (clusterOIDCPresetStrategy) NamespaceScoped() bool {
	return false
}

func (clusterOIDCPresetStrategy) PrepareForCreate(_ context.Context, _ runtime.Object) {

}

func (clusterOIDCPresetStrategy) PrepareForUpdate(_ context.Context, _, _ runtime.Object) {

}

func (clusterOIDCPresetStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	oidcpreset := obj.(*settings.ClusterOpenIDConnectPreset)
	return validation.ValidateClusterOpenIDConnectPreset(oidcpreset)
}

func (clusterOIDCPresetStrategy) Canonicalize(_ runtime.Object) {
}

func (clusterOIDCPresetStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (clusterOIDCPresetStrategy) ValidateUpdate(_ context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	newOIDCPreset := newObj.(*settings.ClusterOpenIDConnectPreset)
	oldOIDCPreset := oldObj.(*settings.ClusterOpenIDConnectPreset)
	return validation.ValidateClusterOpenIDConnectPresetUpdate(newOIDCPreset, oldOIDCPreset)
}

func (clusterOIDCPresetStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// WarningsOnCreate returns warnings to the client performing a create.
func (clusterOIDCPresetStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (clusterOIDCPresetStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

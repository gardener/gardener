// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package clusteropenidconnectpreset

import (
	"context"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/settings"
	"github.com/gardener/gardener/pkg/apis/settings/validation"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"
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

func (clusterOIDCPresetStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {

}

func (clusterOIDCPresetStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {

}

func (clusterOIDCPresetStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	oidcpreset := obj.(*settings.ClusterOpenIDConnectPreset)
	return validation.ValidateClusterOpenIDConnectPreset(oidcpreset)
}

func (clusterOIDCPresetStrategy) Canonicalize(obj runtime.Object) {
}

func (clusterOIDCPresetStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (clusterOIDCPresetStrategy) ValidateUpdate(ctx context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	newOIDCPreset := newObj.(*settings.ClusterOpenIDConnectPreset)
	oldOIDCPreset := oldObj.(*settings.ClusterOpenIDConnectPreset)
	return validation.ValidateClusterOpenIDConnectPresetUpdate(newOIDCPreset, oldOIDCPreset)
}

func (clusterOIDCPresetStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/gardener/gardener/pkg/apis/settings"
	"github.com/gardener/gardener/pkg/apiserver/registry/settings/clusteropenidconnectpreset"
)

// REST implements a RESTStorage for ClusterOpenIDConnectPresets against etcd.
type REST struct {
	*genericregistry.Store
}

// Storage implements the storage for ClusterOpenIDConnectPresets and their status subresource.
type Storage struct {
	ClusterOpenIDConnectPreset *REST
}

// NewStorage creates a new ClusterOpenIDConnectPreset object.
func NewStorage(optsGetter generic.RESTOptionsGetter) Storage {
	ClusterOpenIDConnectPresetRest := NewREST(optsGetter)

	return Storage{
		ClusterOpenIDConnectPreset: ClusterOpenIDConnectPresetRest,
	}
}

// NewREST returns a RESTStorage object that will work against ClusterOpenIDConnectPresets.
func NewREST(optsGetter generic.RESTOptionsGetter) *REST {
	store := &genericregistry.Store{
		NewFunc:     func() runtime.Object { return &settings.ClusterOpenIDConnectPreset{} },
		NewListFunc: func() runtime.Object { return &settings.ClusterOpenIDConnectPresetList{} },

		DefaultQualifiedResource:  settings.Resource("clusteropenidconnectpresets"),
		SingularQualifiedResource: settings.Resource("clusteropenidconnectpreset"),
		EnableGarbageCollection:   true,

		CreateStrategy: clusteropenidconnectpreset.Strategy,
		UpdateStrategy: clusteropenidconnectpreset.Strategy,
		DeleteStrategy: clusteropenidconnectpreset.Strategy,

		TableConvertor: newTableConvertor(),
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter}
	if err := store.CompleteWithOptions(options); err != nil {
		panic(err)
	}

	return &REST{store}
}

// Implement CategoriesProvider
var _ rest.CategoriesProvider = &REST{}

// Categories implements the CategoriesProvider interface. Returns a list of categories a resource is part of.
func (r *REST) Categories() []string {
	return []string{"all"}
}

// Implement ShortNamesProvider
var _ rest.ShortNamesProvider = &REST{}

// ShortNames implements the ShortNamesProvider interface. Returns a list of short names for a resource.
func (r *REST) ShortNames() []string {
	return []string{"coidcps"}
}

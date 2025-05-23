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
	"github.com/gardener/gardener/pkg/apiserver/registry/settings/openidconnectpreset"
)

// REST implements a RESTStorage for OpenIDConnectPresets against etcd.
type REST struct {
	*genericregistry.Store
}

// Storage implements the storage for OpenIDConnectPresets and their status subresource.
type Storage struct {
	OpenIDConnectPreset *REST
}

// NewStorage creates a new OpenIDConnectPreset object.
func NewStorage(optsGetter generic.RESTOptionsGetter) Storage {
	OpenIDConnectPresetRest := NewREST(optsGetter)

	return Storage{
		OpenIDConnectPreset: OpenIDConnectPresetRest,
	}
}

// NewREST returns a RESTStorage object that will work against OpenIDConnectPresets.
func NewREST(optsGetter generic.RESTOptionsGetter) *REST {
	store := &genericregistry.Store{
		NewFunc:     func() runtime.Object { return &settings.OpenIDConnectPreset{} },
		NewListFunc: func() runtime.Object { return &settings.OpenIDConnectPresetList{} },

		DefaultQualifiedResource:  settings.Resource("openidconnectpresets"),
		SingularQualifiedResource: settings.Resource("openidconnectpreset"),
		EnableGarbageCollection:   true,

		CreateStrategy: openidconnectpreset.Strategy,
		UpdateStrategy: openidconnectpreset.Strategy,
		DeleteStrategy: openidconnectpreset.Strategy,

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
	return []string{"oidcps"}
}

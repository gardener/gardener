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

package storage

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/gardener/gardener/pkg/apis/settings"
	"github.com/gardener/gardener/pkg/registry/settings/openidconnectpreset"
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

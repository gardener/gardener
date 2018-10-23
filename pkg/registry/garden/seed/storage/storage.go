// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"context"

	"github.com/gardener/gardener/pkg/apis/garden"
	"github.com/gardener/gardener/pkg/registry/garden/seed"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
)

// REST implements a RESTStorage for Seed
type REST struct {
	*genericregistry.Store
}

// SeedStorage implements the storage for Seeds.
type SeedStorage struct {
	Seed   *REST
	Status *StatusREST
}

// NewStorage creates a new SeedStorage object.
func NewStorage(optsGetter generic.RESTOptionsGetter) SeedStorage {
	seedRest, seedStatusRest := NewREST(optsGetter)

	return SeedStorage{
		Seed:   seedRest,
		Status: seedStatusRest,
	}
}

// NewREST returns a RESTStorage object that will work with Seed objects.
func NewREST(optsGetter generic.RESTOptionsGetter) (*REST, *StatusREST) {
	store := &genericregistry.Store{
		NewFunc:                  func() runtime.Object { return &garden.Seed{} },
		NewListFunc:              func() runtime.Object { return &garden.SeedList{} },
		DefaultQualifiedResource: garden.Resource("seeds"),
		EnableGarbageCollection:  true,

		CreateStrategy: seed.Strategy,
		UpdateStrategy: seed.Strategy,
		DeleteStrategy: seed.Strategy,

		TableConvertor: newTableConvertor(),
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter}
	if err := store.CompleteWithOptions(options); err != nil {
		panic(err)
	}

	statusStore := *store
	statusStore.UpdateStrategy = seed.StatusStrategy
	return &REST{store}, &StatusREST{store: &statusStore}
}

// StatusREST implements the REST endpoint for changing the status of a Seed.
type StatusREST struct {
	store *genericregistry.Store
}

var (
	_ rest.Storage = &StatusREST{}
	_ rest.Getter  = &StatusREST{}
	_ rest.Updater = &StatusREST{}
)

// New creates a new (empty) internal Seed object.
func (r *StatusREST) New() runtime.Object {
	return &garden.Seed{}
}

// Get retrieves the object from the storage. It is required to support Patch.
func (r *StatusREST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return r.store.Get(ctx, name, options)
}

// Update alters the status subset of an object.
func (r *StatusREST) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	return r.store.Update(ctx, name, objInfo, createValidation, updateValidation, forceAllowCreate, options)
}

// Implement ShortNamesProvider
var _ rest.ShortNamesProvider = &REST{}

// ShortNames implements the ShortNamesProvider interface. Returns a list of short names for a resource.
func (r *REST) ShortNames() []string {
	return []string{}
}

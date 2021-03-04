// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	"github.com/gardener/gardener/pkg/registry/seedmanagement/managedseedset"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
)

// REST implements a RESTStorage for ManagedSeedSet.
type REST struct {
	*genericregistry.Store
}

// ManagedSeedSetStorage implements the storage for ManagedSeedSets and their status subresource.
type ManagedSeedSetStorage struct {
	ManagedSeedSet *REST
	Status         *StatusREST
}

// NewStorage creates a new ManagedSeedSetStorage object.
func NewStorage(optsGetter generic.RESTOptionsGetter) ManagedSeedSetStorage {
	managedSeedSetRest, managedSeedSetStatusRest := NewREST(optsGetter)

	return ManagedSeedSetStorage{
		ManagedSeedSet: managedSeedSetRest,
		Status:         managedSeedSetStatusRest,
	}
}

// NewREST returns a RESTStorage object that will work with ManagedSeedSet objects.
func NewREST(optsGetter generic.RESTOptionsGetter) (*REST, *StatusREST) {
	strategy := managedseedset.NewStrategy()
	statusStrategy := managedseedset.NewStatusStrategy()

	store := &genericregistry.Store{
		NewFunc:                  func() runtime.Object { return &seedmanagement.ManagedSeedSet{} },
		NewListFunc:              func() runtime.Object { return &seedmanagement.ManagedSeedSetList{} },
		DefaultQualifiedResource: seedmanagement.Resource("managedseedsets"),
		EnableGarbageCollection:  true,

		CreateStrategy: strategy,
		UpdateStrategy: strategy,
		DeleteStrategy: strategy,

		TableConvertor: newTableConvertor(),
	}
	options := &generic.StoreOptions{
		RESTOptions: optsGetter,
		AttrFunc:    managedseedset.GetAttrs,
	}
	if err := store.CompleteWithOptions(options); err != nil {
		panic(err)
	}

	statusStore := *store
	statusStore.UpdateStrategy = statusStrategy

	return &REST{store}, &StatusREST{store: &statusStore}
}

// StatusREST implements the REST endpoint for changing the status of a ManagedSeedSet.
type StatusREST struct {
	store *genericregistry.Store
}

var (
	_ rest.Storage = &StatusREST{}
	_ rest.Getter  = &StatusREST{}
	_ rest.Updater = &StatusREST{}
)

// New creates a new (empty) internal ManagedSeedSet object.
func (r *StatusREST) New() runtime.Object {
	return &seedmanagement.ManagedSeedSet{}
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

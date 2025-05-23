// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"

	"github.com/gardener/gardener/pkg/apis/operations"
	"github.com/gardener/gardener/pkg/apiserver/registry/operations/bastion"
)

// REST implements a RESTStorage for Bastions against etcd
type REST struct {
	*genericregistry.Store
}

// BastionStorage implements the storage for Bastions and their status subresource.
type BastionStorage struct {
	Bastion *REST
	Status  *StatusREST
}

// NewStorage creates a new BastionStorage object.
func NewStorage(optsGetter generic.RESTOptionsGetter) BastionStorage {
	bastionRest, bastionStatusRest := NewREST(optsGetter)

	return BastionStorage{
		Bastion: bastionRest,
		Status:  bastionStatusRest,
	}
}

// NewREST returns a RESTStorage object that will work against bastions.
func NewREST(optsGetter generic.RESTOptionsGetter) (*REST, *StatusREST) {
	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &operations.Bastion{} },
		NewListFunc:               func() runtime.Object { return &operations.BastionList{} },
		DefaultQualifiedResource:  operations.Resource("bastions"),
		SingularQualifiedResource: operations.Resource("bastion"),
		EnableGarbageCollection:   true,
		PredicateFunc:             bastion.MatchBastion,

		CreateStrategy: bastion.Strategy,
		UpdateStrategy: bastion.Strategy,
		DeleteStrategy: bastion.Strategy,

		TableConvertor: newTableConvertor(),
	}
	options := &generic.StoreOptions{
		RESTOptions: optsGetter,
		AttrFunc:    bastion.GetAttrs,
		TriggerFunc: map[string]storage.IndexerFunc{operations.BastionSeedName: bastion.SeedNameTriggerFunc},
	}
	if err := store.CompleteWithOptions(options); err != nil {
		panic(err)
	}

	statusStore := *store
	statusStore.UpdateStrategy = bastion.StatusStrategy
	return &REST{store}, &StatusREST{store: &statusStore}
}

// Implement CategoriesProvider
var _ rest.CategoriesProvider = &REST{}

// Categories implements the CategoriesProvider interface. Returns a list of categories a resource is part of.
func (r *REST) Categories() []string {
	return []string{"all"}
}

// StatusREST implements the REST endpoint for changing the status of a Bastion.
type StatusREST struct {
	store *genericregistry.Store
}

var (
	_ rest.Storage = &StatusREST{}
	_ rest.Getter  = &StatusREST{}
	_ rest.Updater = &StatusREST{}
)

// New creates a new (empty) internal Bastion object.
func (r *StatusREST) New() runtime.Object {
	return &operations.Bastion{}
}

// Destroy cleans up its resources on shutdown.
func (r *StatusREST) Destroy() {
	// Given that underlying store is shared with REST,
	// we don't destroy it here explicitly.
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

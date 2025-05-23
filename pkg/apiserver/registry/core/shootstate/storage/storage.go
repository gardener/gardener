// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apiserver/registry/core/shootstate"
)

// REST implements a RESTStorage for ShootStates against etcd.
type REST struct {
	*genericregistry.Store
}

// ShootState implements the storage for ShootStates and their status subresource.
type ShootState struct {
	ShootState *REST
}

// NewStorage creates a new ShootState object.
func NewStorage(optsGetter generic.RESTOptionsGetter) ShootState {
	ShootStateRest := NewREST(optsGetter)

	return ShootState{
		ShootState: ShootStateRest,
	}
}

// NewREST returns a RESTStorage object that will work against ShootStates.
func NewREST(optsGetter generic.RESTOptionsGetter) *REST {
	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &core.ShootState{} },
		NewListFunc:               func() runtime.Object { return &core.ShootStateList{} },
		DefaultQualifiedResource:  core.Resource("shootstates"),
		SingularQualifiedResource: core.Resource("shootstate"),
		EnableGarbageCollection:   true,

		CreateStrategy: shootstate.Strategy,
		UpdateStrategy: shootstate.Strategy,
		DeleteStrategy: shootstate.Strategy,

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

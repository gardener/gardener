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

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/registry/core/shootstate"
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
		NewFunc:                  func() runtime.Object { return &core.ShootState{} },
		NewListFunc:              func() runtime.Object { return &core.ShootStateList{} },
		DefaultQualifiedResource: core.Resource("shootstates"),
		EnableGarbageCollection:  true,

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

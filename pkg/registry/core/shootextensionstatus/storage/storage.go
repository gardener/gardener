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
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/registry/core/shootextensionstatus"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
)

// REST implements a RESTStorage for ShootExtensionStatus against etcd.
type REST struct {
	*genericregistry.Store
}

// ShootExtensionStatus implements the storage for ShootExtensionStatus resources.
type ShootExtensionStatus struct {
	ShootExtensionStatus *REST
}

// NewStorage creates a new ShootExtensionStatus object.
func NewStorage(optsGetter generic.RESTOptionsGetter) ShootExtensionStatus {
	ShootExtensionStatusRest := NewREST(optsGetter)

	return ShootExtensionStatus{
		ShootExtensionStatus: ShootExtensionStatusRest,
	}
}

// NewREST returns a RESTStorage object that will work against ShootExtensionStatus resources.
func NewREST(optsGetter generic.RESTOptionsGetter) *REST {
	store := &genericregistry.Store{
		NewFunc:                  func() runtime.Object { return &core.ShootExtensionStatus{} },
		NewListFunc:              func() runtime.Object { return &core.ShootExtensionStatusList{} },
		DefaultQualifiedResource: core.Resource("shootextensionstatuses"),
		EnableGarbageCollection:  true,

		CreateStrategy: shootextensionstatus.Strategy,
		UpdateStrategy: shootextensionstatus.Strategy,
		DeleteStrategy: shootextensionstatus.Strategy,

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

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
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/registry/core/controllerregistration"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
)

// REST implements a RESTStorage for ControllerRegistrations against etcd.
type REST struct {
	*genericregistry.Store
}

// ControllerRegistrationStorage implements the storage for ControllerRegistrations and their status subresource.
type ControllerRegistrationStorage struct {
	ControllerRegistration *REST
}

// NewStorage creates a new ControllerRegistrationStorage object.
func NewStorage(optsGetter generic.RESTOptionsGetter) ControllerRegistrationStorage {
	controllerRegistrationRest := NewREST(optsGetter)

	return ControllerRegistrationStorage{
		ControllerRegistration: controllerRegistrationRest,
	}
}

// NewREST returns a RESTStorage object that will work against controllerRegistrations.
func NewREST(optsGetter generic.RESTOptionsGetter) *REST {
	store := &genericregistry.Store{
		NewFunc:                  func() runtime.Object { return &core.ControllerRegistration{} },
		NewListFunc:              func() runtime.Object { return &core.ControllerRegistrationList{} },
		DefaultQualifiedResource: core.Resource("controllerregistrations"),
		EnableGarbageCollection:  true,

		CreateStrategy: controllerregistration.Strategy,
		UpdateStrategy: controllerregistration.Strategy,
		DeleteStrategy: controllerregistration.Strategy,

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
	return []string{"ctrlreg"}
}

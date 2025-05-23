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
	"github.com/gardener/gardener/pkg/apiserver/registry/core/controllerregistration"
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
		NewFunc:                   func() runtime.Object { return &core.ControllerRegistration{} },
		NewListFunc:               func() runtime.Object { return &core.ControllerRegistrationList{} },
		DefaultQualifiedResource:  core.Resource("controllerregistrations"),
		SingularQualifiedResource: core.Resource("controllerregistration"),
		EnableGarbageCollection:   true,

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

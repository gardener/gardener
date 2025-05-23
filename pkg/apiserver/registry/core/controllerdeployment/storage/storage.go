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
	"github.com/gardener/gardener/pkg/apiserver/registry/core/controllerdeployment"
)

// REST implements a RESTStorage for ControllerDeployments against etcd.
type REST struct {
	*genericregistry.Store
}

// ControllerDeploymentStorage implements the storage for ControllerDeployments and their status subresource.
type ControllerDeploymentStorage struct {
	ControllerDeployment *REST
}

// NewStorage creates a new ControllerDeploymentStorage object.
func NewStorage(optsGetter generic.RESTOptionsGetter) ControllerDeploymentStorage {
	controllerDeploymentRest := NewREST(optsGetter)

	return ControllerDeploymentStorage{
		ControllerDeployment: controllerDeploymentRest,
	}
}

// NewREST returns a RESTStorage object that will work against controllerDeployments.
func NewREST(optsGetter generic.RESTOptionsGetter) *REST {
	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &core.ControllerDeployment{} },
		NewListFunc:               func() runtime.Object { return &core.ControllerDeploymentList{} },
		DefaultQualifiedResource:  core.Resource("controllerdeployments"),
		SingularQualifiedResource: core.Resource("controllerdeployment"),
		EnableGarbageCollection:   true,

		CreateStrategy: controllerdeployment.Strategy,
		UpdateStrategy: controllerdeployment.Strategy,
		DeleteStrategy: controllerdeployment.Strategy,

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
	return []string{"ctrldeploy"}
}

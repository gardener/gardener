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
	"k8s.io/client-go/tools/cache"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apiserver/registry/core/controllerinstallation"
)

// REST implements a RESTStorage for ControllerInstallations against etcd.
type REST struct {
	*genericregistry.Store
}

// ControllerInstallationStorage implements the storage for ControllerInstallations and their status subresource.
type ControllerInstallationStorage struct {
	ControllerInstallation *REST
	Status                 *StatusREST
}

// NewStorage creates a new ControllerInstallationStorage object.
func NewStorage(optsGetter generic.RESTOptionsGetter) ControllerInstallationStorage {
	controllerInstallationRest, controllerInstallationStatusRest := NewREST(optsGetter)

	return ControllerInstallationStorage{
		ControllerInstallation: controllerInstallationRest,
		Status:                 controllerInstallationStatusRest,
	}
}

// NewREST returns a RESTStorage object that will work against controllerInstallations.
func NewREST(optsGetter generic.RESTOptionsGetter) (*REST, *StatusREST) {
	var (
		store = &genericregistry.Store{
			NewFunc:                   func() runtime.Object { return &core.ControllerInstallation{} },
			NewListFunc:               func() runtime.Object { return &core.ControllerInstallationList{} },
			PredicateFunc:             controllerinstallation.MatchControllerInstallation,
			DefaultQualifiedResource:  core.Resource("controllerinstallations"),
			SingularQualifiedResource: core.Resource("controllerinstallation"),
			EnableGarbageCollection:   true,

			CreateStrategy: controllerinstallation.Strategy,
			UpdateStrategy: controllerinstallation.Strategy,
			DeleteStrategy: controllerinstallation.Strategy,

			TableConvertor: newTableConvertor(),
		}
		options = &generic.StoreOptions{
			RESTOptions: optsGetter,
			AttrFunc:    controllerinstallation.GetAttrs,
			Indexers: &cache.Indexers{
				storage.FieldIndex(core.SeedRefName):         controllerinstallation.SeedRefNameIndexFunc,
				storage.FieldIndex(core.RegistrationRefName): controllerinstallation.RegistrationRefNameIndexFunc,
			},
		}
	)

	if err := store.CompleteWithOptions(options); err != nil {
		panic(err)
	}

	statusStore := *store
	statusStore.UpdateStrategy = controllerinstallation.StatusStrategy
	return &REST{store}, &StatusREST{store: &statusStore}
}

// Implement CategoriesProvider
var _ rest.CategoriesProvider = &REST{}

// Categories implements the CategoriesProvider interface. Returns a list of categories a resource is part of.
func (r *REST) Categories() []string {
	return []string{"all"}
}

// StatusREST implements the REST endpoint for changing the status of a ControllerInstallation.
type StatusREST struct {
	store *genericregistry.Store
}

var (
	_ rest.Storage = &StatusREST{}
	_ rest.Getter  = &StatusREST{}
	_ rest.Updater = &StatusREST{}
)

// New creates a new (empty) internal ControllerInstallation object.
func (r *StatusREST) New() runtime.Object {
	return &core.ControllerInstallation{}
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
	return []string{"ctrlinst"}
}

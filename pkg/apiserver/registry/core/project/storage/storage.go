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

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apiserver/registry/core/project"
)

// REST implements a RESTStorage for Project
type REST struct {
	*genericregistry.Store
}

// ProjectStorage implements the storage for Projects.
type ProjectStorage struct {
	Project *REST
	Status  *StatusREST
}

// NewStorage creates a new ProjectStorage object.
func NewStorage(optsGetter generic.RESTOptionsGetter) ProjectStorage {
	projectRest, projectStatusRest := NewREST(optsGetter)

	return ProjectStorage{
		Project: projectRest,
		Status:  projectStatusRest,
	}
}

// NewREST returns a RESTStorage object that will work with Project objects.
func NewREST(optsGetter generic.RESTOptionsGetter) (*REST, *StatusREST) {
	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &core.Project{} },
		NewListFunc:               func() runtime.Object { return &core.ProjectList{} },
		DefaultQualifiedResource:  core.Resource("projects"),
		SingularQualifiedResource: core.Resource("project"),
		EnableGarbageCollection:   true,

		CreateStrategy: project.Strategy,
		UpdateStrategy: project.Strategy,
		DeleteStrategy: project.Strategy,

		TableConvertor: newTableConvertor(),
	}
	options := &generic.StoreOptions{
		RESTOptions: optsGetter,
		AttrFunc:    project.GetAttrs,
		TriggerFunc: map[string]storage.IndexerFunc{core.ProjectNamespace: project.NamespaceTriggerFunc},
	}
	if err := store.CompleteWithOptions(options); err != nil {
		panic(err)
	}

	statusStore := *store
	statusStore.UpdateStrategy = project.StatusStrategy
	return &REST{store}, &StatusREST{store: &statusStore}
}

// StatusREST implements the REST endpoint for changing the status of a Project.
type StatusREST struct {
	store *genericregistry.Store
}

var (
	_ rest.Storage = &StatusREST{}
	_ rest.Getter  = &StatusREST{}
	_ rest.Updater = &StatusREST{}
)

// New creates a new (empty) internal Project object.
func (r *StatusREST) New() runtime.Object {
	return &core.Project{}
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

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
	"context"

	"github.com/gardener/gardener/pkg/apis/garden"
	"github.com/gardener/gardener/pkg/registry/garden/project"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
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
		NewFunc:                  func() runtime.Object { return &garden.Project{} },
		NewListFunc:              func() runtime.Object { return &garden.ProjectList{} },
		DefaultQualifiedResource: garden.Resource("projects"),
		EnableGarbageCollection:  true,

		CreateStrategy: project.Strategy,
		UpdateStrategy: project.Strategy,
		DeleteStrategy: project.Strategy,

		TableConvertor: newTableConvertor(),
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter}
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
	return &garden.Project{}
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

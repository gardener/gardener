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
	"github.com/gardener/gardener/pkg/registry/garden/backupinfrastructure"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
)

// REST implements a RESTStorage for backupInfrastructures against etcd
type REST struct {
	*genericregistry.Store
}

// BackupInfrastructureStorage implements the storage for BackupInfrastructures and their status subresource.
type BackupInfrastructureStorage struct {
	BackupInfrastructure *REST
	Status               *StatusREST
}

// NewStorage creates a new BackupInfrastructureStorage object.
func NewStorage(optsGetter generic.RESTOptionsGetter) BackupInfrastructureStorage {
	backupInfrastructureRest, backupInfrastructureStatusRest := NewREST(optsGetter)

	return BackupInfrastructureStorage{
		BackupInfrastructure: backupInfrastructureRest,
		Status:               backupInfrastructureStatusRest,
	}
}

// NewREST returns a RESTStorage object that will work against backupInfrastructures.
func NewREST(optsGetter generic.RESTOptionsGetter) (*REST, *StatusREST) {
	store := &genericregistry.Store{
		NewFunc:                  func() runtime.Object { return &garden.BackupInfrastructure{} },
		NewListFunc:              func() runtime.Object { return &garden.BackupInfrastructureList{} },
		DefaultQualifiedResource: garden.Resource("backupinfrastructures"),
		EnableGarbageCollection:  true,

		CreateStrategy: backupinfrastructure.Strategy,
		UpdateStrategy: backupinfrastructure.Strategy,
		DeleteStrategy: backupinfrastructure.Strategy,

		TableConvertor: newTableConvertor(),
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter}
	if err := store.CompleteWithOptions(options); err != nil {
		panic(err)
	}

	statusStore := *store
	statusStore.UpdateStrategy = backupinfrastructure.StatusStrategy
	return &REST{store}, &StatusREST{store: &statusStore}
}

// Implement CategoriesProvider
var _ rest.CategoriesProvider = &REST{}

// Categories implements the CategoriesProvider interface. Returns a list of categories a resource is part of.
func (r *REST) Categories() []string {
	return []string{"all"}
}

// StatusREST implements the REST endpoint for changing the status of a BackupInfrastructure.
type StatusREST struct {
	store *genericregistry.Store
}

var (
	_ rest.Storage = &StatusREST{}
	_ rest.Getter  = &StatusREST{}
	_ rest.Updater = &StatusREST{}
)

// New creates a new (empty) internal BackupInfrastructure object.
func (r *StatusREST) New() runtime.Object {
	return &garden.BackupInfrastructure{}
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
	return []string{"backupinfra"}
}

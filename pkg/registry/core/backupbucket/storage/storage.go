// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/registry/core/backupbucket"
)

// REST implements a RESTStorage for backupBuckets against etcd
type REST struct {
	*genericregistry.Store
}

// BackupBucketStorage implements the storage for BackupBuckets and their status subresource.
type BackupBucketStorage struct {
	BackupBucket *REST
	Status       *StatusREST
}

// NewStorage creates a new BackupBucketStorage object.
func NewStorage(optsGetter generic.RESTOptionsGetter) BackupBucketStorage {
	backupBucketRest, backupBucketStatusRest := NewREST(optsGetter)

	return BackupBucketStorage{
		BackupBucket: backupBucketRest,
		Status:       backupBucketStatusRest,
	}
}

// NewREST returns a RESTStorage object that will work against backupBuckets.
func NewREST(optsGetter generic.RESTOptionsGetter) (*REST, *StatusREST) {
	store := &genericregistry.Store{
		NewFunc:                  func() runtime.Object { return &core.BackupBucket{} },
		NewListFunc:              func() runtime.Object { return &core.BackupBucketList{} },
		DefaultQualifiedResource: core.Resource("backupbuckets"),
		EnableGarbageCollection:  true,

		CreateStrategy: backupbucket.Strategy,
		UpdateStrategy: backupbucket.Strategy,
		DeleteStrategy: backupbucket.Strategy,

		TableConvertor: newTableConvertor(),
	}
	options := &generic.StoreOptions{
		RESTOptions: optsGetter,
		AttrFunc:    backupbucket.GetAttrs,
		TriggerFunc: map[string]storage.IndexerFunc{core.BackupBucketSeedName: backupbucket.SeedNameTriggerFunc},
	}
	if err := store.CompleteWithOptions(options); err != nil {
		panic(err)
	}

	statusStore := *store
	statusStore.UpdateStrategy = backupbucket.StatusStrategy
	return &REST{store}, &StatusREST{store: &statusStore}
}

// Implement CategoriesProvider
var _ rest.CategoriesProvider = &REST{}

// Categories implements the CategoriesProvider interface. Returns a list of categories a resource is part of.
func (r *REST) Categories() []string {
	return []string{"all"}
}

// StatusREST implements the REST endpoint for changing the status of a BackupBucket.
type StatusREST struct {
	store *genericregistry.Store
}

var (
	_ rest.Storage = &StatusREST{}
	_ rest.Getter  = &StatusREST{}
	_ rest.Updater = &StatusREST{}
)

// New creates a new (empty) internal BackupBucket object.
func (r *StatusREST) New() runtime.Object {
	return &core.BackupBucket{}
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
	return []string{"bbc"}
}

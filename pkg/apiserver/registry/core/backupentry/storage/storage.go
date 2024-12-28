// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
	"github.com/gardener/gardener/pkg/apiserver/registry/core/backupentry"
)

// REST implements a RESTStorage for backupEntries against etcd
type REST struct {
	*genericregistry.Store
}

// BackupEntryStorage implements the storage for BackupEntries and their status subresource.
type BackupEntryStorage struct {
	BackupEntry *REST
	Status      *StatusREST
}

// NewStorage creates a new BackupEntryStorage object.
func NewStorage(optsGetter generic.RESTOptionsGetter) BackupEntryStorage {
	backupEntryRest, backupEntryStatusRest := NewREST(optsGetter)

	return BackupEntryStorage{
		BackupEntry: backupEntryRest,
		Status:      backupEntryStatusRest,
	}
}

// NewREST returns a RESTStorage object that will work against backupEntries.
func NewREST(optsGetter generic.RESTOptionsGetter) (*REST, *StatusREST) {
	var (
		store = &genericregistry.Store{
			NewFunc:                   func() runtime.Object { return &core.BackupEntry{} },
			NewListFunc:               func() runtime.Object { return &core.BackupEntryList{} },
			DefaultQualifiedResource:  core.Resource("backupentries"),
			SingularQualifiedResource: core.Resource("backupentry"),
			EnableGarbageCollection:   true,

			CreateStrategy: backupentry.Strategy,
			UpdateStrategy: backupentry.Strategy,
			DeleteStrategy: backupentry.Strategy,

			TableConvertor: newTableConvertor(),
		}
		options = &generic.StoreOptions{
			RESTOptions: optsGetter,
			AttrFunc:    backupentry.GetAttrs,
			Indexers: &cache.Indexers{
				storage.FieldIndex(core.BackupEntrySeedName):   backupentry.SeedNameIndexFunc,
				storage.FieldIndex(core.BackupEntryBucketName): backupentry.BucketNameIndexFunc,
			},
		}
	)
	if err := store.CompleteWithOptions(options); err != nil {
		panic(err)
	}

	statusStore := *store
	statusStore.UpdateStrategy = backupentry.StatusStrategy
	return &REST{store}, &StatusREST{store: &statusStore}
}

// Implement CategoriesProvider
var _ rest.CategoriesProvider = &REST{}

// Categories implements the CategoriesProvider interface. Returns a list of categories a resource is part of.
func (r *REST) Categories() []string {
	return []string{"all"}
}

// StatusREST implements the REST endpoint for changing the status of a BackupEntry.
type StatusREST struct {
	store *genericregistry.Store
}

var (
	_ rest.Storage = &StatusREST{}
	_ rest.Getter  = &StatusREST{}
	_ rest.Updater = &StatusREST{}
)

// New creates a new (empty) internal BackupEntry object.
func (r *StatusREST) New() runtime.Object {
	return &core.BackupEntry{}
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
	return []string{"bec"}
}

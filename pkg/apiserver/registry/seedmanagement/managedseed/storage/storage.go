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

	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	"github.com/gardener/gardener/pkg/apiserver/registry/seedmanagement/managedseed"
)

// REST implements a RESTStorage for ManagedSeed.
type REST struct {
	*genericregistry.Store
}

// ManagedSeedStorage implements the storage for ManagedSeeds and their status subresource.
type ManagedSeedStorage struct {
	ManagedSeed *REST
	Status      *StatusREST
}

// NewStorage creates a new ManagedSeedStorage object.
func NewStorage(optsGetter generic.RESTOptionsGetter) ManagedSeedStorage {
	managedSeedRest, managedSeedStatusRest := NewREST(optsGetter)

	return ManagedSeedStorage{
		ManagedSeed: managedSeedRest,
		Status:      managedSeedStatusRest,
	}
}

// NewREST returns a RESTStorage object that will work with ManagedSeed objects.
func NewREST(optsGetter generic.RESTOptionsGetter) (*REST, *StatusREST) {
	strategy := managedseed.NewStrategy()
	statusStrategy := managedseed.NewStatusStrategy()

	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &seedmanagement.ManagedSeed{} },
		NewListFunc:               func() runtime.Object { return &seedmanagement.ManagedSeedList{} },
		PredicateFunc:             managedseed.MatchManagedSeed,
		DefaultQualifiedResource:  seedmanagement.Resource("managedseeds"),
		SingularQualifiedResource: seedmanagement.Resource("managedseed"),
		EnableGarbageCollection:   true,

		CreateStrategy: strategy,
		UpdateStrategy: strategy,
		DeleteStrategy: strategy,
		Decorator:      defaultOnRead,

		TableConvertor: newTableConvertor(),
	}
	options := &generic.StoreOptions{
		RESTOptions: optsGetter,
		AttrFunc:    managedseed.GetAttrs,
		TriggerFunc: map[string]storage.IndexerFunc{seedmanagement.ManagedSeedShootName: managedseed.ShootNameTriggerFunc},
	}
	if err := store.CompleteWithOptions(options); err != nil {
		panic(err)
	}

	statusStore := *store
	statusStore.UpdateStrategy = statusStrategy

	return &REST{store}, &StatusREST{store: &statusStore}
}

// StatusREST implements the REST endpoint for changing the status of a ManagedSeed.
type StatusREST struct {
	store *genericregistry.Store
}

var (
	_ rest.Storage = &StatusREST{}
	_ rest.Getter  = &StatusREST{}
	_ rest.Updater = &StatusREST{}
)

// New creates a new (empty) internal ManagedSeed object.
func (r *StatusREST) New() runtime.Object {
	return &seedmanagement.ManagedSeed{}
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
	return []string{"ms"}
}

// defaultOnRead ensures the backup.credentialsRef field is set on Read requests
// TODO(vpnachev): Remove once the backup.secretRef field is removed.
func defaultOnRead(obj runtime.Object) {
	switch m := obj.(type) {
	case *seedmanagement.ManagedSeed:
		defaultOnReadManagedSeed(m)
	case *seedmanagement.ManagedSeedList:
		defaultOnReadManagedSeeds(m)
	default:
	}
}

func defaultOnReadManagedSeed(managedSeed *seedmanagement.ManagedSeed) {
	managedseed.SyncSeedBackupCredentials(managedSeed)
}

func defaultOnReadManagedSeeds(managedSeedList *seedmanagement.ManagedSeedList) {
	if managedSeedList == nil {
		return
	}

	for i := range managedSeedList.Items {
		defaultOnReadManagedSeed(&managedSeedList.Items[i])
	}
}

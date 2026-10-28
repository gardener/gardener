// SPDX-FileCopyrightText: Contributors to the Gardener project
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

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apiserver/registry/core/extensionprofile"
)

// REST implements a RESTStorage for ExtensionProfile.
type REST struct {
	*genericregistry.Store
}

// ExtensionProfileStorage implements the storage for ExtensionProfiles.
type ExtensionProfileStorage struct {
	ExtensionProfile *REST
	Status           *StatusREST
}

// NewStorage creates a new ExtensionProfileStorage object.
func NewStorage(optsGetter generic.RESTOptionsGetter) ExtensionProfileStorage {
	extensionProfileRest, extensionProfileStatusRest := NewREST(optsGetter)

	return ExtensionProfileStorage{
		ExtensionProfile: extensionProfileRest,
		Status:           extensionProfileStatusRest,
	}
}

// NewREST returns a RESTStorage object that will work with ExtensionProfile objects.
func NewREST(optsGetter generic.RESTOptionsGetter) (*REST, *StatusREST) {
	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &core.ExtensionProfile{} },
		NewListFunc:               func() runtime.Object { return &core.ExtensionProfileList{} },
		DefaultQualifiedResource:  core.Resource("extensionprofiles"),
		SingularQualifiedResource: core.Resource("extensionprofile"),
		EnableGarbageCollection:   true,

		CreateStrategy: extensionprofile.Strategy,
		UpdateStrategy: extensionprofile.Strategy,
		DeleteStrategy: extensionprofile.Strategy,

		TableConvertor: newTableConvertor(),
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter}
	if err := store.CompleteWithOptions(options); err != nil {
		panic(err)
	}

	statusStore := *store
	statusStore.UpdateStrategy = extensionprofile.StatusStrategy
	return &REST{store}, &StatusREST{store: &statusStore}
}

// Implement ShortNamesProvider
var _ rest.ShortNamesProvider = &REST{}

// ShortNames implements the ShortNamesProvider interface. Returns a list of short names for a resource.
func (r *REST) ShortNames() []string {
	return []string{"extprofile"}
}

// StatusREST implements the REST endpoint for changing the status of an ExtensionProfile.
type StatusREST struct {
	store *genericregistry.Store
}

var (
	_ rest.Storage = &StatusREST{}
	_ rest.Getter  = &StatusREST{}
	_ rest.Updater = &StatusREST{}
)

// New creates a new (empty) internal ExtensionProfile object.
func (r *StatusREST) New() runtime.Object {
	return &core.ExtensionProfile{}
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

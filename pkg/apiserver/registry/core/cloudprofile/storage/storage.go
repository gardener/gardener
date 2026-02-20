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

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apiserver/registry/core/cloudprofile"
)

// REST implements a RESTStorage for CloudProfile
type REST struct {
	*genericregistry.Store
}

// CloudProfileStorage implements the storage for CloudProfiles.
type CloudProfileStorage struct {
	CloudProfile *REST
	Status       *StatusREST
}

// NewStorage creates a new CloudProfileStorage object.
func NewStorage(optsGetter generic.RESTOptionsGetter) CloudProfileStorage {
	cloudProfileRest, cloudProfileStatusRest := NewREST(optsGetter)

	return CloudProfileStorage{
		CloudProfile: cloudProfileRest,
		Status:       cloudProfileStatusRest,
	}
}

// NewREST returns a RESTStorage object that will work with CloudProfile objects.
func NewREST(optsGetter generic.RESTOptionsGetter) (*REST, *StatusREST) {
	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &core.CloudProfile{} },
		NewListFunc:               func() runtime.Object { return &core.CloudProfileList{} },
		DefaultQualifiedResource:  core.Resource("cloudprofiles"),
		SingularQualifiedResource: core.Resource("cloudprofile"),
		EnableGarbageCollection:   true,

		CreateStrategy: cloudprofile.Strategy,
		UpdateStrategy: cloudprofile.Strategy,
		DeleteStrategy: cloudprofile.Strategy,

		TableConvertor: newTableConvertor(),
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter}
	if err := store.CompleteWithOptions(options); err != nil {
		panic(err)
	}

	statusStore := *store
	statusStore.UpdateStrategy = cloudprofile.StatusStrategy
	return &REST{store}, &StatusREST{store: &statusStore}
}

// Implement ShortNamesProvider
var _ rest.ShortNamesProvider = &REST{}

// ShortNames implements the ShortNamesProvider interface. Returns a list of short names for a resource.
func (r *REST) ShortNames() []string {
	return []string{"cprofile", "cpfl"}
}

// StatusREST implements the REST endpoint for changing the status of a CloudProfile.
type StatusREST struct {
	store *genericregistry.Store
}

var (
	_ rest.Storage = &StatusREST{}
	_ rest.Getter  = &StatusREST{}
	_ rest.Updater = &StatusREST{}
)

// New creates a new (empty) internal CloudProfile object.
func (r *StatusREST) New() runtime.Object {
	return &core.CloudProfile{}
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

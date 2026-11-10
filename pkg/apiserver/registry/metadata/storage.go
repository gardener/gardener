// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

// Package metadata provides a generic REST storage implementation for the metadata subresource. The metadata
// subresource behaves like the status subresource: it serves and accepts the full object, but only object
// metadata (labels, annotations, finalizers, owner references, ...) is persisted. It is implemented once and
// reused for all resources served by the gardener-apiserver instead of being duplicated per resource.
package metadata

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
)

// REST implements the metadata subresource for any resource backed by a *genericregistry.Store.
type REST struct {
	store *genericregistry.Store
}

var (
	_ rest.Storage = &REST{}
	_ rest.Getter  = &REST{}
	_ rest.Updater = &REST{}
)

// NewREST returns a REST storage for the metadata subresource sharing the given parent store. The returned
// storage uses an update strategy that only permits changes to object metadata.
func NewREST(parent *genericregistry.Store) *REST {
	metadataStore := *parent
	metadataStore.UpdateStrategy = newStrategy(parent.UpdateStrategy)
	return &REST{store: &metadataStore}
}

// New creates a new (empty) object of the parent resource's type.
func (r *REST) New() runtime.Object {
	return r.store.New()
}

// Destroy cleans up its resources on shutdown. The underlying store is shared with the parent resource's REST
// storage, so it is not destroyed here explicitly.
func (r *REST) Destroy() {}

// Get retrieves the object from the storage. It is required to support Patch.
func (r *REST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return r.store.Get(ctx, name, options)
}

// Update alters the metadata subset of an object.
func (r *REST) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	return r.store.Update(ctx, name, objInfo, createValidation, updateValidation, forceAllowCreate, options)
}

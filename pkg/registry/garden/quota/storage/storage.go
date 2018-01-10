// Copyright 2018 The Gardener Authors.
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
	"github.com/gardener/gardener/pkg/apis/garden"
	"github.com/gardener/gardener/pkg/registry/garden/quota"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
)

// REST implements a RESTStorage for Quota
type REST struct {
	*genericregistry.Store
}

// QuotaStorage implements the storage for Quotas and their status subresource.
type QuotaStorage struct {
	Quota  *REST
	Status *StatusREST
}

// NewStorage creates a new QuotaStorage object.
func NewStorage(optsGetter generic.RESTOptionsGetter) QuotaStorage {
	quotaRest, quotaStatusRest := NewREST(optsGetter)

	return QuotaStorage{
		Quota:  quotaRest,
		Status: quotaStatusRest,
	}
}

// NewREST returns a RESTStorage object that will work with Quota objects.
func NewREST(optsGetter generic.RESTOptionsGetter) (*REST, *StatusREST) {
	store := &genericregistry.Store{
		NewFunc:                  func() runtime.Object { return &garden.Quota{} },
		NewListFunc:              func() runtime.Object { return &garden.QuotaList{} },
		DefaultQualifiedResource: garden.Resource("quotas"),
		EnableGarbageCollection:  true,

		CreateStrategy: quota.Strategy,
		UpdateStrategy: quota.Strategy,
		DeleteStrategy: quota.Strategy,
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter}
	if err := store.CompleteWithOptions(options); err != nil {
		panic(err)
	}

	statusStore := *store
	statusStore.UpdateStrategy = quota.StatusStrategy
	return &REST{store}, &StatusREST{store: &statusStore}
}

// Implement ShortNamesProvider
var _ rest.ShortNamesProvider = &REST{}

// ShortNames implements the ShortNamesProvider interface. Returns a list of short names for a resource.
func (r *REST) ShortNames() []string {
	return []string{}
}

// StatusREST implements the REST endpoint for changing the status of a Quota
type StatusREST struct {
	store *genericregistry.Store
}

// New creates a new (empty) internal Quota object.
func (r *StatusREST) New() runtime.Object {
	return &garden.Quota{}
}

// Get retrieves the object from the storage. It is required to support Patch.
func (r *StatusREST) Get(ctx genericapirequest.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return r.store.Get(ctx, name, options)
}

// Update alters the status subset of an object.
func (r *StatusREST) Update(ctx genericapirequest.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc) (runtime.Object, bool, error) {
	return r.store.Update(ctx, name, objInfo, createValidation, updateValidation)
}

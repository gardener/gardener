// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/registry/core/quota"

	"k8s.io/apimachinery/pkg/runtime"
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
	Quota *REST
}

// NewStorage creates a new QuotaStorage object.
func NewStorage(optsGetter generic.RESTOptionsGetter) QuotaStorage {
	quotaRest := NewREST(optsGetter)

	return QuotaStorage{
		Quota: quotaRest,
	}
}

// NewREST returns a RESTStorage object that will work with Quota objects.
func NewREST(optsGetter generic.RESTOptionsGetter) *REST {
	store := &genericregistry.Store{
		NewFunc:                  func() runtime.Object { return &core.Quota{} },
		NewListFunc:              func() runtime.Object { return &core.QuotaList{} },
		DefaultQualifiedResource: core.Resource("quotas"),
		EnableGarbageCollection:  true,

		CreateStrategy: quota.Strategy,
		UpdateStrategy: quota.Strategy,
		DeleteStrategy: quota.Strategy,

		TableConvertor: newTableConvertor(),
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter}
	if err := store.CompleteWithOptions(options); err != nil {
		panic(err)
	}

	return &REST{store}
}

// Implement ShortNamesProvider
var _ rest.ShortNamesProvider = &REST{}

// ShortNames implements the ShortNamesProvider interface. Returns a list of short names for a resource.
func (r *REST) ShortNames() []string {
	return []string{"squota"}
}

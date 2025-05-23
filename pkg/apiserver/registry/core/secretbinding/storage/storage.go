// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apiserver/registry/core/secretbinding"
)

// REST implements a RESTStorage for SecretBinding
type REST struct {
	*genericregistry.Store
}

// SecretBindingStorage implements the storage for SecretBindings.
type SecretBindingStorage struct {
	SecretBinding *REST
}

// NewStorage creates a new SecretBindingStorage object.
func NewStorage(optsGetter generic.RESTOptionsGetter) SecretBindingStorage {
	secretBindingRest := NewREST(optsGetter)

	return SecretBindingStorage{
		SecretBinding: secretBindingRest,
	}
}

// NewREST returns a RESTStorage object that will work with SecretBinding objects.
func NewREST(optsGetter generic.RESTOptionsGetter) *REST {
	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &core.SecretBinding{} },
		NewListFunc:               func() runtime.Object { return &core.SecretBindingList{} },
		DefaultQualifiedResource:  core.Resource("secretbindings"),
		SingularQualifiedResource: core.Resource("secretbinding"),
		EnableGarbageCollection:   true,

		CreateStrategy: secretbinding.Strategy,
		UpdateStrategy: secretbinding.Strategy,
		DeleteStrategy: secretbinding.Strategy,

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
	return []string{"sb"}
}

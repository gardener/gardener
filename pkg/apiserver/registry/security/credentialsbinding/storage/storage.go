// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/gardener/gardener/pkg/apis/security"
	"github.com/gardener/gardener/pkg/apiserver/registry/security/credentialsbinding"
)

// REST implements a RESTStorage for CredentialsBinding.
type REST struct {
	*genericregistry.Store
}

// CredentialsBindingStorage implements the storage for CredentialsBinding.
type CredentialsBindingStorage struct {
	CredentialsBinding *REST
}

// NewStorage creates a new CredentialsBindingStorage object.
func NewStorage(optsGetter generic.RESTOptionsGetter) CredentialsBindingStorage {
	credentialsBindingRest := NewREST(optsGetter)

	return CredentialsBindingStorage{
		CredentialsBinding: credentialsBindingRest,
	}
}

// NewREST returns a RESTStorage object that will work against CredentialsBinding.
func NewREST(optsGetter generic.RESTOptionsGetter) *REST {
	store := &genericregistry.Store{
		NewFunc:     func() runtime.Object { return &security.CredentialsBinding{} },
		NewListFunc: func() runtime.Object { return &security.CredentialsBindingList{} },

		DefaultQualifiedResource:  security.Resource("credentialsbindings"),
		SingularQualifiedResource: security.Resource("credentialsbinding"),
		EnableGarbageCollection:   true,

		CreateStrategy: credentialsbinding.Strategy,
		UpdateStrategy: credentialsbinding.Strategy,
		DeleteStrategy: credentialsbinding.Strategy,

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
	return []string{"cb"}
}

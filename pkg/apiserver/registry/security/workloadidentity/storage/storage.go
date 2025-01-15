// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/gardener/gardener/pkg/apis/security"
	"github.com/gardener/gardener/pkg/apiserver/registry/security/workloadidentity"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	workloadidentityutils "github.com/gardener/gardener/pkg/utils/workloadidentity"
)

// REST implements a RESTStorage for WorkloadIdentity.
type REST struct {
	*genericregistry.Store
}

// WorkloadIdentityStorage implements the storage for WorkloadIdentity.
type WorkloadIdentityStorage struct {
	WorkloadIdentity *REST
	TokenRequest     *TokenRequestREST
}

// NewStorage creates a new WorkloadIdentityStorage object.
func NewStorage(
	optsGetter generic.RESTOptionsGetter,
	tokenIssuer workloadidentityutils.TokenIssuer,
	coreInformerFactory gardencoreinformers.SharedInformerFactory,
) WorkloadIdentityStorage {
	var issuerURL *string
	if tokenIssuer != nil {
		issuerURL = new(string)
		*issuerURL = tokenIssuer.Issuer()
	}
	workloadIdentityRest := NewREST(optsGetter, issuerURL)

	return WorkloadIdentityStorage{
		WorkloadIdentity: workloadIdentityRest,
		TokenRequest:     NewTokenRequestREST(workloadIdentityRest, tokenIssuer, coreInformerFactory),
	}
}

// NewREST returns a RESTStorage object that will work against WorkloadIdentity.
func NewREST(optsGetter generic.RESTOptionsGetter, issuerURL *string) *REST {
	strategy := workloadidentity.NewStrategy(issuerURL)

	store := &genericregistry.Store{
		NewFunc:     func() runtime.Object { return &security.WorkloadIdentity{} },
		NewListFunc: func() runtime.Object { return &security.WorkloadIdentityList{} },

		DefaultQualifiedResource:  security.Resource("workloadidentities"),
		SingularQualifiedResource: security.Resource("workloadidentity"),
		EnableGarbageCollection:   true,

		CreateStrategy: strategy,
		UpdateStrategy: strategy,
		DeleteStrategy: strategy,

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
	return []string{"wi"}
}

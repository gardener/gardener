// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/storage"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apiserver/registry/core/internalsecret"
)

// REST defines the RESTStorage object that will work against internalsecrets.
type REST struct {
	*genericregistry.Store
}

// NewREST returns a RESTStorage object that will work against internalsecrets.
func NewREST(optsGetter generic.RESTOptionsGetter) *REST {
	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &core.InternalSecret{} },
		NewListFunc:               func() runtime.Object { return &core.InternalSecretList{} },
		PredicateFunc:             internalsecret.MatchInternalSecret,
		DefaultQualifiedResource:  core.Resource("internalsecrets"),
		SingularQualifiedResource: core.Resource("internalsecret"),

		CreateStrategy: internalsecret.Strategy,
		UpdateStrategy: internalsecret.Strategy,
		DeleteStrategy: internalsecret.Strategy,

		TableConvertor: newTableConvertor(),
	}
	options := &generic.StoreOptions{
		RESTOptions: optsGetter,
		AttrFunc:    internalsecret.GetAttrs,
		TriggerFunc: map[string]storage.IndexerFunc{core.InternalSecretType: internalsecret.TypeTriggerFunc},
	}

	if err := store.CompleteWithOptions(options); err != nil {
		panic(err)
	}

	return &REST{store}
}

// ShortNames implements the ShortNamesProvider interface. Returns a list of short names for a resource.
func (r *REST) ShortNames() []string {
	return []string{"ins"}
}

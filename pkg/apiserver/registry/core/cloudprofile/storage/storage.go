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
	"github.com/gardener/gardener/pkg/apiserver/registry/core/cloudprofile"
)

// REST implements a RESTStorage for CloudProfile
type REST struct {
	*genericregistry.Store
}

// CloudProfileStorage implements the storage for CloudProfiles.
type CloudProfileStorage struct {
	CloudProfile *REST
}

// NewStorage creates a new CloudProfileStorage object.
func NewStorage(optsGetter generic.RESTOptionsGetter) CloudProfileStorage {
	cloudProfileRest := NewREST(optsGetter)

	return CloudProfileStorage{
		CloudProfile: cloudProfileRest,
	}
}

// NewREST returns a RESTStorage object that will work with CloudProfile objects.
func NewREST(optsGetter generic.RESTOptionsGetter) *REST {
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
	return &REST{store}
}

// Implement ShortNamesProvider
var _ rest.ShortNamesProvider = &REST{}

// ShortNames implements the ShortNamesProvider interface. Returns a list of short names for a resource.
func (r *REST) ShortNames() []string {
	return []string{"cprofile", "cpfl"}
}

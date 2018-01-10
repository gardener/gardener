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
	"github.com/gardener/gardener/pkg/registry/garden/crosssecretbinding"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
)

// REST implements a RESTStorage for CrossSecretBinding
type REST struct {
	*genericregistry.Store
}

// CrossSecretBindingStorage implements the storage for CrossSecretBindings.
type CrossSecretBindingStorage struct {
	CrossSecretBinding *REST
}

// NewStorage creates a new CrossSecretBindingStorage object.
func NewStorage(optsGetter generic.RESTOptionsGetter) CrossSecretBindingStorage {
	crossSecretBindingRest := NewREST(optsGetter)

	return CrossSecretBindingStorage{
		CrossSecretBinding: crossSecretBindingRest,
	}
}

// NewREST returns a RESTStorage object that will work with CrossSecretBinding objects.
func NewREST(optsGetter generic.RESTOptionsGetter) *REST {
	store := &genericregistry.Store{
		NewFunc:                  func() runtime.Object { return &garden.CrossSecretBinding{} },
		NewListFunc:              func() runtime.Object { return &garden.CrossSecretBindingList{} },
		DefaultQualifiedResource: garden.Resource("crosssecretbindings"),
		EnableGarbageCollection:  true,

		CreateStrategy: crosssecretbinding.Strategy,
		UpdateStrategy: crosssecretbinding.Strategy,
		DeleteStrategy: crosssecretbinding.Strategy,
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
	return []string{"csb"}
}

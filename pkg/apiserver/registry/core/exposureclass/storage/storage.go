// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apiserver/registry/core/exposureclass"
)

// REST implements a RESTStorage for ExposureClass.
type REST struct {
	*genericregistry.Store
}

// ExposureClassStorage implements the storage for ExposureClass.
type ExposureClassStorage struct {
	ExposureClass *REST
}

// NewStorage creates a new ExposureClassStorage object.
func NewStorage(optsGetter generic.RESTOptionsGetter) ExposureClassStorage {
	return ExposureClassStorage{
		ExposureClass: NewREST(optsGetter),
	}
}

// NewREST returns a RESTStorage object that will work with ExposureClass objects.
func NewREST(optsGetter generic.RESTOptionsGetter) *REST {
	exposureClassStrategy := exposureclass.NewStrategy()
	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &core.ExposureClass{} },
		NewListFunc:               func() runtime.Object { return &core.ExposureClassList{} },
		DefaultQualifiedResource:  core.Resource("exposureclasses"),
		SingularQualifiedResource: core.Resource("exposureclass"),
		EnableGarbageCollection:   true,

		CreateStrategy: exposureClassStrategy,
		UpdateStrategy: exposureClassStrategy,
		DeleteStrategy: exposureClassStrategy,

		TableConvertor: newTableConvertor(),
	}

	options := &generic.StoreOptions{RESTOptions: optsGetter}
	if err := store.CompleteWithOptions(options); err != nil {
		panic(err)
	}
	return &REST{store}
}

// Implement ShortNamesProvider.
var _ rest.ShortNamesProvider = &REST{}

// ShortNames implements the ShortNamesProvider interface. Returns a list of short names for a resource.
func (r *REST) ShortNames() []string {
	return []string{"expclass", "expcls"}
}

// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package storage

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/storage"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/registry/core/internalsecret"
)

// REST defines the RESTStorage object that will work against internalsecrets.
type REST struct {
	*genericregistry.Store
}

// NewREST returns a RESTStorage object that will work against internalsecrets.
func NewREST(optsGetter generic.RESTOptionsGetter) *REST {
	store := &genericregistry.Store{
		NewFunc:                  func() runtime.Object { return &core.InternalSecret{} },
		NewListFunc:              func() runtime.Object { return &core.InternalSecretList{} },
		PredicateFunc:            internalsecret.MatchInternalSecret,
		DefaultQualifiedResource: core.Resource("internalsecrets"),

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

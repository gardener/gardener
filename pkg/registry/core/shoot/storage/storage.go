// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/internalversion"
	"github.com/gardener/gardener/pkg/registry/core/shoot"
)

// REST implements a RESTStorage for shoots against etcd
type REST struct {
	*genericregistry.Store
}

// ShootStorage implements the storage for Shoots and all their subresources.
type ShootStorage struct {
	Shoot            *REST
	Status           *StatusREST
	AdminKubeconfig  *KubeconfigREST
	ViewerKubeconfig *KubeconfigREST
	Binding          *BindingREST
}

// NewStorage creates a new ShootStorage object.
func NewStorage(
	optsGetter generic.RESTOptionsGetter,
	internalSecretLister gardencorelisters.InternalSecretLister,
	secretLister kubecorev1listers.SecretLister,
	configMapLister kubecorev1listers.ConfigMapLister,
	adminKubeconfigMaxExpiration time.Duration,
	viewerKubeconfigMaxExpiration time.Duration,
	credentialsRotationInterval time.Duration,
) ShootStorage {
	shootRest, shootStatusRest, bindingREST := NewREST(optsGetter, credentialsRotationInterval)

	return ShootStorage{
		Shoot:            shootRest,
		Status:           shootStatusRest,
		Binding:          bindingREST,
		AdminKubeconfig:  NewAdminKubeconfigREST(shootRest, secretLister, internalSecretLister, configMapLister, adminKubeconfigMaxExpiration),
		ViewerKubeconfig: NewViewerKubeconfigREST(shootRest, secretLister, internalSecretLister, configMapLister, viewerKubeconfigMaxExpiration),
	}
}

// NewREST returns a RESTStorage object that will work against shoots.
func NewREST(optsGetter generic.RESTOptionsGetter, credentialsRotationInterval time.Duration) (*REST, *StatusREST, *BindingREST) {
	var (
		shootStrategy = shoot.NewStrategy(credentialsRotationInterval)
		store         = &genericregistry.Store{
			NewFunc:                   func() runtime.Object { return &core.Shoot{} },
			NewListFunc:               func() runtime.Object { return &core.ShootList{} },
			PredicateFunc:             shoot.MatchShoot,
			DefaultQualifiedResource:  core.Resource("shoots"),
			SingularQualifiedResource: core.Resource("shoot"),
			EnableGarbageCollection:   true,

			CreateStrategy: shootStrategy,
			UpdateStrategy: shootStrategy,
			DeleteStrategy: shootStrategy,

			TableConvertor: newTableConvertor(),
		}
		options = &generic.StoreOptions{
			RESTOptions: optsGetter,
			AttrFunc:    shoot.GetAttrs,
			TriggerFunc: map[string]storage.IndexerFunc{core.ShootSeedName: shoot.SeedNameTriggerFunc},
		}
	)

	if err := store.CompleteWithOptions(options); err != nil {
		panic(err)
	}

	statusStore := *store
	statusStore.UpdateStrategy = shoot.NewStatusStrategy()
	bindingStore := *store
	bindingStore.UpdateStrategy = shoot.NewBindingStrategy()
	return &REST{store}, &StatusREST{store: &statusStore}, &BindingREST{store: &bindingStore}
}

// Implement CategoriesProvider
var _ rest.CategoriesProvider = &REST{}

// Categories implements the CategoriesProvider interface. Returns a list of categories a resource is part of.
func (r *REST) Categories() []string {
	return []string{"all"}
}

// StatusREST implements the REST endpoint for changing the status of a Shoot.
type StatusREST struct {
	store *genericregistry.Store
}

var (
	_ rest.Storage = &StatusREST{}
	_ rest.Getter  = &StatusREST{}
	_ rest.Updater = &StatusREST{}
)

// New creates a new (empty) internal Shoot object.
func (r *StatusREST) New() runtime.Object {
	return &core.Shoot{}
}

// Destroy cleans up its resources on shutdown.
func (r *StatusREST) Destroy() {
	// Given that underlying store is shared with REST,
	// we don't destroy it here explicitly.
}

// Get retrieves the object from the storage. It is required to support Patch.
func (r *StatusREST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return r.store.Get(ctx, name, options)
}

// Update alters the status subset of an object.
func (r *StatusREST) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	return r.store.Update(ctx, name, objInfo, createValidation, updateValidation, forceAllowCreate, options)
}

// BindingREST implements the REST endpoint for changing the binding of a Shoot.
type BindingREST struct {
	store *genericregistry.Store
}

var (
	_ rest.Storage = &BindingREST{}
	_ rest.Getter  = &BindingREST{}
	_ rest.Updater = &BindingREST{}
)

// New creates a new (empty) internal Shoot object.
func (r *BindingREST) New() runtime.Object {
	return &core.Shoot{}
}

// Destroy cleans up its resources on shutdown.
func (r *BindingREST) Destroy() {
	// Given that underlying store is shared with REST,
	// we don't destroy it here explicitly.
}

// Get retrieves the object from the storage. It is required to support Patch.
func (r *BindingREST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return r.store.Get(ctx, name, options)
}

// Update alters the binding subset of an object.
func (r *BindingREST) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	return r.store.Update(ctx, name, objInfo, createValidation, updateValidation, forceAllowCreate, options)
}

// Implement ShortNamesProvider
var _ rest.ShortNamesProvider = &REST{}

// ShortNames implements the ShortNamesProvider interface. Returns a list of short names for a resource.
func (r *REST) ShortNames() []string {
	return []string{}
}

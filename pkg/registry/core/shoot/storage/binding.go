/*
Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package storage

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/apis/core"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/util/dryrun"
)

// BindingREST implements the REST endpoint for shoots/binding.
type BindingREST struct {
	store *genericregistry.Store
}

var _ = rest.NamedCreater(&BindingREST{})

// NamespaceScoped fulfill rest.Scoper
func (r *BindingREST) NamespaceScoped() bool {
	return r.store.NamespaceScoped()
}

// New returns and instance of Binding
func (r *BindingREST) New() runtime.Object {
	return &core.Binding{}
}

// Destroy cleans up resources on shutdown.
func (r *BindingREST) Destroy() {
	// Given that underlying store is shared with REST,
	// we don't destroy it here explicitly.
}

// Create binds a shoot to a seed
func (r *BindingREST) Create(ctx context.Context, name string, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (out runtime.Object, err error) {
	binding, ok := obj.(*core.Binding)
	if !ok {
		return nil, errors.NewBadRequest(fmt.Sprintf("not a Binding object: %#v", obj))
	}

	if name != binding.Name {
		return nil, errors.NewBadRequest(fmt.Sprintf("name in URL does not match name in Binding object: %v,%v", name, binding.Name))
	}

	if createValidation != nil {
		if err := createValidation(ctx, binding.DeepCopyObject()); err != nil {
			return nil, err
		}
	}

	shootObj, err := r.store.Get(ctx, name, &metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	shoot, ok := shootObj.(*core.Shoot)
	if !ok {
		return nil, errors.NewInternalError(fmt.Errorf("cannot convert to *core.Shoot object - got type %T", shootObj))
	}

	if err := r.assignShoot(ctx, binding.UID, binding.ResourceVersion, shoot.Name, binding.Target.Name, dryrun.IsDryRun(options.DryRun)); err != nil {
		return nil, fmt.Errorf("failed to create binding: %w", err)
	}

	out = &metav1.Status{Status: metav1.StatusSuccess}
	return
}

func (r *BindingREST) assignShoot(ctx context.Context, shootUID types.UID, shootResourceVersion, shootName string, seedName string, dryRun bool) (err error) {
	shootKey, err := r.store.KeyFunc(ctx, shootName)
	if err != nil {
		return err
	}

	var preconditions *storage.Preconditions
	if shootUID != "" || shootResourceVersion != "" {
		preconditions = &storage.Preconditions{}
		if shootUID != "" {
			preconditions.UID = &shootUID
		}
		if shootResourceVersion != "" {
			preconditions.ResourceVersion = &shootResourceVersion
		}
	}

	return r.store.Storage.GuaranteedUpdate(ctx, shootKey, &core.Shoot{}, false, preconditions, storage.SimpleUpdate(func(obj runtime.Object) (runtime.Object, error) {
		shoot, ok := obj.(*core.Shoot)
		if !ok {
			return nil, fmt.Errorf("unexpected object: %#v", obj)
		}
		if shoot.DeletionTimestamp != nil {
			return nil, fmt.Errorf("shoot %s is being deleted, cannot be assigned to a seed", shoot.Name)
		}

		shoot.Spec.SeedName = &seedName

		return shoot, nil
	}), dryRun, nil)
}

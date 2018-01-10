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

package privatesecretbinding

import (
	"github.com/gardener/gardener/pkg/apis/garden"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

// Registry is an interface for things that know how to store PrivateSecretBindings.
type Registry interface {
	ListPrivateSecretBindings(ctx genericapirequest.Context, options *metainternalversion.ListOptions) (*garden.PrivateSecretBindingList, error)
	WatchPrivateSecretBindings(ctx genericapirequest.Context, options *metainternalversion.ListOptions) (watch.Interface, error)
	GetPrivateSecretBinding(ctx genericapirequest.Context, name string, options *metav1.GetOptions) (*garden.PrivateSecretBinding, error)
	CreatePrivateSecretBinding(ctx genericapirequest.Context, binding *garden.PrivateSecretBinding, createValidation rest.ValidateObjectFunc) (*garden.PrivateSecretBinding, error)
	UpdatePrivateSecretBinding(ctx genericapirequest.Context, binding *garden.PrivateSecretBinding, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc) (*garden.PrivateSecretBinding, error)
	DeletePrivateSecretBinding(ctx genericapirequest.Context, name string) error
}

// storage puts strong typing around storage calls
type storage struct {
	rest.StandardStorage
}

// NewRegistry returns a new Registry interface for the given Storage. Any mismatched
// types will panic.
func NewRegistry(s rest.StandardStorage) Registry {
	return &storage{s}
}

func (s *storage) ListPrivateSecretBindings(ctx genericapirequest.Context, options *metainternalversion.ListOptions) (*garden.PrivateSecretBindingList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}

	return obj.(*garden.PrivateSecretBindingList), err
}

func (s *storage) WatchPrivateSecretBindings(ctx genericapirequest.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
	return s.Watch(ctx, options)
}

func (s *storage) GetPrivateSecretBinding(ctx genericapirequest.Context, name string, options *metav1.GetOptions) (*garden.PrivateSecretBinding, error) {
	obj, err := s.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}

	return obj.(*garden.PrivateSecretBinding), nil
}

func (s *storage) CreatePrivateSecretBinding(ctx genericapirequest.Context, binding *garden.PrivateSecretBinding, createValidation rest.ValidateObjectFunc) (*garden.PrivateSecretBinding, error) {
	obj, err := s.Create(ctx, binding, createValidation, false)
	if err != nil {
		return nil, err
	}

	return obj.(*garden.PrivateSecretBinding), nil
}

func (s *storage) UpdatePrivateSecretBinding(ctx genericapirequest.Context, binding *garden.PrivateSecretBinding, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc) (*garden.PrivateSecretBinding, error) {
	obj, _, err := s.Update(ctx, binding.Name, rest.DefaultUpdatedObjectInfo(binding), createValidation, updateValidation)
	if err != nil {
		return nil, err
	}

	return obj.(*garden.PrivateSecretBinding), nil
}

func (s *storage) DeletePrivateSecretBinding(ctx genericapirequest.Context, name string) error {
	_, _, err := s.Delete(ctx, name, nil)
	return err
}

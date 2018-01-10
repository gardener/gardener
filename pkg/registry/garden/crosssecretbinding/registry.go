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

package crosssecretbinding

import (
	"github.com/gardener/gardener/pkg/apis/garden"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

// Registry is an interface for things that know how to store CrossSecretBindings.
type Registry interface {
	ListCrossSecretBindings(ctx genericapirequest.Context, options *metainternalversion.ListOptions) (*garden.CrossSecretBindingList, error)
	WatchCrossSecretBindings(ctx genericapirequest.Context, options *metainternalversion.ListOptions) (watch.Interface, error)
	GetCrossSecretBinding(ctx genericapirequest.Context, name string, options *metav1.GetOptions) (*garden.CrossSecretBinding, error)
	CreateCrossSecretBinding(ctx genericapirequest.Context, binding *garden.CrossSecretBinding, createValidation rest.ValidateObjectFunc) (*garden.CrossSecretBinding, error)
	UpdateCrossSecretBinding(ctx genericapirequest.Context, binding *garden.CrossSecretBinding, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc) (*garden.CrossSecretBinding, error)
	DeleteCrossSecretBinding(ctx genericapirequest.Context, name string) error
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

func (s *storage) ListCrossSecretBindings(ctx genericapirequest.Context, options *metainternalversion.ListOptions) (*garden.CrossSecretBindingList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}

	return obj.(*garden.CrossSecretBindingList), err
}

func (s *storage) WatchCrossSecretBindings(ctx genericapirequest.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
	return s.Watch(ctx, options)
}

func (s *storage) GetCrossSecretBinding(ctx genericapirequest.Context, name string, options *metav1.GetOptions) (*garden.CrossSecretBinding, error) {
	obj, err := s.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}

	return obj.(*garden.CrossSecretBinding), nil
}

func (s *storage) CreateCrossSecretBinding(ctx genericapirequest.Context, binding *garden.CrossSecretBinding, createValidation rest.ValidateObjectFunc) (*garden.CrossSecretBinding, error) {
	obj, err := s.Create(ctx, binding, createValidation, false)
	if err != nil {
		return nil, err
	}

	return obj.(*garden.CrossSecretBinding), nil
}

func (s *storage) UpdateCrossSecretBinding(ctx genericapirequest.Context, binding *garden.CrossSecretBinding, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc) (*garden.CrossSecretBinding, error) {
	obj, _, err := s.Update(ctx, binding.Name, rest.DefaultUpdatedObjectInfo(binding), createValidation, updateValidation)
	if err != nil {
		return nil, err
	}

	return obj.(*garden.CrossSecretBinding), nil
}

func (s *storage) DeleteCrossSecretBinding(ctx genericapirequest.Context, name string) error {
	_, _, err := s.Delete(ctx, name, nil)
	return err
}

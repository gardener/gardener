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

package shoot

import (
	"fmt"

	"github.com/gardener/gardener/pkg/apis/garden"
	"k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

// Registry is an interface for things that know how to store Shoots.
type Registry interface {
	ListShoots(ctx genericapirequest.Context, options *metainternalversion.ListOptions) (*garden.ShootList, error)
	WatchShoots(ctx genericapirequest.Context, options *metainternalversion.ListOptions) (watch.Interface, error)
	GetShoot(ctx genericapirequest.Context, shootID string, options *metav1.GetOptions) (*garden.Shoot, error)
	CreateShoot(ctx genericapirequest.Context, shoot *garden.Shoot, createValidation rest.ValidateObjectFunc) (*garden.Shoot, error)
	UpdateShoot(ctx genericapirequest.Context, shoot *garden.Shoot, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc) (*garden.Shoot, error)
	DeleteShoot(ctx genericapirequest.Context, shootID string) error
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

func (s *storage) ListShoots(ctx genericapirequest.Context, options *metainternalversion.ListOptions) (*garden.ShootList, error) {
	if options != nil && options.FieldSelector != nil && !options.FieldSelector.Empty() {
		return nil, fmt.Errorf("field selector not supported yet")
	}
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}
	return obj.(*garden.ShootList), err
}

func (s *storage) WatchShoots(ctx genericapirequest.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
	return s.Watch(ctx, options)
}

func (s *storage) GetShoot(ctx genericapirequest.Context, shootID string, options *metav1.GetOptions) (*garden.Shoot, error) {
	obj, err := s.Get(ctx, shootID, options)
	if err != nil {
		return nil, errors.NewNotFound(garden.Resource("shoots"), shootID)
	}
	return obj.(*garden.Shoot), nil
}

func (s *storage) CreateShoot(ctx genericapirequest.Context, shoot *garden.Shoot, createValidation rest.ValidateObjectFunc) (*garden.Shoot, error) {
	obj, err := s.Create(ctx, shoot, rest.ValidateAllObjectFunc, false)
	if err != nil {
		return nil, err
	}
	return obj.(*garden.Shoot), nil
}

func (s *storage) UpdateShoot(ctx genericapirequest.Context, shoot *garden.Shoot, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc) (*garden.Shoot, error) {
	obj, _, err := s.Update(ctx, shoot.Name, rest.DefaultUpdatedObjectInfo(shoot), createValidation, updateValidation)
	if err != nil {
		return nil, err
	}
	return obj.(*garden.Shoot), nil
}

func (s *storage) DeleteShoot(ctx genericapirequest.Context, shootID string) error {
	_, _, err := s.Delete(ctx, shootID, nil)
	return err
}

// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package quota

import (
	"context"

	"github.com/gardener/gardener/pkg/apis/garden"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/registry/rest"
)

// Registry is an interface for things that know how to store Quotas.
type Registry interface {
	ListQuotas(ctx context.Context, options *metainternalversion.ListOptions) (*garden.QuotaList, error)
	WatchQuotas(ctx context.Context, options *metainternalversion.ListOptions) (watch.Interface, error)
	GetQuota(ctx context.Context, name string, options *metav1.GetOptions) (*garden.Quota, error)
	CreateQuota(ctx context.Context, quota *garden.Quota, createValidation rest.ValidateObjectFunc) (*garden.Quota, error)
	UpdateQuota(ctx context.Context, quota *garden.Quota, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc) (*garden.Quota, error)
	DeleteQuota(ctx context.Context, name string) error
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

func (s *storage) ListQuotas(ctx context.Context, options *metainternalversion.ListOptions) (*garden.QuotaList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}

	return obj.(*garden.QuotaList), err
}

func (s *storage) WatchQuotas(ctx context.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
	return s.Watch(ctx, options)
}

func (s *storage) GetQuota(ctx context.Context, name string, options *metav1.GetOptions) (*garden.Quota, error) {
	obj, err := s.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}

	return obj.(*garden.Quota), nil
}

func (s *storage) CreateQuota(ctx context.Context, quota *garden.Quota, createValidation rest.ValidateObjectFunc) (*garden.Quota, error) {
	obj, err := s.Create(ctx, quota, createValidation, &metav1.CreateOptions{IncludeUninitialized: false})
	if err != nil {
		return nil, err
	}

	return obj.(*garden.Quota), nil
}

func (s *storage) UpdateQuota(ctx context.Context, quota *garden.Quota, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc) (*garden.Quota, error) {
	obj, _, err := s.Update(ctx, quota.Name, rest.DefaultUpdatedObjectInfo(quota), createValidation, updateValidation, false, &metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return obj.(*garden.Quota), nil
}

func (s *storage) DeleteQuota(ctx context.Context, name string) error {
	_, _, err := s.Delete(ctx, name, nil)
	return err
}

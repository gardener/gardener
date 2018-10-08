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

package backupinfrastructure

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/apis/garden"
	"k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/registry/rest"
)

// Registry is an interface for things that know how to store BackupInfrastructures.
type Registry interface {
	ListBackupInfrastructures(ctx context.Context, options *metainternalversion.ListOptions) (*garden.BackupInfrastructureList, error)
	WatchBackupInfrastructures(ctx context.Context, options *metainternalversion.ListOptions) (watch.Interface, error)
	GetBackupInfrastructure(ctx context.Context, backupInfrastructureID string, options *metav1.GetOptions) (*garden.BackupInfrastructure, error)
	CreateBackupInfrastructure(ctx context.Context, backupInfrastructure *garden.BackupInfrastructure, createValidation rest.ValidateObjectFunc) (*garden.BackupInfrastructure, error)
	UpdateBackupInfrastructure(ctx context.Context, backupInfrastructure *garden.BackupInfrastructure, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc) (*garden.BackupInfrastructure, error)
	DeleteBackupInfrastructure(ctx context.Context, backupInfrastructureID string) error
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

func (s *storage) ListBackupInfrastructures(ctx context.Context, options *metainternalversion.ListOptions) (*garden.BackupInfrastructureList, error) {
	if options != nil && options.FieldSelector != nil && !options.FieldSelector.Empty() {
		return nil, fmt.Errorf("field selector not supported yet")
	}
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}
	return obj.(*garden.BackupInfrastructureList), err
}

func (s *storage) WatchBackupInfrastructures(ctx context.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
	return s.Watch(ctx, options)
}

func (s *storage) GetBackupInfrastructure(ctx context.Context, backupInfrastructureID string, options *metav1.GetOptions) (*garden.BackupInfrastructure, error) {
	obj, err := s.Get(ctx, backupInfrastructureID, options)
	if err != nil {
		return nil, errors.NewNotFound(garden.Resource("backupinfrastructures"), backupInfrastructureID)
	}
	return obj.(*garden.BackupInfrastructure), nil
}

func (s *storage) CreateBackupInfrastructure(ctx context.Context, backupInfrastructure *garden.BackupInfrastructure, createValidation rest.ValidateObjectFunc) (*garden.BackupInfrastructure, error) {
	obj, err := s.Create(ctx, backupInfrastructure, rest.ValidateAllObjectFunc, &metav1.CreateOptions{IncludeUninitialized: false})
	if err != nil {
		return nil, err
	}
	return obj.(*garden.BackupInfrastructure), nil
}

func (s *storage) UpdateBackupInfrastructure(ctx context.Context, backupInfrastructure *garden.BackupInfrastructure, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc) (*garden.BackupInfrastructure, error) {
	obj, _, err := s.Update(ctx, backupInfrastructure.Name, rest.DefaultUpdatedObjectInfo(backupInfrastructure), createValidation, updateValidation, false, &metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	return obj.(*garden.BackupInfrastructure), nil
}

func (s *storage) DeleteBackupInfrastructure(ctx context.Context, backupInfrastructureID string) error {
	_, _, err := s.Delete(ctx, backupInfrastructureID, nil)
	return err
}

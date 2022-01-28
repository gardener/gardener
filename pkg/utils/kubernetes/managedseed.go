// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetes

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
)

// GetManagedSeedWithReader gets the ManagedSeed resource for the given shoot namespace and name,
// by searching for all ManagedSeeds in the shoot namespace that have spec.shoot.name set to the shoot name.
// If no such ManagedSeeds are found, nil is returned.
func GetManagedSeedWithReader(ctx context.Context, r client.Reader, shootNamespace, shootName string) (*seedmanagementv1alpha1.ManagedSeed, error) {
	managedSeedList := &seedmanagementv1alpha1.ManagedSeedList{}
	if err := r.List(ctx, managedSeedList, client.InNamespace(shootNamespace), client.MatchingFields{seedmanagement.ManagedSeedShootName: shootName}); err != nil {
		return nil, err
	}
	if len(managedSeedList.Items) == 0 {
		return nil, nil
	}
	if len(managedSeedList.Items) > 1 {
		return nil, fmt.Errorf("found more than one ManagedSeed objects for shoot %s/%s", shootNamespace, shootName)
	}
	return &managedSeedList.Items[0], nil
}

// GetManagedSeedByName tries to reads a ManagedSeed in the garden namespace. If it's not found then `nil` is returned.
func GetManagedSeedByName(ctx context.Context, client client.Client, name string) (*seedmanagementv1alpha1.ManagedSeed, error) {
	managedSeed := &seedmanagementv1alpha1.ManagedSeed{}
	if err := client.Get(ctx, Key(constants.GardenNamespace, name), managedSeed); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return managedSeed, nil
}

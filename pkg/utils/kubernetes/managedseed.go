// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
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

// GetManagedSeedByName tries to read a ManagedSeed in the garden namespace. If it's not found then `nil` is returned.
func GetManagedSeedByName(ctx context.Context, c client.Client, name string) (*seedmanagementv1alpha1.ManagedSeed, error) {
	managedSeed := &seedmanagementv1alpha1.ManagedSeed{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: name}, managedSeed); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return managedSeed, nil
}

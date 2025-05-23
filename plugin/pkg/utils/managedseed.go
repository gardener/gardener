// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"

	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/seedmanagement/clientset/versioned"
)

// GetManagedSeed gets the ManagedSeed resource for the given shoot namespace and name,
// by searching for all ManagedSeeds in the shoot namespace that have spec.shoot.name set to the shoot name.
// If no such ManagedSeeds are found, nil is returned.
func GetManagedSeed(ctx context.Context, seedManagementClient versioned.Interface, shootNamespace, shootName string) (*seedmanagementv1alpha1.ManagedSeed, error) {
	managedSeedList, err := seedManagementClient.SeedmanagementV1alpha1().ManagedSeeds(shootNamespace).List(ctx, metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{seedmanagement.ManagedSeedShootName: shootName}).String(),
	})
	if err != nil {
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

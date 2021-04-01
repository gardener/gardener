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

package managedseedset

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllermanagerfeatures "github.com/gardener/gardener/pkg/controllermanager/features"
	"github.com/gardener/gardener/pkg/features"
)

// ReplicaGetter provides a method for getting all existing replicas of a ManagedSeedSet.
type ReplicaGetter interface {
	// GetReplicas gets and returns all existing replicas of the given set.
	GetReplicas(context.Context, *seedmanagementv1alpha1.ManagedSeedSet) ([]Replica, error)
}

// NewReplicaGetter creates and returns a new ReplicaGetter with the given parameters.
func NewReplicaGetter(client kubernetes.Interface, replicaFactory ReplicaFactory) ReplicaGetter {
	return &replicaGetter{
		client:         client,
		replicaFactory: replicaFactory,
	}
}

// replicaGetter is a concrete implementation of ReplicaGetter.
type replicaGetter struct {
	client         kubernetes.Interface
	replicaFactory ReplicaFactory
}

// GetReplicas gets and returns all existing replicas of the given set.
func (rg *replicaGetter) GetReplicas(ctx context.Context, set *seedmanagementv1alpha1.ManagedSeedSet) ([]Replica, error) {
	// Convert spec.selector to labels.Selector
	selector, err := metav1.LabelSelectorAsSelector(&set.Spec.Selector)
	if err != nil {
		return nil, err
	}

	// Get managed seeds, shoots, and seeds in set's namespace matching selector
	shoots := &gardencorev1beta1.ShootList{}
	if err := rg.client.Client().List(ctx, shoots, client.InNamespace(set.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return nil, err
	}
	managedSeeds := &seedmanagementv1alpha1.ManagedSeedList{}
	if err := rg.client.Client().List(ctx, managedSeeds, client.InNamespace(set.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return nil, err
	}
	seeds := &gardencorev1beta1.SeedList{}
	if err := rg.client.Client().List(ctx, seeds, client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return nil, err
	}

	// If CachedRuntimeClients feature gate is enabled, cross-check number of shoots with a partial metadata list
	// from the API server to ensure what we got from the cache is up-to-date
	if controllermanagerfeatures.FeatureGate.Enabled(features.CachedRuntimeClients) {
		shoots2 := &metav1.PartialObjectMetadataList{}
		shoots2.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("ShootList"))
		if err := rg.client.APIReader().List(ctx, shoots2, client.InNamespace(set.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
			return nil, err
		}
		if len(shoots2.Items) != len(shoots.Items) {
			return nil, fmt.Errorf("cross-checking number of shoots failed")
		}
	}

	// Map names to objects for managed seeds and seeds
	managedSeedsByName := make(map[string]*seedmanagementv1alpha1.ManagedSeed)
	for i, managedSeed := range managedSeeds.Items {
		managedSeedsByName[managedSeed.Name] = &managedSeeds.Items[i]
	}
	seedsByName := make(map[string]*gardencorev1beta1.Seed)
	for i, seed := range seeds.Items {
		seedsByName[seed.Name] = &seeds.Items[i]
	}

	// Initialize replicas
	var replicas []Replica
	for i, shoot := range shoots.Items {
		// Get shoots scheduled onto this seed
		hasScheduledShoots, err := rg.hasScheduledShoots(ctx, seedsByName[shoot.Name])
		if err != nil {
			return nil, err
		}

		// Add new replica
		r := rg.replicaFactory.NewReplica(set, &shoots.Items[i], managedSeedsByName[shoot.Name], seedsByName[shoot.Name], hasScheduledShoots)
		replicas = append(replicas, r)
	}

	return replicas, nil
}

func (rg *replicaGetter) hasScheduledShoots(ctx context.Context, seed *gardencorev1beta1.Seed) (bool, error) {
	if seed != nil {
		shoots := &metav1.PartialObjectMetadataList{}
		shoots.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("ShootList"))
		if err := rg.client.APIReader().List(ctx, shoots, client.MatchingFields{gardencore.ShootSeedName: seed.Name}, client.Limit(1)); err != nil {
			return false, err
		}
		return len(shoots.Items) > 0, nil
	}
	return false, nil
}

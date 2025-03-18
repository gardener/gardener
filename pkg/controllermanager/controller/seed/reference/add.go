// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reference

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/controller/reference"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
)

// AddToManager adds the seed-reference controller to the given manager.
func AddToManager(mgr manager.Manager, gardenNamespace string, cfg controllermanagerconfigv1alpha1.SeedReferenceControllerConfiguration) error {
	return (&reference.Reconciler{
		ConcurrentSyncs:             cfg.ConcurrentSyncs,
		NewObjectFunc:               func() client.Object { return &gardencorev1beta1.Seed{} },
		NewObjectListFunc:           func() client.ObjectList { return &gardencorev1beta1.SeedList{} },
		GetNamespace:                func(_ client.Object) string { return gardenNamespace },
		GetReferencedSecretNames:    getReferencedSecretNames,
		GetReferencedConfigMapNames: getReferencedConfigMapNames,
		ReferenceChangedPredicate:   Predicate,
	}).AddToManager(mgr)
}

// Predicate is a predicate function for checking whether a reference changed in the Seed
// specification.
func Predicate(oldObj, newObj client.Object) bool {
	newSeed, ok := newObj.(*gardencorev1beta1.Seed)
	if !ok {
		return false
	}

	oldSeed, ok := oldObj.(*gardencorev1beta1.Seed)
	if !ok {
		return false
	}

	return !v1beta1helper.ResourceReferencesEqual(oldSeed.Spec.Resources, newSeed.Spec.Resources)
}

func getReferencedSecretNames(obj client.Object) []string {
	seed, ok := obj.(*gardencorev1beta1.Seed)
	if !ok {
		return nil
	}

	return namesForReferencedResources(seed, "Secret")
}

func getReferencedConfigMapNames(obj client.Object) []string {
	seed, ok := obj.(*gardencorev1beta1.Seed)
	if !ok {
		return nil
	}

	return namesForReferencedResources(seed, "ConfigMap")
}

func namesForReferencedResources(seed *gardencorev1beta1.Seed, kind string) []string {
	var names []string
	for _, ref := range seed.Spec.Resources {
		if ref.ResourceRef.APIVersion == "v1" && ref.ResourceRef.Kind == kind {
			names = append(names, ref.ResourceRef.Name)
		}
	}
	return names
}

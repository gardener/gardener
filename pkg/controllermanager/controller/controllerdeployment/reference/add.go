// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reference

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	v1helper "github.com/gardener/gardener/pkg/api/core/v1/helper"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/controllermanager/v1alpha1"
	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controller/reference"
)

// AddToManager adds the controllerdeployment-reference controller to the given manager.
func AddToManager(mgr manager.Manager, cfg controllermanagerconfigv1alpha1.ControllerDeploymentReferenceControllerConfiguration) error {
	return (&reference.Reconciler{
		ConcurrentSyncs:             cfg.ConcurrentSyncs,
		NewObjectFunc:               func() client.Object { return &gardencorev1.ControllerDeployment{} },
		NewObjectListFunc:           func() client.ObjectList { return &gardencorev1.ControllerDeploymentList{} },
		GetNamespace:                func(_ client.Object) string { return constants.GardenNamespace },
		GetReferencedSecretNames:    getReferencedSecretNames,
		GetReferencedConfigMapNames: getReferencedConfigMapNames,
		ReferenceChangedPredicate:   Predicate,
	}).AddToManager(mgr, "controllerdeployment")
}

// Predicate is a predicate function for checking whether a reference changed in the ControllerDeployment
// specification.
func Predicate(oldObj, newObj client.Object) bool {
	newControllerDeployment, ok := newObj.(*gardencorev1.ControllerDeployment)
	if !ok {
		return false
	}

	oldControllerDeployment, ok := oldObj.(*gardencorev1.ControllerDeployment)
	if !ok {
		return false
	}

	return !v1helper.ResourceReferencesEqual(oldControllerDeployment.Resources, newControllerDeployment.Resources)
}

func getReferencedSecretNames(obj client.Object) []string {
	controllerDeployment, ok := obj.(*gardencorev1.ControllerDeployment)
	if !ok {
		return nil
	}

	return namesForReferencedResources(controllerDeployment, corev1.SchemeGroupVersion.String(), "Secret")
}

func getReferencedConfigMapNames(obj client.Object) []string {
	controllerDeployment, ok := obj.(*gardencorev1.ControllerDeployment)
	if !ok {
		return nil
	}

	return namesForReferencedResources(controllerDeployment, corev1.SchemeGroupVersion.String(), "ConfigMap")
}

func namesForReferencedResources(controllerDeployment *gardencorev1.ControllerDeployment, apiVersion, kind string) []string {
	var names []string
	for _, ref := range controllerDeployment.Resources {
		if ref.ResourceRef.APIVersion == apiVersion && ref.ResourceRef.Kind == kind {
			names = append(names, ref.ResourceRef.Name)
		}
	}
	return names
}

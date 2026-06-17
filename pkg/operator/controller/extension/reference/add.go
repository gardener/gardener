// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reference

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	v1helper "github.com/gardener/gardener/pkg/api/core/v1/helper"
	operatorconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/controller/reference"
)

// AddToManager adds the extension-reference controller to the given manager.
func AddToManager(mgr manager.Manager, cfg operatorconfigv1alpha1.ExtensionReferenceControllerConfiguration) error {
	return (&reference.Reconciler{
		ConcurrentSyncs:             cfg.ConcurrentSyncs,
		NewObjectFunc:               func() client.Object { return &operatorv1alpha1.Extension{} },
		NewObjectListFunc:           func() client.ObjectList { return &operatorv1alpha1.ExtensionList{} },
		GetNamespace:                func(_ client.Object) string { return constants.GardenNamespace },
		GetReferencedSecretNames:    getReferencedSecretNames,
		GetReferencedConfigMapNames: getReferencedConfigMapNames,
		ReferenceChangedPredicate:   Predicate,
	}).AddToManager(mgr, "extension")
}

// Predicate is a predicate function for checking whether a reference changed in the Extension specification.
func Predicate(oldObj, newObj client.Object) bool {
	newExtension, ok := newObj.(*operatorv1alpha1.Extension)
	if !ok {
		return false
	}

	oldExtension, ok := oldObj.(*operatorv1alpha1.Extension)
	if !ok {
		return false
	}

	return !v1helper.ResourceReferencesEqual(oldExtension.Spec.Deployment.Resources, newExtension.Spec.Deployment.Resources)
}

func getReferencedSecretNames(obj client.Object) []string {
	extension, ok := obj.(*operatorv1alpha1.Extension)
	if !ok {
		return nil
	}

	return namesForReferencedResources(extension, corev1.SchemeGroupVersion.String(), "Secret")
}

func getReferencedConfigMapNames(obj client.Object) []string {
	extension, ok := obj.(*operatorv1alpha1.Extension)
	if !ok {
		return nil
	}

	return namesForReferencedResources(extension, corev1.SchemeGroupVersion.String(), "ConfigMap")
}

func namesForReferencedResources(extension *operatorv1alpha1.Extension, apiVersion, kind string) []string {
	var names []string
	for _, ref := range extension.Spec.Deployment.Resources {
		if ref.ResourceRef.APIVersion == apiVersion && ref.ResourceRef.Kind == kind {
			names = append(names, ref.ResourceRef.Name)
		}
	}
	return names
}

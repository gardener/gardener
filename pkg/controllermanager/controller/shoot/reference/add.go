// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package reference

import (
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/controller/reference"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
)

// AddToManager adds the shoot-reference controller to the given manager.
func AddToManager(mgr manager.Manager, cfg config.ShootReferenceControllerConfiguration) error {
	return (&reference.Reconciler{
		ConcurrentSyncs:             cfg.ConcurrentSyncs,
		NewObjectFunc:               func() client.Object { return &gardencorev1beta1.Shoot{} },
		NewObjectListFunc:           func() client.ObjectList { return &gardencorev1beta1.ShootList{} },
		GetNamespace:                func(obj client.Object) string { return obj.GetNamespace() },
		GetReferencedSecretNames:    getReferencedSecretNames,
		GetReferencedConfigMapNames: getReferencedConfigMapNames,
		ReferenceChangedPredicate:   Predicate,
	}).AddToManager(mgr)
}

// Predicate is a predicate function for checking whether a reference changed in the Shoot
// specification.
func Predicate(oldObj, newObj client.Object) bool {
	newShoot, ok := newObj.(*gardencorev1beta1.Shoot)
	if !ok {
		return false
	}

	oldShoot, ok := oldObj.(*gardencorev1beta1.Shoot)
	if !ok {
		return false
	}

	return shootDNSFieldChanged(oldShoot, newShoot) ||
		shootKubeAPIServerAuditConfigFieldChanged(oldShoot, newShoot) || !v1beta1helper.ShootResourceReferencesEqual(oldShoot.Spec.Resources, newShoot.Spec.Resources)
}

func shootDNSFieldChanged(oldShoot, newShoot *gardencorev1beta1.Shoot) bool {
	return !apiequality.Semantic.Equalities.DeepEqual(oldShoot.Spec.DNS, newShoot.Spec.DNS)
}

func shootKubeAPIServerAuditConfigFieldChanged(oldShoot, newShoot *gardencorev1beta1.Shoot) bool {
	return !apiequality.Semantic.Equalities.DeepEqual(oldShoot.Spec.Kubernetes.KubeAPIServer.AuditConfig, newShoot.Spec.Kubernetes.KubeAPIServer.AuditConfig)
}

func getReferencedSecretNames(obj client.Object) []string {
	shoot, ok := obj.(*gardencorev1beta1.Shoot)
	if !ok {
		return nil
	}

	var out []string
	out = append(out, secretNamesForDNSProviders(shoot)...)
	out = append(out, namesForReferencedResources(shoot, "Secret")...)
	return out
}

func getReferencedConfigMapNames(obj client.Object) []string {
	shoot, ok := obj.(*gardencorev1beta1.Shoot)
	if !ok {
		return nil
	}

	var out []string
	if configMapRef := getAuditPolicyConfigMapRef(shoot.Spec.Kubernetes.KubeAPIServer); configMapRef != nil {
		out = append(out, configMapRef.Name)
	}
	out = append(out, namesForReferencedResources(shoot, "ConfigMap")...)
	return out
}

func secretNamesForDNSProviders(shoot *gardencorev1beta1.Shoot) []string {
	if shoot.Spec.DNS == nil {
		return nil
	}
	var names = make([]string, 0, len(shoot.Spec.DNS.Providers))
	for _, provider := range shoot.Spec.DNS.Providers {
		if provider.SecretName == nil {
			continue
		}
		names = append(names, *provider.SecretName)
	}

	return names
}

func namesForReferencedResources(shoot *gardencorev1beta1.Shoot, kind string) []string {
	var names []string
	for _, ref := range shoot.Spec.Resources {
		if ref.ResourceRef.APIVersion == "v1" && ref.ResourceRef.Kind == kind {
			names = append(names, ref.ResourceRef.Name)
		}
	}
	return names
}

func getAuditPolicyConfigMapRef(apiServerConfig *gardencorev1beta1.KubeAPIServerConfig) *corev1.ObjectReference {
	if apiServerConfig != nil &&
		apiServerConfig.AuditConfig != nil &&
		apiServerConfig.AuditConfig.AuditPolicy != nil &&
		apiServerConfig.AuditConfig.AuditPolicy.ConfigMapRef != nil {

		return apiServerConfig.AuditConfig.AuditPolicy.ConfigMapRef
	}

	return nil
}

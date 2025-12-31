// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reference

import (
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/controller/reference"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
)

// AddToManager adds the shoot-reference controller to the given manager.
func AddToManager(mgr manager.Manager, cfg controllermanagerconfigv1alpha1.ShootReferenceControllerConfiguration) error {
	return (&reference.Reconciler{
		ConcurrentSyncs:                    cfg.ConcurrentSyncs,
		NewObjectFunc:                      func() client.Object { return &gardencorev1beta1.Shoot{} },
		NewObjectListFunc:                  func() client.ObjectList { return &gardencorev1beta1.ShootList{} },
		GetNamespace:                       func(obj client.Object) string { return obj.GetNamespace() },
		GetReferencedSecretNames:           getReferencedSecretNames,
		GetReferencedConfigMapNames:        getReferencedConfigMapNames,
		GetReferencedWorkloadIdentityNames: getReferencedWorkloadIdentityNames,
		ReferenceChangedPredicate:          Predicate,
	}).AddToManager(mgr, "shoot")
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

	return !apiequality.Semantic.DeepEqual(oldShoot.Spec.DNS, newShoot.Spec.DNS) ||
		!apiequality.Semantic.DeepEqual(oldShoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins, newShoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins) ||
		!apiequality.Semantic.DeepEqual(oldShoot.Spec.Kubernetes.KubeAPIServer.AuditConfig, newShoot.Spec.Kubernetes.KubeAPIServer.AuditConfig) ||
		!apiequality.Semantic.DeepEqual(oldShoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthentication, newShoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthentication) ||
		!apiequality.Semantic.DeepEqual(oldShoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthorization, newShoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthorization) ||
		!v1beta1helper.ResourceReferencesEqual(oldShoot.Spec.Resources, newShoot.Spec.Resources)
}

func getReferencedSecretNames(obj client.Object) []string {
	shoot, ok := obj.(*gardencorev1beta1.Shoot)
	if !ok {
		return nil
	}

	var out []string
	out = append(out, namesForDNSProviderCredentials(shoot, corev1.SchemeGroupVersion.String(), "Secret")...)
	out = append(out, secretNamesForAdmissionPlugins(shoot)...)
	out = append(out, secretNamesForStructuredAuthorization(shoot)...)
	out = append(out, namesForReferencedResources(shoot, corev1.SchemeGroupVersion.String(), "Secret")...)
	return out
}

func getReferencedConfigMapNames(obj client.Object) []string {
	shoot, ok := obj.(*gardencorev1beta1.Shoot)
	if !ok {
		return nil
	}

	var out []string
	if configMapName := v1beta1helper.GetShootAuditPolicyConfigMapName(shoot.Spec.Kubernetes.KubeAPIServer); configMapName != "" {
		out = append(out, configMapName)
	}
	if configMapName := v1beta1helper.GetShootAuthenticationConfigurationConfigMapName(shoot.Spec.Kubernetes.KubeAPIServer); configMapName != "" {
		out = append(out, configMapName)
	}
	if configMapName := v1beta1helper.GetShootAuthorizationConfigurationConfigMapName(shoot.Spec.Kubernetes.KubeAPIServer); configMapName != "" {
		out = append(out, configMapName)
	}
	out = append(out, namesForReferencedResources(shoot, corev1.SchemeGroupVersion.String(), "ConfigMap")...)
	return out
}

func getReferencedWorkloadIdentityNames(obj client.Object) []string {
	shoot, ok := obj.(*gardencorev1beta1.Shoot)
	if !ok {
		return nil
	}

	var out []string
	out = append(out, namesForReferencedResources(shoot, securityv1alpha1.SchemeGroupVersion.String(), "WorkloadIdentity")...)
	out = append(out, namesForDNSProviderCredentials(shoot, securityv1alpha1.SchemeGroupVersion.String(), "WorkloadIdentity")...)
	return out
}

func namesForDNSProviderCredentials(shoot *gardencorev1beta1.Shoot, apiVersion, kind string) []string {
	if shoot.Spec.DNS == nil {
		return nil
	}

	var names = make([]string, 0, len(shoot.Spec.DNS.Providers))
	for _, provider := range shoot.Spec.DNS.Providers {
		if provider.CredentialsRef == nil {
			continue
		}
		if provider.CredentialsRef.APIVersion != apiVersion || provider.CredentialsRef.Kind != kind {
			continue
		}
		names = append(names, provider.CredentialsRef.Name)
	}

	return names
}

func secretNamesForAdmissionPlugins(shoot *gardencorev1beta1.Shoot) []string {
	if shoot.Spec.Kubernetes.KubeAPIServer == nil {
		return nil
	}

	var names []string
	for _, plugin := range shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins {
		if plugin.KubeconfigSecretName != nil {
			names = append(names, *plugin.KubeconfigSecretName)
		}
	}

	return names
}

func secretNamesForStructuredAuthorization(shoot *gardencorev1beta1.Shoot) []string {
	if shoot.Spec.Kubernetes.KubeAPIServer == nil || shoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthorization == nil {
		return nil
	}

	var names = make([]string, 0, len(shoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthorization.Kubeconfigs))
	for _, kubeconfig := range shoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthorization.Kubeconfigs {
		names = append(names, kubeconfig.SecretName)
	}

	return names
}

func namesForReferencedResources(shoot *gardencorev1beta1.Shoot, apiVersion, kind string) []string {
	var names []string
	for _, ref := range shoot.Spec.Resources {
		if ref.ResourceRef.APIVersion == apiVersion && ref.ResourceRef.Kind == kind {
			names = append(names, ref.ResourceRef.Name)
		}
	}
	return names
}

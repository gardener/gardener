// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reference

import (
	"reflect"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/controller/reference"
)

// AddToManager adds the garden-reference controller to the given manager.
func AddToManager(mgr manager.Manager, gardenNamespace string) error {
	return (&reference.Reconciler{
		ConcurrentSyncs:             ptr.To(1),
		NewObjectFunc:               func() client.Object { return &operatorv1alpha1.Garden{} },
		NewObjectListFunc:           func() client.ObjectList { return &operatorv1alpha1.GardenList{} },
		GetNamespace:                func(client.Object) string { return gardenNamespace },
		GetReferencedSecretNames:    getReferencedSecretNames,
		GetReferencedConfigMapNames: getReferencedConfigMapNames,
		ReferenceChangedPredicate:   Predicate,
	}).AddToManager(mgr, "garden")
}

// Predicate is a predicate function for checking whether a reference changed in the Garden specification.
func Predicate(oldObj, newObj client.Object) bool {
	newGarden, ok := newObj.(*operatorv1alpha1.Garden)
	if !ok {
		return false
	}

	oldGarden, ok := oldObj.(*operatorv1alpha1.Garden)
	if !ok {
		return false
	}

	return kubeAPIServerAuditPolicyConfigMapChanged(oldGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer, newGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer) ||
		gardenerAPIServerAuditPolicyConfigMapChanged(oldGarden.Spec.VirtualCluster.Gardener.APIServer, newGarden.Spec.VirtualCluster.Gardener.APIServer) ||
		etcdBackupSecretChanged(oldGarden.Spec.VirtualCluster.ETCD, newGarden.Spec.VirtualCluster.ETCD) ||
		dnsSecretsChanged(oldGarden.Spec.DNS, newGarden.Spec.DNS) ||
		authenticationWebhookSecretChanged(oldGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer, newGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer) ||
		sniSecretChanged(oldGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer, newGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer) ||
		kubeAPIServerAuditWebhookSecretChanged(oldGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer, newGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer) ||
		gardenerAPIServerAuditWebhookSecretChanged(oldGarden.Spec.VirtualCluster.Gardener.APIServer, newGarden.Spec.VirtualCluster.Gardener.APIServer) ||
		kubeAPIServerAdmissionPluginSecretsChanged(oldGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer, newGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer) ||
		gardenerAPIServerAdmissionPluginSecretChanged(oldGarden.Spec.VirtualCluster.Gardener.APIServer, newGarden.Spec.VirtualCluster.Gardener.APIServer) ||
		kubeAPIServerStructuredAuthenticationConfigMapChanged(oldGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer, newGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer) ||
		kubeAPIServerStructuredAuthorizationConfigMapChanged(oldGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer, newGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer) ||
		kubeAPIServerStructuredAuthorizationSecretsChanged(oldGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer, newGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer)
}

func kubeAPIServerAuditPolicyConfigMapChanged(oldKubeAPIServer, newKubeAPIServer *operatorv1alpha1.KubeAPIServerConfig) bool {
	var oldConfigMap, newConfigMap string

	if oldKubeAPIServer != nil && oldKubeAPIServer.AuditConfig != nil && oldKubeAPIServer.AuditConfig.AuditPolicy != nil && oldKubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef != nil {
		oldConfigMap = oldKubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.Name
	}
	if newKubeAPIServer != nil && newKubeAPIServer.AuditConfig != nil && newKubeAPIServer.AuditConfig.AuditPolicy != nil && newKubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef != nil {
		newConfigMap = newKubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.Name
	}

	return oldConfigMap != newConfigMap
}

func gardenerAPIServerAuditPolicyConfigMapChanged(oldGardenerAPIServer, newGardenerAPIServer *operatorv1alpha1.GardenerAPIServerConfig) bool {
	var oldConfigMap, newConfigMap string

	if oldGardenerAPIServer != nil && oldGardenerAPIServer.AuditConfig != nil && oldGardenerAPIServer.AuditConfig.AuditPolicy != nil && oldGardenerAPIServer.AuditConfig.AuditPolicy.ConfigMapRef != nil {
		oldConfigMap = oldGardenerAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.Name
	}
	if newGardenerAPIServer != nil && newGardenerAPIServer.AuditConfig != nil && newGardenerAPIServer.AuditConfig.AuditPolicy != nil && newGardenerAPIServer.AuditConfig.AuditPolicy.ConfigMapRef != nil {
		newConfigMap = newGardenerAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.Name
	}

	return oldConfigMap != newConfigMap
}

func etcdBackupSecretChanged(oldETCD, newETCD *operatorv1alpha1.ETCD) bool {
	var oldSecret, newSecret string

	if oldETCD != nil && oldETCD.Main != nil && oldETCD.Main.Backup != nil {
		oldSecret = oldETCD.Main.Backup.SecretRef.Name
	}

	if newETCD != nil && newETCD.Main != nil && newETCD.Main.Backup != nil {
		newSecret = newETCD.Main.Backup.SecretRef.Name
	}

	return oldSecret != newSecret
}

func dnsSecretsChanged(oldDNS, newDNS *operatorv1alpha1.DNSManagement) bool {
	oldProviderSecretNames := map[string]string{}
	newProvidersSecretNames := map[string]string{}
	if oldDNS != nil {
		for _, provider := range oldDNS.Providers {
			oldProviderSecretNames[provider.Name] = provider.SecretRef.Name
		}
	}
	if newDNS != nil {
		for _, provider := range newDNS.Providers {
			newProvidersSecretNames[provider.Name] = provider.SecretRef.Name
		}
	}
	return !reflect.DeepEqual(oldProviderSecretNames, newProvidersSecretNames)
}

func authenticationWebhookSecretChanged(oldKubeAPIServer, newKubeAPIServer *operatorv1alpha1.KubeAPIServerConfig) bool {
	var oldSecret, newSecret string

	if oldKubeAPIServer != nil && oldKubeAPIServer.Authentication != nil && oldKubeAPIServer.Authentication.Webhook != nil {
		oldSecret = oldKubeAPIServer.Authentication.Webhook.KubeconfigSecretName
	}

	if newKubeAPIServer != nil && newKubeAPIServer.Authentication != nil && newKubeAPIServer.Authentication.Webhook != nil {
		newSecret = newKubeAPIServer.Authentication.Webhook.KubeconfigSecretName
	}

	return oldSecret != newSecret
}

func sniSecretChanged(oldKubeAPIServer, newKubeAPIServer *operatorv1alpha1.KubeAPIServerConfig) bool {
	var oldSecret, newSecret *string

	if oldKubeAPIServer != nil && oldKubeAPIServer.SNI != nil {
		oldSecret = oldKubeAPIServer.SNI.SecretName
	}

	if newKubeAPIServer != nil && newKubeAPIServer.SNI != nil {
		newSecret = newKubeAPIServer.SNI.SecretName
	}

	return !apiequality.Semantic.DeepEqual(oldSecret, newSecret)
}

func kubeAPIServerAuditWebhookSecretChanged(oldKubeAPIServer, newKubeAPIServer *operatorv1alpha1.KubeAPIServerConfig) bool {
	var oldSecret, newSecret string

	if oldKubeAPIServer != nil && oldKubeAPIServer.AuditWebhook != nil {
		oldSecret = oldKubeAPIServer.AuditWebhook.KubeconfigSecretName
	}
	if newKubeAPIServer != nil && newKubeAPIServer.AuditWebhook != nil {
		newSecret = newKubeAPIServer.AuditWebhook.KubeconfigSecretName
	}

	return oldSecret != newSecret
}

func gardenerAPIServerAuditWebhookSecretChanged(oldGardenerAPIServer, newGardenerAPIServer *operatorv1alpha1.GardenerAPIServerConfig) bool {
	var oldSecret, newSecret string

	if oldGardenerAPIServer != nil && oldGardenerAPIServer.AuditWebhook != nil {
		oldSecret = oldGardenerAPIServer.AuditWebhook.KubeconfigSecretName
	}
	if newGardenerAPIServer != nil && newGardenerAPIServer.AuditWebhook != nil {
		newSecret = newGardenerAPIServer.AuditWebhook.KubeconfigSecretName
	}

	return oldSecret != newSecret
}

func kubeAPIServerStructuredAuthenticationConfigMapChanged(oldKubeAPIServer, newKubeAPIServer *operatorv1alpha1.KubeAPIServerConfig) bool {
	var oldConfigMap, newConfigMap string

	if oldKubeAPIServer != nil {
		oldConfigMap = v1beta1helper.GetShootAuthenticationConfigurationConfigMapName(oldKubeAPIServer.KubeAPIServerConfig)
	}
	if newKubeAPIServer != nil {
		newConfigMap = v1beta1helper.GetShootAuthenticationConfigurationConfigMapName(newKubeAPIServer.KubeAPIServerConfig)
	}

	return oldConfigMap != newConfigMap
}

func kubeAPIServerStructuredAuthorizationConfigMapChanged(oldKubeAPIServer, newKubeAPIServer *operatorv1alpha1.KubeAPIServerConfig) bool {
	var oldConfigMap, newConfigMap string

	if oldKubeAPIServer != nil {
		oldConfigMap = v1beta1helper.GetShootAuthorizationConfigurationConfigMapName(oldKubeAPIServer.KubeAPIServerConfig)
	}
	if newKubeAPIServer != nil {
		newConfigMap = v1beta1helper.GetShootAuthorizationConfigurationConfigMapName(newKubeAPIServer.KubeAPIServerConfig)
	}

	return oldConfigMap != newConfigMap
}

func kubeAPIServerStructuredAuthorizationSecretsChanged(oldKubeAPIServer, newKubeAPIServer *operatorv1alpha1.KubeAPIServerConfig) bool {
	oldSecrets, newSecrets := sets.Set[string]{}, sets.Set[string]{}

	if oldKubeAPIServer != nil && oldKubeAPIServer.StructuredAuthorization != nil {
		for _, plugin := range oldKubeAPIServer.StructuredAuthorization.Kubeconfigs {
			oldSecrets.Insert(plugin.SecretName)
		}
	}
	if newKubeAPIServer != nil && newKubeAPIServer.StructuredAuthorization != nil {
		for _, plugin := range newKubeAPIServer.StructuredAuthorization.Kubeconfigs {
			newSecrets.Insert(plugin.SecretName)
		}
	}

	return !oldSecrets.Equal(newSecrets)
}

func kubeAPIServerAdmissionPluginSecretsChanged(oldKubeAPIServer, newKubeAPIServer *operatorv1alpha1.KubeAPIServerConfig) bool {
	oldSecrets, newSecrets := sets.Set[string]{}, sets.Set[string]{}

	if oldKubeAPIServer != nil {
		for _, plugin := range oldKubeAPIServer.AdmissionPlugins {
			if plugin.KubeconfigSecretName != nil {
				oldSecrets.Insert(*plugin.KubeconfigSecretName)
			}
		}
	}
	if newKubeAPIServer != nil {
		for _, plugin := range newKubeAPIServer.AdmissionPlugins {
			if plugin.KubeconfigSecretName != nil {
				newSecrets.Insert(*plugin.KubeconfigSecretName)
			}
		}
	}

	return !oldSecrets.Equal(newSecrets)
}

func gardenerAPIServerAdmissionPluginSecretChanged(oldGardenerAPIServer, newGardenerAPIServer *operatorv1alpha1.GardenerAPIServerConfig) bool {
	oldSecrets, newSecrets := sets.Set[string]{}, sets.Set[string]{}

	if oldGardenerAPIServer != nil {
		for _, plugin := range oldGardenerAPIServer.AdmissionPlugins {
			if plugin.KubeconfigSecretName != nil {
				oldSecrets.Insert(*plugin.KubeconfigSecretName)
			}
		}
	}
	if newGardenerAPIServer != nil {
		for _, plugin := range newGardenerAPIServer.AdmissionPlugins {
			if plugin.KubeconfigSecretName != nil {
				newSecrets.Insert(*plugin.KubeconfigSecretName)
			}
		}
	}

	return !oldSecrets.Equal(newSecrets)
}

func getReferencedSecretNames(obj client.Object) []string {
	garden, ok := obj.(*operatorv1alpha1.Garden)
	if !ok {
		return nil
	}

	var (
		virtualCluster = garden.Spec.VirtualCluster
		out            []string
	)

	if virtualCluster.ETCD != nil && virtualCluster.ETCD.Main != nil && virtualCluster.ETCD.Main.Backup != nil {
		out = append(out, virtualCluster.ETCD.Main.Backup.SecretRef.Name)
	}

	if garden.Spec.DNS != nil {
		for _, provider := range garden.Spec.DNS.Providers {
			out = append(out, provider.SecretRef.Name)
		}
	}

	if virtualCluster.Kubernetes.KubeAPIServer != nil {
		for _, plugin := range virtualCluster.Kubernetes.KubeAPIServer.AdmissionPlugins {
			if plugin.KubeconfigSecretName != nil {
				out = append(out, *plugin.KubeconfigSecretName)
			}
		}

		if virtualCluster.Kubernetes.KubeAPIServer.StructuredAuthorization != nil {
			for _, kubeconfig := range virtualCluster.Kubernetes.KubeAPIServer.StructuredAuthorization.Kubeconfigs {
				out = append(out, kubeconfig.SecretName)
			}
		}

		if virtualCluster.Kubernetes.KubeAPIServer.Authentication != nil && virtualCluster.Kubernetes.KubeAPIServer.Authentication.Webhook != nil {
			out = append(out, virtualCluster.Kubernetes.KubeAPIServer.Authentication.Webhook.KubeconfigSecretName)
		}

		if virtualCluster.Kubernetes.KubeAPIServer.SNI != nil && virtualCluster.Kubernetes.KubeAPIServer.SNI.SecretName != nil {
			out = append(out, *virtualCluster.Kubernetes.KubeAPIServer.SNI.SecretName)
		}

		if virtualCluster.Kubernetes.KubeAPIServer.AuditWebhook != nil {
			out = append(out, virtualCluster.Kubernetes.KubeAPIServer.AuditWebhook.KubeconfigSecretName)
		}
	}

	if virtualCluster.Gardener.APIServer != nil {
		for _, plugin := range virtualCluster.Gardener.APIServer.AdmissionPlugins {
			if plugin.KubeconfigSecretName != nil {
				out = append(out, *plugin.KubeconfigSecretName)
			}
		}

		if virtualCluster.Gardener.APIServer.AuditWebhook != nil {
			out = append(out, virtualCluster.Gardener.APIServer.AuditWebhook.KubeconfigSecretName)
		}
	}

	return out
}

func getReferencedConfigMapNames(obj client.Object) []string {
	garden, ok := obj.(*operatorv1alpha1.Garden)
	if !ok {
		return nil
	}

	var (
		virtualCluster = garden.Spec.VirtualCluster
		out            []string
	)

	if virtualCluster.Kubernetes.KubeAPIServer != nil {
		if virtualCluster.Kubernetes.KubeAPIServer.AuditConfig != nil && virtualCluster.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy != nil && virtualCluster.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef != nil {
			out = append(out, virtualCluster.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.Name)
		}
		if configMapName := v1beta1helper.GetShootAuthenticationConfigurationConfigMapName(virtualCluster.Kubernetes.KubeAPIServer.KubeAPIServerConfig); configMapName != "" {
			out = append(out, configMapName)
		}
		if configMapName := v1beta1helper.GetShootAuthorizationConfigurationConfigMapName(virtualCluster.Kubernetes.KubeAPIServer.KubeAPIServerConfig); configMapName != "" {
			out = append(out, configMapName)
		}
	}

	if virtualCluster.Gardener.APIServer != nil && virtualCluster.Gardener.APIServer.AuditConfig != nil && virtualCluster.Gardener.APIServer.AuditConfig.AuditPolicy != nil && virtualCluster.Gardener.APIServer.AuditConfig.AuditPolicy.ConfigMapRef != nil {
		out = append(out, virtualCluster.Gardener.APIServer.AuditConfig.AuditPolicy.ConfigMapRef.Name)
	}

	return out
}

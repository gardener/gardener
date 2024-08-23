// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/component/apiserver"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/gardener/secretsrotation"
)

func computeAPIServerAuditConfig(
	ctx context.Context,
	cl client.Client,
	objectMeta metav1.ObjectMeta,
	config *gardencorev1beta1.AuditConfig,
	webhookConfig *apiserver.AuditWebhook,
) (
	*apiserver.AuditConfig,
	error,
) {
	if config == nil || config.AuditPolicy == nil || config.AuditPolicy.ConfigMapRef == nil {
		return nil, nil
	}

	var (
		out = &apiserver.AuditConfig{
			Webhook: webhookConfig,
		}
		key = client.ObjectKey{Namespace: objectMeta.Namespace, Name: config.AuditPolicy.ConfigMapRef.Name}
	)

	configMap := &corev1.ConfigMap{}
	if err := cl.Get(ctx, key, configMap); err != nil {
		// Ignore missing audit configuration on cluster deletion to prevent failing redeployments of the
		// API server in case the end-user deleted the configmap before/simultaneously to the deletion.
		if !apierrors.IsNotFound(err) || objectMeta.DeletionTimestamp == nil {
			return nil, fmt.Errorf("retrieving audit policy from the ConfigMap %s failed: %w", key, err)
		}
	} else {
		policy, ok := configMap.Data["policy"]
		if !ok {
			return nil, fmt.Errorf("missing '.data.policy' in audit policy ConfigMap %s", key)
		}
		out.Policy = &policy
	}

	return out, nil
}

func computeAPIServerAuthenticationConfig(
	ctx context.Context,
	cl client.Client,
	objectMeta metav1.ObjectMeta,
	structuredAuthentication *gardencorev1beta1.StructuredAuthentication,
) (
	*string,
	error,
) {
	if structuredAuthentication == nil || len(structuredAuthentication.ConfigMapName) == 0 {
		return nil, nil
	}

	var (
		out *string
		key = client.ObjectKey{Namespace: objectMeta.Namespace, Name: structuredAuthentication.ConfigMapName}
	)

	configMap := &corev1.ConfigMap{}
	if err := cl.Get(ctx, key, configMap); err != nil {
		// Ignore missing authentication configuration on cluster deletion to prevent failing redeployments of the
		// API server in case the end-user deleted the configmap before/simultaneously to the deletion.
		if !apierrors.IsNotFound(err) || objectMeta.DeletionTimestamp == nil {
			return nil, fmt.Errorf("retrieving authentication configuration from the ConfigMap %s failed: %w", key, err)
		}
	} else {
		config, ok := configMap.Data["config.yaml"]
		if !ok {
			return nil, fmt.Errorf("missing '.data[config.yaml]' in authentication configuration ConfigMap %s", key)
		}
		out = ptr.To(config)
	}

	return out, nil
}

func computeEnabledAPIServerAdmissionPlugins(defaultPlugins, configuredPlugins []gardencorev1beta1.AdmissionPlugin) []gardencorev1beta1.AdmissionPlugin {
	for _, plugin := range configuredPlugins {
		pluginOverwritesDefault := false

		for i, defaultPlugin := range defaultPlugins {
			if defaultPlugin.Name == plugin.Name {
				pluginOverwritesDefault = true
				defaultPlugins[i] = plugin

				break
			}
		}

		if !pluginOverwritesDefault {
			defaultPlugins = append(defaultPlugins, plugin)
		}
	}

	var admissionPlugins []gardencorev1beta1.AdmissionPlugin
	for _, defaultPlugin := range defaultPlugins {
		if !ptr.Deref(defaultPlugin.Disabled, false) {
			admissionPlugins = append(admissionPlugins, defaultPlugin)
		}
	}
	return admissionPlugins
}

func computeDisabledAPIServerAdmissionPlugins(configuredPlugins []gardencorev1beta1.AdmissionPlugin) []gardencorev1beta1.AdmissionPlugin {
	var disabledAdmissionPlugins []gardencorev1beta1.AdmissionPlugin

	for _, plugin := range configuredPlugins {
		if ptr.Deref(plugin.Disabled, false) {
			disabledAdmissionPlugins = append(disabledAdmissionPlugins, plugin)
		}
	}

	return disabledAdmissionPlugins
}

func convertToAdmissionPluginConfigs(ctx context.Context, gardenClient client.Client, namespace string, plugins []gardencorev1beta1.AdmissionPlugin) ([]apiserver.AdmissionPluginConfig, error) {
	var (
		err error
		out []apiserver.AdmissionPluginConfig
	)

	for _, plugin := range plugins {
		config := apiserver.AdmissionPluginConfig{AdmissionPlugin: plugin}
		if plugin.KubeconfigSecretName != nil {
			key := client.ObjectKey{Namespace: namespace, Name: *plugin.KubeconfigSecretName}
			config.Kubeconfig, err = gardenerutils.FetchKubeconfigFromSecret(ctx, gardenClient, key)
			if err != nil {
				return nil, fmt.Errorf("failed reading kubeconfig for admission plugin from referenced secret %s: %w", key, err)
			}
		}
		out = append(out, config)
	}

	return out, nil
}

func computeAPIServerETCDEncryptionConfig(
	ctx context.Context,
	runtimeClient client.Client,
	runtimeNamespace string,
	deploymentName string,
	etcdEncryptionKeyRotationPhase gardencorev1beta1.CredentialsRotationPhase,
	resourcesToEncrypt []string,
	encryptedResources []string,
) (
	apiserver.ETCDEncryptionConfig,
	error,
) {
	config := apiserver.ETCDEncryptionConfig{
		RotationPhase:         etcdEncryptionKeyRotationPhase,
		EncryptWithCurrentKey: true,
		ResourcesToEncrypt:    resourcesToEncrypt,
		EncryptedResources:    encryptedResources,
	}

	if etcdEncryptionKeyRotationPhase == gardencorev1beta1.RotationPreparing {
		deployment := &metav1.PartialObjectMetadata{}
		deployment.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))
		if err := runtimeClient.Get(ctx, client.ObjectKey{Namespace: runtimeNamespace, Name: deploymentName}, deployment); err != nil {
			if !apierrors.IsNotFound(err) {
				return apiserver.ETCDEncryptionConfig{}, err
			}
		}

		// If the new encryption key was not yet populated to all replicas then we should still use the old key for
		// encryption of data. Only if all replicas know the new key we can switch and start encrypting with the new/
		// current key, see https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/#rotating-a-decryption-key.
		if !metav1.HasAnnotation(deployment.ObjectMeta, secretsrotation.AnnotationKeyNewEncryptionKeyPopulated) {
			config.EncryptWithCurrentKey = false
		}
	}

	return config, nil
}

func handleETCDEncryptionKeyRotation(
	ctx context.Context,
	runtimeClient client.Client,
	runtimeNamespace string,
	deploymentName string,
	apiServer apiserver.Interface,
	etcdEncryptionConfig apiserver.ETCDEncryptionConfig,
	etcdEncryptionKeyRotationPhase gardencorev1beta1.CredentialsRotationPhase,
) error {
	switch etcdEncryptionKeyRotationPhase {
	case gardencorev1beta1.RotationPreparing:
		if !etcdEncryptionConfig.EncryptWithCurrentKey {
			if err := apiServer.Wait(ctx); err != nil {
				return err
			}

			// If we have hit this point then we have deployed API server successfully with the configuration option to
			// still use the old key for the encryption of ETCD data. Now we can mark this step as "completed" (via an
			// annotation) and redeploy it with the option to use the current/new key for encryption, see
			// https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/#rotating-a-decryption-key for details.
			if err := secretsrotation.PatchAPIServerDeploymentMeta(ctx, runtimeClient, runtimeNamespace, deploymentName, func(meta *metav1.PartialObjectMetadata) {
				metav1.SetMetaDataAnnotation(&meta.ObjectMeta, secretsrotation.AnnotationKeyNewEncryptionKeyPopulated, "true")
			}); err != nil {
				return err
			}

			etcdEncryptionConfig.EncryptWithCurrentKey = true
			apiServer.SetETCDEncryptionConfig(etcdEncryptionConfig)

			if err := apiServer.Deploy(ctx); err != nil {
				return err
			}
		}

	case gardencorev1beta1.RotationCompleting:
		if err := secretsrotation.PatchAPIServerDeploymentMeta(ctx, runtimeClient, runtimeNamespace, deploymentName, func(meta *metav1.PartialObjectMetadata) {
			delete(meta.Annotations, secretsrotation.AnnotationKeyNewEncryptionKeyPopulated)
		}); err != nil {
			return err
		}
	}

	return nil
}

// GetResourcesForEncryptionFromConfig returns the list of resources requiring encryption from the EncryptionConfig.
func GetResourcesForEncryptionFromConfig(encryptionConfig *gardencorev1beta1.EncryptionConfig) []string {
	if encryptionConfig == nil {
		return nil
	}

	return sets.List(sets.New(encryptionConfig.Resources...))
}

// NormalizeResources returns the list of resources after trimming the suffix '.' if present.
// This is needed for core resources which can be specified as '<resource>.' as well.
func NormalizeResources(resources []string) []string {
	var out []string

	for _, resource := range resources {
		out = append(out, strings.TrimSuffix(resource, "."))
	}

	return out
}

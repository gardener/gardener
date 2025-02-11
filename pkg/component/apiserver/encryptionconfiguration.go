// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"context"
	"fmt"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/apiserver/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

var encryptionCodec runtime.Codec

func init() {
	encryptionScheme := runtime.NewScheme()
	utilruntime.Must(apiserverconfigv1.AddToScheme(encryptionScheme))

	var (
		ser = json.NewSerializerWithOptions(json.DefaultMetaFactory, encryptionScheme, encryptionScheme, json.SerializerOptions{
			Yaml:   true,
			Pretty: false,
			Strict: false,
		})
		versions = schema.GroupVersions([]schema.GroupVersion{
			apiserverconfigv1.SchemeGroupVersion,
		})
	)

	encryptionCodec = serializer.NewCodecFactory(encryptionScheme).CodecForVersions(ser, ser, versions, versions)
}

const (
	secretETCDEncryptionConfigurationDataKey = "encryption-configuration.yaml"

	volumeNameEtcdEncryptionConfig      = "etcd-encryption-secret"
	volumeMountPathEtcdEncryptionConfig = "/etc/kubernetes/etcd-encryption-secret"
)

// ReconcileSecretETCDEncryptionConfiguration reconciles the ETCD encryption secret configuration.
func ReconcileSecretETCDEncryptionConfiguration(
	ctx context.Context,
	c client.Client,
	secretsManager secretsmanager.Interface,
	config ETCDEncryptionConfig,
	secretETCDEncryptionConfiguration *corev1.Secret,
	secretNameETCDEncryptionKey string,
	roleLabel string,
) error {
	options := []secretsmanager.GenerateOption{
		secretsmanager.Persist(),
		secretsmanager.Rotate(secretsmanager.KeepOld),
	}

	if config.RotationPhase == gardencorev1beta1.RotationCompleting {
		options = append(options, secretsmanager.IgnoreOldSecrets())
	}

	keySecret, err := secretsManager.Generate(ctx, &secretsutils.ETCDEncryptionKeySecretConfig{
		Name:         secretNameETCDEncryptionKey,
		SecretLength: 32,
	}, options...)
	if err != nil {
		return err
	}

	var (
		keySecretOld, _         = secretsManager.Get(secretNameETCDEncryptionKey, secretsmanager.Old)
		encryptionKeys          = etcdEncryptionAESKeys(keySecret, keySecretOld, config.EncryptWithCurrentKey)
		encryptionConfiguration = &apiserverconfigv1.EncryptionConfiguration{
			Resources: []apiserverconfigv1.ResourceConfiguration{
				{
					Resources: config.ResourcesToEncrypt,
					Providers: []apiserverconfigv1.ProviderConfiguration{
						{
							AESCBC: &apiserverconfigv1.AESConfiguration{
								Keys: encryptionKeys,
							},
						},
						{
							Identity: &apiserverconfigv1.IdentityConfiguration{},
						},
					},
				},
			},
		}
	)

	if !reflect.DeepEqual(config.ResourcesToEncrypt, config.EncryptedResources) {
		removedResources := sets.New(config.EncryptedResources...).Difference(sets.New(config.ResourcesToEncrypt...))
		if removedResources.Len() > 0 {
			encryptionConfiguration.Resources = append(encryptionConfiguration.Resources, apiserverconfigv1.ResourceConfiguration{
				Resources: sets.List(removedResources),
				Providers: []apiserverconfigv1.ProviderConfiguration{
					{
						Identity: &apiserverconfigv1.IdentityConfiguration{},
					},
					{
						AESCBC: &apiserverconfigv1.AESConfiguration{
							Keys: encryptionKeys,
						},
					},
				},
			})
		}
	}

	data, err := runtime.Encode(encryptionCodec, encryptionConfiguration)
	if err != nil {
		return err
	}

	secretETCDEncryptionConfiguration.Labels = map[string]string{v1beta1constants.LabelRole: roleLabel}
	secretETCDEncryptionConfiguration.Data = map[string][]byte{secretETCDEncryptionConfigurationDataKey: data}
	utilruntime.Must(kubernetesutils.MakeUnique(secretETCDEncryptionConfiguration))
	desiredLabels := utils.MergeStringMaps(secretETCDEncryptionConfiguration.Labels) // copy

	if err := c.Create(ctx, secretETCDEncryptionConfiguration); err == nil || !apierrors.IsAlreadyExists(err) {
		return err
	}

	// creation of secret failed as it already exists => reconcile labels of existing secret
	if err := c.Get(ctx, client.ObjectKeyFromObject(secretETCDEncryptionConfiguration), secretETCDEncryptionConfiguration); err != nil {
		return err
	}
	patch := client.MergeFrom(secretETCDEncryptionConfiguration.DeepCopy())
	secretETCDEncryptionConfiguration.Labels = desiredLabels
	return c.Patch(ctx, secretETCDEncryptionConfiguration, patch)
}

func etcdEncryptionAESKeys(keySecretCurrent, keySecretOld *corev1.Secret, encryptWithCurrentKey bool) []apiserverconfigv1.Key {
	if keySecretOld == nil {
		return []apiserverconfigv1.Key{
			aesKeyFromSecretData(keySecretCurrent.Data),
		}
	}

	keyForEncryption, keyForDecryption := keySecretCurrent, keySecretOld
	if !encryptWithCurrentKey {
		keyForEncryption, keyForDecryption = keySecretOld, keySecretCurrent
	}

	return []apiserverconfigv1.Key{
		aesKeyFromSecretData(keyForEncryption.Data),
		aesKeyFromSecretData(keyForDecryption.Data),
	}
}

func aesKeyFromSecretData(data map[string][]byte) apiserverconfigv1.Key {
	var key string
	if v, ok := data[secretsutils.DataKeyEncryptionSecretEncoding]; ok && string(v) == "none" {
		// key is not encoded, so we need to encode it before passing it to the kube-apiserver
		key = utils.EncodeBase64(data[secretsutils.DataKeyEncryptionSecret])
	} else {
		key = string(data[secretsutils.DataKeyEncryptionSecret])
	}
	return apiserverconfigv1.Key{
		Name:   string(data[secretsutils.DataKeyEncryptionKeyName]),
		Secret: key,
	}
}

// InjectEncryptionSettings injects the encryption settings into `gardener-apiserver` and `kube-apiserver` deployments.
func InjectEncryptionSettings(deployment *appsv1.Deployment, secretETCDEncryptionConfiguration *corev1.Secret) {
	deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--encryption-provider-config=%s/%s", volumeMountPathEtcdEncryptionConfig, secretETCDEncryptionConfigurationDataKey))
	deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
		Name:      volumeNameEtcdEncryptionConfig,
		MountPath: volumeMountPathEtcdEncryptionConfig,
		ReadOnly:  true,
	})
	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: volumeNameEtcdEncryptionConfig,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  secretETCDEncryptionConfiguration.Name,
				DefaultMode: ptr.To[int32](0640),
			},
		},
	})
}

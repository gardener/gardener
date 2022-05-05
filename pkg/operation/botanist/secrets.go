// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
)

// InitializeSecretsManagement initializes the secrets management and deploys the required secrets to the shoot
// namespace in the seed.
func (b *Botanist) InitializeSecretsManagement(ctx context.Context) error {
	// Generally, the existing secrets in the shoot namespace in the seeds are the source of truth for the secret
	// manager. Hence, if we restore a shoot control plane then let's fetch the existing data from the ShootState and
	// create corresponding secrets in the shoot namespace in the seed before initializing it. Note that this is
	// explicitly only done in case of restoration to prevent split-brain situations as described in
	// https://github.com/gardener/gardener/issues/5377.
	if b.isRestorePhase() {
		if err := b.restoreSecretsFromShootStateForSecretsManagerAdoption(ctx); err != nil {
			return err
		}
	}

	return flow.Sequential(
		b.generateCertificateAuthorities,
		b.generateSSHKeypair,
		b.generateGenericTokenKubeconfig,
		b.reconcileWildcardIngressCertificate,
		// TODO(rfranzke): Remove this function in a future release.
		b.reconcileGenericKubeconfigSecret,
	)(ctx)
}

func (b *Botanist) lastSecretRotationStartTimes() map[string]time.Time {
	rotation := make(map[string]time.Time)

	if shootStatus := b.Shoot.GetInfo().Status; shootStatus.Credentials != nil && shootStatus.Credentials.Rotation != nil {
		if shootStatus.Credentials.Rotation.CertificateAuthorities != nil && shootStatus.Credentials.Rotation.CertificateAuthorities.LastInitiationTime != nil {
			for _, config := range caCertConfigurations() {
				rotation[config.GetName()] = shootStatus.Credentials.Rotation.CertificateAuthorities.LastInitiationTime.Time
			}
		}

		if shootStatus.Credentials.Rotation.Kubeconfig != nil && shootStatus.Credentials.Rotation.Kubeconfig.LastInitiationTime != nil {
			rotation[kubeapiserver.SecretStaticTokenName] = shootStatus.Credentials.Rotation.Kubeconfig.LastInitiationTime.Time
			rotation[kubeapiserver.SecretBasicAuthName] = shootStatus.Credentials.Rotation.Kubeconfig.LastInitiationTime.Time
		}

		if shootStatus.Credentials.Rotation.SSHKeypair != nil && shootStatus.Credentials.Rotation.SSHKeypair.LastInitiationTime != nil {
			rotation[v1beta1constants.SecretNameSSHKeyPair] = shootStatus.Credentials.Rotation.SSHKeypair.LastInitiationTime.Time
		}
	}

	return rotation
}

func (b *Botanist) restoreSecretsFromShootStateForSecretsManagerAdoption(ctx context.Context) error {
	var fns []flow.TaskFn

	for _, v := range b.GetShootState().Spec.Gardener {
		entry := v

		if entry.Labels[secretsmanager.LabelKeyManagedBy] != secretsmanager.LabelValueSecretsManager ||
			entry.Type != "secret" {
			continue
		}

		fns = append(fns, func(ctx context.Context) error {
			objectMeta := metav1.ObjectMeta{
				Name:      entry.Name,
				Namespace: b.Shoot.SeedNamespace,
				Labels:    entry.Labels,
			}

			data := make(map[string][]byte)
			if err := json.Unmarshal(entry.Data.Raw, &data); err != nil {
				return err
			}

			secret := secretsmanager.Secret(objectMeta, data)
			return kutil.IgnoreAlreadyExists(b.K8sSeedClient.Client().Create(ctx, secret))
		})
	}

	return flow.Parallel(fns...)(ctx)
}

func caCertConfigurations() []secretutils.ConfigInterface {
	return []secretutils.ConfigInterface{
		// The CommonNames for CA certificates will be overridden with the secret name by the secrets manager when
		// generated to ensure that each CA has a unique common name. For backwards-compatibility, we still keep the
		// CommonNames here (if we removed them then new CAs would be generated with the next shoot reconciliation
		// without the end-user to explicitly trigger it).
		&secretutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCACluster, CommonName: "kubernetes", CertType: secretutils.CACert},
		&secretutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCAClient, CommonName: "kubernetes-client", CertType: secretutils.CACert},
		&secretutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCAETCD, CommonName: "etcd", CertType: secretutils.CACert},
		&secretutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCAFrontProxy, CommonName: "front-proxy", CertType: secretutils.CACert},
		&secretutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCAKubelet, CommonName: "kubelet", CertType: secretutils.CACert},
		&secretutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCAMetricsServer, CommonName: "metrics-server", CertType: secretutils.CACert},
		&secretutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCAVPN, CommonName: "vpn", CertType: secretutils.CACert},
	}
}

func (b *Botanist) caCertGenerateOptionsFor(configName string) []secretsmanager.GenerateOption {
	options := []secretsmanager.GenerateOption{
		secretsmanager.Persist(),
		secretsmanager.Rotate(secretsmanager.KeepOld),
	}

	if gardencorev1beta1helper.GetShootCARotationPhase(b.Shoot.GetInfo().Status.Credentials) == gardencorev1beta1.RotationCompleting {
		options = append(options, secretsmanager.IgnoreOldSecrets())
	}

	if configName == v1beta1constants.SecretNameCAClient {
		return options
	}

	// For all CAs other than the client CA we ignore the checksum for the CA secret name due to backwards compatibility
	// reasons in case the CA certificate were never rotated yet. With the first rotation we consider the config
	// checksums since we can now assume that all components are able to cater with it (since we only allow triggering
	// CA rotations after we have adapted all components that might rely on the constant CA secret names).
	// The client CA was only introduced late with https://github.com/gardener/gardener/pull/5779, hence nobody was
	// using it and the config checksum could be considered right away.
	if shootStatus := b.Shoot.GetInfo().Status; shootStatus.Credentials == nil ||
		shootStatus.Credentials.Rotation == nil ||
		shootStatus.Credentials.Rotation.CertificateAuthorities == nil {
		options = append(options, secretsmanager.IgnoreConfigChecksumForCASecretName())
	}

	return options
}

func (b *Botanist) generateCertificateAuthorities(ctx context.Context) error {
	for _, config := range caCertConfigurations() {
		if _, err := b.SecretsManager.Generate(ctx, config, b.caCertGenerateOptionsFor(config.GetName())...); err != nil {
			return err
		}
	}

	caBundleSecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
	}

	return b.syncShootCredentialToGarden(
		ctx,
		gutil.ShootProjectSecretSuffixCACluster,
		map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleCACluster},
		nil,
		map[string][]byte{secretutils.DataKeyCertificateCA: caBundleSecret.Data[secretutils.DataKeyCertificateBundle]},
	)
}

func (b *Botanist) generateGenericTokenKubeconfig(ctx context.Context) error {
	clusterCABundleSecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
	}

	config := &secretutils.KubeconfigSecretConfig{
		Name:        v1beta1constants.SecretNameGenericTokenKubeconfig,
		ContextName: b.Shoot.SeedNamespace,
		Cluster: clientcmdv1.Cluster{
			Server:                   b.Shoot.ComputeInClusterAPIServerAddress(true),
			CertificateAuthorityData: clusterCABundleSecret.Data[secretutils.DataKeyCertificateBundle],
		},
		AuthInfo: clientcmdv1.AuthInfo{
			TokenFile: gutil.PathShootToken,
		},
	}

	genericTokenKubeconfigSecret, err := b.SecretsManager.Generate(ctx, config, secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return err
	}

	cluster := &extensionsv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: b.Shoot.SeedNamespace}}
	_, err = controllerutils.GetAndCreateOrMergePatch(ctx, b.K8sSeedClient.Client(), cluster, func() error {
		metav1.SetMetaDataAnnotation(&cluster.ObjectMeta, v1beta1constants.AnnotationKeyGenericTokenKubeconfigSecretName, genericTokenKubeconfigSecret.Name)
		return nil
	})
	return err
}

func (b *Botanist) generateSSHKeypair(ctx context.Context) error {
	sshKeypairSecret, err := b.SecretsManager.Generate(ctx, &secretutils.RSASecretConfig{
		Name:       v1beta1constants.SecretNameSSHKeyPair,
		Bits:       4096,
		UsedForSSH: true,
	}, secretsmanager.Persist(), secretsmanager.Rotate(secretsmanager.KeepOld))
	if err != nil {
		return err
	}

	if err := b.syncShootCredentialToGarden(
		ctx,
		gutil.ShootProjectSecretSuffixSSHKeypair,
		map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleSSHKeyPair},
		nil,
		sshKeypairSecret.Data,
	); err != nil {
		return err
	}

	if sshKeypairSecretOld, found := b.SecretsManager.Get(v1beta1constants.SecretNameSSHKeyPair, secretsmanager.Old); found {
		if err := b.syncShootCredentialToGarden(
			ctx,
			gutil.ShootProjectSecretSuffixOldSSHKeypair,
			map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleSSHKeyPair},
			nil,
			sshKeypairSecretOld.Data,
		); err != nil {
			return err
		}
	}

	// TODO(rfranzke): Remove in a future release.
	if err := b.SaveGardenerResourceDataInShootState(ctx, func(gardenerResourceData *[]gardencorev1alpha1.GardenerResourceData) error {
		gardenerResourceDataList := gardencorev1alpha1helper.GardenerResourceDataList(*gardenerResourceData)
		gardenerResourceDataList.Delete("ssh-keypair")
		*gardenerResourceData = gardenerResourceDataList
		return nil
	}); err != nil {
		return err
	}

	// TODO(rfranzke): Remove this in a future release.
	return kutil.DeleteObject(ctx, b.K8sSeedClient.Client(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ssh-keypair", Namespace: b.Shoot.SeedNamespace}})
}

func (b *Botanist) syncShootCredentialToGarden(
	ctx context.Context,
	nameSuffix string,
	labels map[string]string,
	annotations map[string]string,
	data map[string][]byte,
) error {
	gardenSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gutil.ComputeShootProjectSecretName(b.Shoot.GetInfo().Name, nameSuffix),
			Namespace: b.Shoot.GetInfo().Namespace,
		},
	}

	_, err := controllerutils.GetAndCreateOrStrategicMergePatch(ctx, b.K8sGardenClient.Client(), gardenSecret, func() error {
		gardenSecret.OwnerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(b.Shoot.GetInfo(), gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot")),
		}
		gardenSecret.Annotations = annotations
		gardenSecret.Labels = labels
		gardenSecret.Type = corev1.SecretTypeOpaque
		gardenSecret.Data = data
		return nil
	})
	return err
}

func (b *Botanist) reconcileWildcardIngressCertificate(ctx context.Context) error {
	wildcardCert, err := seed.GetWildcardCertificate(ctx, b.K8sSeedClient.Client())
	if err != nil {
		return err
	}
	if wildcardCert == nil {
		return nil
	}

	// Copy certificate to shoot namespace
	certSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      wildcardCert.GetName(),
			Namespace: b.Shoot.SeedNamespace,
		},
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, b.K8sSeedClient.Client(), certSecret, func() error {
		certSecret.Data = wildcardCert.Data
		return nil
	}); err != nil {
		return err
	}

	b.ControlPlaneWildcardCert = certSecret
	return nil
}

// TODO(rfranzke): Remove this function in a future release.
func (b *Botanist) reconcileGenericKubeconfigSecret(ctx context.Context) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1beta1constants.SecretNameGenericTokenKubeconfig,
			Namespace: b.Shoot.SeedNamespace,
		},
	}

	clusterCASecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
	}

	kubeconfig, err := runtime.Encode(clientcmdlatest.Codec, kutil.NewKubeconfig(
		b.Shoot.SeedNamespace,
		clientcmdv1.Cluster{
			Server:                   b.Shoot.ComputeInClusterAPIServerAddress(true),
			CertificateAuthorityData: clusterCASecret.Data[secretutils.DataKeyCertificateBundle],
		},
		clientcmdv1.AuthInfo{TokenFile: gutil.PathShootToken},
	))
	if err != nil {
		return err
	}

	_, err = controllerutils.CreateOrGetAndMergePatch(ctx, b.K8sSeedClient.Client(), secret, func() error {
		secret.Type = corev1.SecretTypeOpaque
		secret.Data = map[string][]byte{secretutils.DataKeyKubeconfig: kubeconfig}
		return nil
	})
	return err
}

// DeployCloudProviderSecret creates or updates the cloud provider secret in the Shoot namespace
// in the Seed cluster.
func (b *Botanist) DeployCloudProviderSecret(ctx context.Context) error {
	var (
		checksum = utils.ComputeSecretChecksum(b.Shoot.Secret.Data)
		secret   = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: b.Shoot.SeedNamespace,
			},
		}
	)

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, b.K8sSeedClient.Client(), secret, func() error {
		secret.Annotations = map[string]string{
			"checksum/data": checksum,
		}
		secret.Labels = map[string]string{
			v1beta1constants.GardenerPurpose: v1beta1constants.SecretNameCloudProvider,
		}
		secret.Type = corev1.SecretTypeOpaque
		secret.Data = b.Shoot.Secret.Data
		return nil
	})
	return err
}

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
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnshoot"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/operation/shootsecrets"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/infodata"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/clock"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (b *Botanist) wantedCertificateAuthorities() map[string]*secretutils.CertificateSecretConfig {
	return map[string]*secretutils.CertificateSecretConfig{
		v1beta1constants.SecretNameCACluster: {
			Name:       v1beta1constants.SecretNameCACluster,
			CommonName: "kubernetes",
			CertType:   secretutils.CACert,
		},
		v1beta1constants.SecretNameCAETCD: {
			Name:       v1beta1constants.SecretNameCAETCD,
			CommonName: "etcd",
			CertType:   secretutils.CACert,
		},
		v1beta1constants.SecretNameCAFrontProxy: {
			Name:       v1beta1constants.SecretNameCAFrontProxy,
			CommonName: "front-proxy",
			CertType:   secretutils.CACert,
		},
		v1beta1constants.SecretNameCAKubelet: {
			Name:       v1beta1constants.SecretNameCAKubelet,
			CommonName: "kubelet",
			CertType:   secretutils.CACert,
		},
		v1beta1constants.SecretNameCAMetricsServer: {
			Name:       v1beta1constants.SecretNameCAMetricsServer,
			CommonName: "metrics-server",
			CertType:   secretutils.CACert,
		},
		v1beta1constants.SecretNameCAVPN: {
			Name:       v1beta1constants.SecretNameCAVPN,
			CommonName: "vpn",
			CertType:   secretutils.CACert,
		},
	}
}

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

	for _, config := range b.wantedCertificateAuthorities() {
		if _, err := b.SecretsManager.Generate(ctx, config, secretsmanager.Persist(), secretsmanager.Rotate(secretsmanager.KeepOld)); err != nil {
			return err
		}
	}

	return b.getOrGenerateGenericTokenKubeconfig(ctx)
}

func (b *Botanist) lastSecretRotationStartTimes() map[string]time.Time {
	rotation := make(map[string]time.Time)

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

func (b *Botanist) getOrGenerateGenericTokenKubeconfig(ctx context.Context) error {
	clusterCABundleSecret, err := b.SecretsManager.Get(v1beta1constants.SecretNameCACluster)
	if err != nil {
		return err
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

func (b *Botanist) syncShootCredentialToGarden(
	ctx context.Context,
	seedSecretName string,
	nameSuffix string,
	annotations map[string]string,
	labels map[string]string,
) error {
	seedSecret, err := b.SecretsManager.Get(seedSecretName)
	if err != nil {
		return err
	}

	gardenSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gutil.ComputeShootProjectSecretName(b.Shoot.GetInfo().Name, nameSuffix),
			Namespace: b.Shoot.GetInfo().Namespace,
		},
	}

	_, err = controllerutils.GetAndCreateOrStrategicMergePatch(ctx, b.K8sGardenClient.Client(), gardenSecret, func() error {
		gardenSecret.OwnerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(b.Shoot.GetInfo(), gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot")),
		}
		gardenSecret.Annotations = annotations
		gardenSecret.Labels = labels
		gardenSecret.Type = corev1.SecretTypeOpaque
		gardenSecret.Data = seedSecret.Data
		return nil
	})
	return err
}

func (b *Botanist) fetchCertificateAuthoritiesForLegacySecretsManager(ctx context.Context, legacySecretsManager *shootsecrets.SecretsManager, addToDeployedSecrets bool) (map[string]*secretutils.Certificate, error) {
	cas := make(map[string]*secretutils.Certificate)

	for _, config := range b.wantedCertificateAuthorities() {
		secret, err := b.SecretsManager.Generate(ctx, config, secretsmanager.Persist(), secretsmanager.Rotate(secretsmanager.KeepOld))
		if err != nil {
			return nil, err
		}

		if addToDeployedSecrets {
			legacySecretsManager.DeployedSecrets[secret.Name] = secret
		}

		cert, err := secretutils.LoadCertificate(secret.Name, secret.Data[secretutils.DataKeyPrivateKeyCA], secret.Data[secretutils.DataKeyCertificateCA])
		if err != nil {
			return nil, err
		}

		cas[secret.Name] = cert
	}

	return cas, nil
}

// GenerateAndSaveSecrets creates a CA certificate for the Shoot cluster and uses it to sign the server certificate
// used by the kube-apiserver, and all client certificates used for communication. It also creates RSA key
// pairs for SSH connections to the nodes/VMs and for the VPN tunnel. Moreover, basic authentication
// credentials are computed which will be used to secure the Ingress resources and the kube-apiserver itself.
// Server certificates for the exposed monitoring endpoints (via Ingress) are generated as well.
func (b *Botanist) GenerateAndSaveSecrets(ctx context.Context) error {
	return b.SaveGardenerResourceDataInShootState(ctx, func(gardenerResourceData *[]gardencorev1alpha1.GardenerResourceData) error {
		gardenerResourceDataList := gardencorev1alpha1helper.GardenerResourceDataList(*gardenerResourceData)

		// Remove legacy secrets from ShootState.
		// TODO(rfranzke): Remove in a future version.
		for _, name := range []string{
			"gardener",
			"gardener-internal",
			"kube-controller-manager",
			"cluster-autoscaler",
			"kube-state-metrics",
			// TODO(rfranzke): Uncomment this in a future release once all monitoring configurations of extensions have been
			// adapted.
			// "prometheus",
			"kube-scheduler-server",
		} {
			gardenerResourceDataList.Delete(name)
		}

		switch b.Shoot.GetInfo().Annotations[v1beta1constants.GardenerOperation] {
		case v1beta1constants.ShootOperationRotateKubeconfigCredentials:
			if err := b.rotateKubeconfigSecrets(ctx, &gardenerResourceDataList); err != nil {
				return err
			}

		case v1beta1constants.ShootOperationRotateSSHKeypair:
			if err := b.rotateSSHKeypairSecrets(ctx, &gardenerResourceDataList); err != nil {
				return err
			}
		}

		if b.Shoot.GetInfo().DeletionTimestamp == nil {
			if b.Shoot.ReversedVPNEnabled {
				if err := b.cleanupSecrets(ctx, &gardenerResourceDataList,
					kubeapiserver.SecretNameVPNSeed,
					kubeapiserver.SecretNameVPNSeedTLSAuth,
					vpnshoot.SecretNameVPNShoot,
				); err != nil {
					return err
				}

				// Delete existing VPN-related secrets which were not signed with the newly introduced ca-vpn so that
				// they get regenerated.
				// TODO(rfranzke): Remove in a future version.
				if gardenerResourceDataList.Get(v1beta1constants.SecretNameCAVPN) == nil {
					if err := b.cleanupSecrets(ctx, &gardenerResourceDataList,
						vpnseedserver.DeploymentName,
						kubeapiserver.SecretNameHTTPProxy,
						vpnshoot.SecretNameVPNShootClient,
					); err != nil {
						return err
					}
				}
			} else {
				if err := b.cleanupSecrets(ctx, &gardenerResourceDataList,
					vpnseedserver.DeploymentName,
					vpnshoot.SecretNameVPNShootClient,
					vpnseedserver.VpnSeedServerTLSAuth,
					kubeapiserver.SecretNameHTTPProxy,
				); err != nil {
					return err
				}
			}

			if !gardencorev1beta1helper.SeedSettingDependencyWatchdogProbeEnabled(b.Seed.GetInfo().Spec.Settings) {
				if err := b.cleanupSecrets(ctx, &gardenerResourceDataList, kubeapiserver.DependencyWatchdogInternalProbeSecretName, kubeapiserver.DependencyWatchdogExternalProbeSecretName); err != nil {
					return err
				}
			}
		}

		// Trigger replacement of operator/user facing certificates if required
		expiredTLSSecrets, err := getExpiredCerts(gardenerResourceDataList, common.CrtRenewalWindow, common.IngressTLSSecretNames...)
		if err != nil {
			return err
		}

		if len(expiredTLSSecrets) > 0 {
			b.Logger.Infof("Deleting secrets for certificate rotation: %v", expiredTLSSecrets)
			if err := b.deleteSecrets(ctx, &gardenerResourceDataList, expiredTLSSecrets...); err != nil {
				return err
			}
		}

		secretsManager := shootsecrets.NewSecretsManager(
			gardenerResourceDataList,
			b.generateWantedSecretConfigs,
		)

		// For backwards-compatibility, we need to make the CAs known to the legacy secret manager.
		// TODO(rfranzke): This can be removed in a future release once all secrets where adapted.
		cas, err := b.fetchCertificateAuthoritiesForLegacySecretsManager(ctx, secretsManager, false)
		if err != nil {
			return err
		}
		secretsManager = secretsManager.WithCertificateAuthorities(cas)

		if err := secretsManager.Generate(); err != nil {
			return err
		}

		*gardenerResourceData = secretsManager.GardenerResourceDataList

		return nil
	})
}

// DeploySecrets takes all existing secrets from the ShootState resource and deploys them in the shoot's control plane.
func (b *Botanist) DeploySecrets(ctx context.Context) error {
	gardenerResourceDataList := gardencorev1alpha1helper.GardenerResourceDataList(b.GetShootState().Spec.Gardener)
	existingSecrets, err := b.fetchExistingSecrets(ctx)
	if err != nil {
		return err
	}

	secretsManager := shootsecrets.NewSecretsManager(
		gardenerResourceDataList,
		b.generateWantedSecretConfigs,
	)

	// For backwards-compatibility, we need to make the CAs known to the legacy secret manager.
	// TODO(rfranzke): This can be removed in a future release once all secrets where adapted.
	cas, err := b.fetchCertificateAuthoritiesForLegacySecretsManager(ctx, secretsManager, true)
	if err != nil {
		return err
	}
	secretsManager = secretsManager.WithCertificateAuthorities(cas)

	if err := secretsManager.WithExistingSecrets(existingSecrets).Deploy(ctx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace); err != nil {
		return err
	}

	for name, secret := range secretsManager.DeployedSecrets {
		b.StoreSecret(name, secret)
	}

	for _, name := range b.AllSecretKeys() {
		b.StoreCheckSum(name, utils.ComputeSecretChecksum(b.LoadSecret(name).Data))
	}

	// copy ssh-keypair.old secret to shoot namespace in seed
	oldSSHKeyPair, err := infodata.GetInfoData(gardenerResourceDataList, v1beta1constants.SecretNameOldSSHKeyPair)
	if err != nil {
		return err
	}
	if oldSSHKeyPair != nil {
		secretConfig := &secrets.RSASecretConfig{
			Name:       v1beta1constants.SecretNameOldSSHKeyPair,
			Bits:       4096,
			UsedForSSH: true,
		}

		oldSSHKeyPairData, err := secretConfig.GenerateFromInfoData(oldSSHKeyPair)
		if err != nil {
			return err
		}

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.SecretNameOldSSHKeyPair,
				Namespace: b.Shoot.SeedNamespace,
			},
		}
		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, b.K8sSeedClient.Client(), secret, func() error {
			secret.Type = corev1.SecretTypeOpaque
			secret.Data = oldSSHKeyPairData.SecretData()
			return nil
		}); err != nil {
			return err
		}
		b.StoreSecret(v1beta1constants.SecretNameOldSSHKeyPair, secret)
	}

	wildcardCert, err := seed.GetWildcardCertificate(ctx, b.K8sSeedClient.Client())
	if err != nil {
		return err
	}

	if wildcardCert != nil {
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
	}

	return b.reconcileGenericKubeconfigSecret(ctx)
}

// TODO(rfranzke): Remove this function in a future release.
func (b *Botanist) reconcileGenericKubeconfigSecret(ctx context.Context) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1beta1constants.SecretNameGenericTokenKubeconfig,
			Namespace: b.Shoot.SeedNamespace,
		},
	}

	kubeconfig, err := runtime.Encode(clientcmdlatest.Codec, kutil.NewKubeconfig(
		b.Shoot.SeedNamespace,
		clientcmdv1.Cluster{
			Server:                   b.Shoot.ComputeInClusterAPIServerAddress(true),
			CertificateAuthorityData: b.LoadSecret(v1beta1constants.SecretNameCACluster).Data[secretutils.DataKeyCertificateCA],
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

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, b.K8sSeedClient.Client(), secret, func() error {
		secret.Annotations = map[string]string{
			"checksum/data": checksum,
		}
		secret.Labels = map[string]string{
			v1beta1constants.GardenerPurpose: v1beta1constants.SecretNameCloudProvider,
		}
		secret.Type = corev1.SecretTypeOpaque
		secret.Data = b.Shoot.Secret.Data
		return nil
	}); err != nil {
		return err
	}

	b.StoreSecret(v1beta1constants.SecretNameCloudProvider, b.Shoot.Secret)
	b.StoreCheckSum(v1beta1constants.SecretNameCloudProvider, checksum)

	return nil
}

func (b *Botanist) fetchExistingSecrets(ctx context.Context) (map[string]*corev1.Secret, error) {
	secretList := &corev1.SecretList{}
	if err := b.K8sSeedClient.Client().List(ctx, secretList, client.InNamespace(b.Shoot.SeedNamespace)); err != nil {
		return nil, err
	}

	existingSecretsMap := make(map[string]*corev1.Secret, len(secretList.Items))
	for _, secret := range secretList.Items {
		secretObj := secret
		existingSecretsMap[secret.Name] = &secretObj
	}

	return existingSecretsMap, nil
}

// deleteSecrets removes the given secrets from the shoot namespace in the seed
// as well as removes it from the given `gardenerResourceDataList`.
func (b *Botanist) deleteSecrets(ctx context.Context, gardenerResourceDataList *gardencorev1alpha1helper.GardenerResourceDataList, secretNames ...string) error {
	for _, secretName := range secretNames {
		if err := b.K8sSeedClient.Client().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: b.Shoot.SeedNamespace}}); client.IgnoreNotFound(err) != nil {
			return err
		}
		gardenerResourceDataList.Delete(secretName)
	}
	return nil
}

func (b *Botanist) rotateKubeconfigSecrets(ctx context.Context, gardenerResourceDataList *gardencorev1alpha1helper.GardenerResourceDataList) error {
	secrets := []string{
		kubeapiserver.SecretNameStaticToken,
		kubeapiserver.SecretNameBasicAuth,
		common.KubecfgSecretName,
	}

	if err := b.deleteSecrets(ctx, gardenerResourceDataList, secrets...); err != nil {
		return err
	}

	// remove operation annotation
	return b.Shoot.UpdateInfo(ctx, b.K8sGardenClient.Client(), false, func(shoot *gardencorev1beta1.Shoot) error {
		delete(shoot.Annotations, v1beta1constants.GardenerOperation)
		return nil
	})
}

func (b *Botanist) rotateSSHKeypairSecrets(ctx context.Context, gardenerResourceDataList *gardencorev1alpha1helper.GardenerResourceDataList) error {
	currentSecret := gardenerResourceDataList.Get(v1beta1constants.SecretNameSSHKeyPair)
	if currentSecret == nil {
		b.Logger.Debugf("No %s Secret loaded, not rotating keypair.", v1beta1constants.SecretNameSSHKeyPair)
		return nil
	}

	// copy current key to old secret
	oldSecret := currentSecret.DeepCopy()
	oldSecret.Name = v1beta1constants.SecretNameOldSSHKeyPair
	gardenerResourceDataList.Upsert(oldSecret)

	if err := b.K8sSeedClient.Client().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.SecretNameSSHKeyPair, Namespace: b.Shoot.SeedNamespace}}); client.IgnoreNotFound(err) != nil {
		return err
	}

	gardenerResourceDataList.Delete(v1beta1constants.SecretNameSSHKeyPair)

	// remove operation annotation
	return b.Shoot.UpdateInfo(ctx, b.K8sGardenClient.Client(), false, func(shoot *gardencorev1beta1.Shoot) error {
		delete(shoot.Annotations, v1beta1constants.GardenerOperation)
		return nil
	})
}

func getExpiredCerts(gardenerResourceDataList gardencorev1alpha1helper.GardenerResourceDataList, renewalWindow time.Duration, secretNames ...string) ([]string, error) {
	var expiredCerts []string

	for _, secretName := range secretNames {
		data := gardenerResourceDataList.Get(secretName)
		if data == nil {
			continue
		}

		certObj := &secretutils.CertificateJSONData{}
		if err := json.Unmarshal(data.Data.Raw, certObj); err != nil {
			return nil, err
		}

		expired, err := secretutils.CertificateIsExpired(clock.RealClock{}, certObj.Certificate, renewalWindow)
		if err != nil {
			return nil, err
		}

		if expired {
			expiredCerts = append(expiredCerts, secretName)
		}
	}
	return expiredCerts, nil
}

type projectSecret struct {
	secretName  string
	suffix      string
	annotations map[string]string
	labels      map[string]string
}

// SyncShootCredentialsToGarden copies the kubeconfig generated for the user, the SSH keypair to
// the project namespace in the Garden cluster and the monitoring credentials for the
// user-facing monitoring stack are also copied.
func (b *Botanist) SyncShootCredentialsToGarden(ctx context.Context) error {
	// Secrets which are created by Gardener itself are usually excluded from informers to improve performance.
	// Hence, if new secrets are synced to the Garden cluster, please consider adding the used `gardener.cloud/role`
	// label value to the `v1beta1constants.ControlPlaneSecretRoles` list.
	projectSecrets := []projectSecret{
		{
			secretName: v1beta1constants.SecretNameSSHKeyPair,
			suffix:     gutil.ShootProjectSecretSuffixSSHKeypair,
			labels:     map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleSSHKeyPair},
		},
		{
			secretName:  "monitoring-ingress-credentials-users",
			suffix:      gutil.ShootProjectSecretSuffixMonitoring,
			annotations: map[string]string{"url": "https://" + b.ComputeGrafanaUsersHost()},
			labels:      map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleMonitoring},
		},
	}

	// ssh-keypair.old secret will be synced to the Garden cluster if it is present in shoot namespace in seed.
	oldSecret := &corev1.Secret{}
	if err := b.K8sSeedClient.Client().Get(ctx, kutil.Key(b.Shoot.SeedNamespace, v1beta1constants.SecretNameOldSSHKeyPair), oldSecret); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	} else {
		projectSecrets = append(projectSecrets, projectSecret{
			secretName: v1beta1constants.SecretNameOldSSHKeyPair,
			suffix:     gutil.ShootProjectSecretSuffixOldSSHKeypair,
			labels:     map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleSSHKeyPair},
		})
	}

	var fns []flow.TaskFn
	for _, projectSecret := range projectSecrets {
		s := projectSecret
		fns = append(fns, func(ctx context.Context) error {
			secretObj := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gutil.ComputeShootProjectSecretName(b.Shoot.GetInfo().Name, s.suffix),
					Namespace: b.Shoot.GetInfo().Namespace,
				},
			}

			_, err := controllerutils.GetAndCreateOrStrategicMergePatch(ctx, b.K8sGardenClient.Client(), secretObj, func() error {
				secretObj.OwnerReferences = []metav1.OwnerReference{
					*metav1.NewControllerRef(b.Shoot.GetInfo(), gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot")),
				}
				secretObj.Annotations = s.annotations
				secretObj.Labels = s.labels
				secretObj.Type = corev1.SecretTypeOpaque
				secretObj.Data = b.LoadSecret(s.secretName).Data
				return nil
			})
			return err
		})
	}

	return flow.Parallel(fns...)(ctx)
}

func (b *Botanist) cleanupSecrets(ctx context.Context, gardenerResourceDataList *gardencorev1alpha1helper.GardenerResourceDataList, secretNames ...string) error {
	for _, secret := range secretNames {
		if err := b.K8sSeedClient.Client().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secret, Namespace: b.Shoot.SeedNamespace}}); client.IgnoreNotFound(err) != nil {
			return err
		}
		gardenerResourceDataList.Delete(secret)
	}
	return nil
}

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
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/logging"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnshoot"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/operation/shootsecrets"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
			"kube-scheduler",
			"kube-controller-manager",
			"cluster-autoscaler",
			"kube-state-metrics",
			// TODO(rfranzke): Uncomment this in a future release once all monitoring configurations of extensions have been
			// adapted.
			// "prometheus",
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

		shootWantsBasicAuth := gardencorev1beta1helper.ShootWantsBasicAuthentication(b.Shoot.GetInfo())
		shootHasBasicAuth := gardenerResourceDataList.Get(kubeapiserver.SecretNameBasicAuth) != nil
		if shootWantsBasicAuth != shootHasBasicAuth {
			if err := b.deleteBasicAuthDependantSecrets(ctx, &gardenerResourceDataList); err != nil {
				return err
			}
		}

		secretsManager := shootsecrets.NewSecretsManager(
			gardenerResourceDataList,
			b.generateStaticTokenConfig(),
			b.wantedCertificateAuthorities(),
			b.generateWantedSecretConfigs,
		)

		if shootWantsBasicAuth {
			secretsManager = secretsManager.WithAPIServerBasicAuthConfig(basicAuthSecretAPIServer)
		}

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
		b.generateStaticTokenConfig(),
		b.wantedCertificateAuthorities(),
		b.generateWantedSecretConfigs,
	)

	if gardencorev1beta1helper.ShootWantsBasicAuthentication(b.Shoot.GetInfo()) {
		secretsManager.WithAPIServerBasicAuthConfig(basicAuthSecretAPIServer)
	}

	if err := secretsManager.WithExistingSecrets(existingSecrets).Deploy(ctx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace); err != nil {
		return err
	}

	if err := b.storeAPIServerHealthCheckToken(secretsManager.StaticToken); err != nil {
		return err
	}

	if b.isShootNodeLoggingEnabled() {
		if err := b.storePromtailRBACAuthToken(secretsManager.StaticToken); err != nil {
			return err
		}
	}

	for name, secret := range secretsManager.DeployedSecrets {
		b.StoreSecret(name, secret)
	}
	for _, name := range b.AllSecretKeys() {
		b.StoreCheckSum(name, utils.ComputeSecretChecksum(b.LoadSecret(name).Data))
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

func (b *Botanist) reconcileGenericKubeconfigSecret(ctx context.Context) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1beta1constants.SecretNameGenericTokenKubeconfig,
			Namespace: b.Shoot.SeedNamespace,
		},
	}

	kubeconfig, err := runtime.Encode(clientcmdlatest.Codec, kutil.NewKubeconfig(
		b.Shoot.SeedNamespace,
		b.Shoot.ComputeInClusterAPIServerAddress(true),
		b.LoadSecret(v1beta1constants.SecretNameCACluster).Data[secretutils.DataKeyCertificateCA],
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

	names := []string{
		v1beta1constants.SecretNameSSHKeyPair,
		v1beta1constants.SecretNameOldSSHKeyPair,
	}

	for _, secretName := range names {
		if err := b.K8sSeedClient.Client().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: b.Shoot.SeedNamespace}}); client.IgnoreNotFound(err) != nil {
			return err
		}
	}
	gardenerResourceDataList.Delete(v1beta1constants.SecretNameSSHKeyPair)

	// remove operation annotation
	return b.Shoot.UpdateInfo(ctx, b.K8sGardenClient.Client(), false, func(shoot *gardencorev1beta1.Shoot) error {
		delete(shoot.Annotations, v1beta1constants.GardenerOperation)
		return nil
	})
}

func (b *Botanist) deleteBasicAuthDependantSecrets(ctx context.Context, gardenerResourceDataList *gardencorev1alpha1helper.GardenerResourceDataList) error {
	return b.deleteSecrets(ctx, gardenerResourceDataList, kubeapiserver.SecretNameBasicAuth, common.KubecfgSecretName)
}

func (b *Botanist) storeAPIServerHealthCheckToken(staticToken *secretutils.StaticToken) error {
	kubeAPIServerHealthCheckToken, err := staticToken.GetTokenForUsername(common.KubeAPIServerHealthCheck)
	if err != nil {
		return err
	}

	b.APIServerHealthCheckToken = kubeAPIServerHealthCheckToken.Token
	return nil
}

func (b *Botanist) storePromtailRBACAuthToken(staticToken *secretutils.StaticToken) error {
	promtailRBACAuthToken, err := staticToken.GetTokenForUsername(logging.PromtailRBACName)
	if err != nil {
		return err
	}

	b.PromtailRBACAuthToken = promtailRBACAuthToken.Token
	return nil
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

		expired, err := secretutils.CertificateIsExpired(certObj.Certificate, renewalWindow)
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
	kubecfgURL := gutil.GetAPIServerDomain(b.Shoot.InternalClusterDomain)
	if b.Shoot.ExternalClusterDomain != nil {
		kubecfgURL = gutil.GetAPIServerDomain(*b.Shoot.ExternalClusterDomain)
	}

	// Secrets which are created by Gardener itself are usually excluded from informers to improve performance.
	// Hence, if new secrets are synced to the Garden cluster, please consider adding the used `gardener.cloud/role`
	// label value to the `v1beta1constants.ControlPlaneSecretRoles` list.
	projectSecrets := []projectSecret{
		{
			secretName:  common.KubecfgSecretName,
			suffix:      gutil.ShootProjectSecretSuffixKubeconfig,
			annotations: map[string]string{"url": "https://" + kubecfgURL},
			labels:      map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleKubeconfig},
		},
		{
			secretName: v1beta1constants.SecretNameSSHKeyPair,
			suffix:     gutil.ShootProjectSecretSuffixSSHKeypair,
			labels:     map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleSSHKeyPair},
		},
		{
			secretName: v1beta1constants.SecretNameOldSSHKeyPair,
			suffix:     gutil.ShootProjectSecretSuffixOldSSHKeypair,
			labels:     map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleSSHKeyPair},
		},
		{
			secretName:  "monitoring-ingress-credentials-users",
			suffix:      gutil.ShootProjectSecretSuffixMonitoring,
			annotations: map[string]string{"url": "https://" + b.ComputeGrafanaUsersHost()},
			labels:      map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleMonitoring},
		},
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

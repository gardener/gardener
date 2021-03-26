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
	"fmt"
	"reflect"

	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/operation/botanist/component/konnectivity"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/operation/shootsecrets"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/secrets"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// GenerateAndSaveSecrets creates a CA certificate for the Shoot cluster and uses it to sign the server certificate
// used by the kube-apiserver, and all client certificates used for communication. It also creates RSA key
// pairs for SSH connections to the nodes/VMs and for the VPN tunnel. Moreover, basic authentication
// credentials are computed which will be used to secure the Ingress resources and the kube-apiserver itself.
// Server certificates for the exposed monitoring endpoints (via Ingress) are generated as well.
func (b *Botanist) GenerateAndSaveSecrets(ctx context.Context) error {
	gardenerResourceDataList := gardencorev1alpha1helper.GardenerResourceDataList(b.ShootState.Spec.Gardener).DeepCopy()

	if val, ok := b.Shoot.Info.Annotations[v1beta1constants.GardenerOperation]; ok && val == common.ShootOperationRotateKubeconfigCredentials {
		if err := b.rotateKubeconfigSecrets(ctx, &gardenerResourceDataList); err != nil {
			return err
		}
	}

	if b.Shoot.Info.DeletionTimestamp == nil {
		if b.Shoot.KonnectivityTunnelEnabled {
			if err := b.cleanupTunnelSecrets(ctx, &gardenerResourceDataList, "vpn-seed", "vpn-seed-tlsauth", "vpn-shoot"); err != nil {
				return err
			}
		} else {
			if err := b.cleanupTunnelSecrets(ctx, &gardenerResourceDataList, konnectivity.SecretNameServerKubeconfig, konnectivity.SecretNameServerTLS); err != nil {
				return err
			}
		}
	}

	shootWantsBasicAuth := gardencorev1beta1helper.ShootWantsBasicAuthentication(b.Shoot.Info)
	shootHasBasicAuth := gardenerResourceDataList.Get(common.BasicAuthSecretName) != nil
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

	if reflect.DeepEqual(secretsManager.GardenerResourceDataList, gardencorev1alpha1helper.GardenerResourceDataList(b.ShootState.Spec.Gardener)) {
		return nil
	}
	return b.SaveGardenerResourcesInShootState(ctx, secretsManager.GardenerResourceDataList)
}

// DeploySecrets takes all existing secrets from the ShootState resource and deploys them in the shoot's control plane.
func (b *Botanist) DeploySecrets(ctx context.Context) error {
	gardenerResourceDataList := gardencorev1alpha1helper.GardenerResourceDataList(b.ShootState.Spec.Gardener)
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

	if gardencorev1beta1helper.ShootWantsBasicAuthentication(b.Shoot.Info) {
		secretsManager.WithAPIServerBasicAuthConfig(basicAuthSecretAPIServer)
	}

	if err := secretsManager.WithExistingSecrets(existingSecrets).Deploy(ctx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace); err != nil {
		return err
	}

	if err := b.storeAPIServerHealthCheckToken(secretsManager.StaticToken); err != nil {
		return err
	}

	if b.Shoot.WantsVerticalPodAutoscaler {
		if err := b.storeStaticTokenAsSecrets(ctx, secretsManager.StaticToken, secretsManager.DeployedSecrets[v1beta1constants.SecretNameCACluster].Data[secrets.DataKeyCertificateCA], vpaSecrets); err != nil {
			return err
		}
	}

	func() {
		b.mutex.Lock()
		defer b.mutex.Unlock()
		for name, secret := range secretsManager.DeployedSecrets {
			b.Secrets[name] = secret
		}
		for name, secret := range b.Secrets {
			b.CheckSums[name] = utils.ComputeSecretCheckSum(secret.Data)
		}
	}()

	wildcardCert, err := seed.GetWildcardCertificate(ctx, b.K8sSeedClient.Client())
	if err != nil {
		return err
	}

	if wildcardCert != nil {
		// Copy certificate to shoot namespace
		crt := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      wildcardCert.GetName(),
				Namespace: b.Shoot.SeedNamespace,
			},
		}

		if _, err := controllerutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), crt, func() error {
			crt.Data = wildcardCert.Data
			return nil
		}); err != nil {
			return err
		}

		b.ControlPlaneWildcardCert = crt
	}

	return nil
}

// DeployCloudProviderSecret creates or updates the cloud provider secret in the Shoot namespace
// in the Seed cluster.
func (b *Botanist) DeployCloudProviderSecret(ctx context.Context) error {
	var (
		checksum = utils.ComputeSecretCheckSum(b.Shoot.Secret.Data)
		secret   = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: b.Shoot.SeedNamespace,
			},
		}
	)

	if _, err := controllerutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), secret, func() error {
		secret.Annotations = map[string]string{
			"checksum/data": checksum,
		}
		secret.Type = corev1.SecretTypeOpaque
		secret.Data = b.Shoot.Secret.Data
		return nil
	}); err != nil {
		return err
	}

	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.Secrets[v1beta1constants.SecretNameCloudProvider] = b.Shoot.Secret
	b.CheckSums[v1beta1constants.SecretNameCloudProvider] = checksum

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

func (b *Botanist) rotateKubeconfigSecrets(ctx context.Context, gardenerResourceDataList *gardencorev1alpha1helper.GardenerResourceDataList) error {
	secrets := []string{
		common.StaticTokenSecretName,
		common.BasicAuthSecretName,
		common.KubecfgSecretName,
	}
	if b.Shoot.KonnectivityTunnelEnabled {
		secrets = append(secrets, konnectivity.SecretNameServerKubeconfig)
	}

	for _, secretName := range secrets {
		if err := b.K8sSeedClient.Client().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: b.Shoot.SeedNamespace}}); client.IgnoreNotFound(err) != nil {
			return err
		}
		gardenerResourceDataList.Delete(secretName)
	}

	oldObj := b.Shoot.Info.DeepCopy()
	delete(b.Shoot.Info.Annotations, v1beta1constants.GardenerOperation)
	return b.K8sGardenClient.Client().Patch(ctx, b.Shoot.Info, client.MergeFrom(oldObj))
}

func (b *Botanist) deleteBasicAuthDependantSecrets(ctx context.Context, gardenerResourceDataList *gardencorev1alpha1helper.GardenerResourceDataList) error {
	for _, secretName := range []string{common.BasicAuthSecretName, common.KubecfgSecretName} {
		if err := b.K8sSeedClient.Client().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: b.Shoot.SeedNamespace}}); client.IgnoreNotFound(err) != nil {
			return err
		}
		gardenerResourceDataList.Delete(secretName)
	}
	return nil
}

func (b *Botanist) storeAPIServerHealthCheckToken(staticToken *secrets.StaticToken) error {
	kubeAPIServerHealthCheckToken, err := staticToken.GetTokenForUsername(common.KubeAPIServerHealthCheck)
	if err != nil {
		return err
	}

	b.APIServerHealthCheckToken = kubeAPIServerHealthCheckToken.Token
	return nil
}

func (b *Botanist) storeStaticTokenAsSecrets(ctx context.Context, staticToken *secrets.StaticToken, caCert []byte, secretNameToUsername map[string]string) error {
	for secretName, username := range secretNameToUsername {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: b.Shoot.SeedNamespace,
			},
			Type: corev1.SecretTypeOpaque,
		}

		token, err := staticToken.GetTokenForUsername(username)
		if err != nil {
			return err
		}

		if _, err := controllerutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), secret, func() error {
			secret.Data = map[string][]byte{
				secrets.DataKeyToken:         []byte(token.Token),
				secrets.DataKeyCertificateCA: caCert,
			}
			return nil
		}); err != nil {
			return err
		}

		b.CheckSums[secretName] = utils.ComputeSecretCheckSum(secret.Data)
	}

	return nil
}

const (
	secretSuffixKubeConfig = "kubeconfig"
	secretSuffixSSHKeyPair = v1beta1constants.SecretNameSSHKeyPair
	secretSuffixMonitoring = "monitoring"
)

func computeProjectSecretName(shootName, suffix string) string {
	return fmt.Sprintf("%s.%s", shootName, suffix)
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
			suffix:      secretSuffixKubeConfig,
			annotations: map[string]string{"url": "https://" + kubecfgURL},
			labels:      map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleKubeconfig},
		},
		{
			secretName: v1beta1constants.SecretNameSSHKeyPair,
			suffix:     secretSuffixSSHKeyPair,
			labels:     map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleSSHKeyPair},
		},
		{
			secretName:  "monitoring-ingress-credentials-users",
			suffix:      secretSuffixMonitoring,
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
					Name:      computeProjectSecretName(b.Shoot.Info.Name, s.suffix),
					Namespace: b.Shoot.Info.Namespace,
				},
			}

			_, err := controllerutil.CreateOrUpdate(ctx, b.K8sGardenClient.Client(), secretObj, func() error {
				secretObj.OwnerReferences = []metav1.OwnerReference{
					*metav1.NewControllerRef(b.Shoot.Info, gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot")),
				}
				secretObj.Annotations = s.annotations
				secretObj.Labels = s.labels
				secretObj.Type = corev1.SecretTypeOpaque
				secretObj.Data = b.Secrets[s.secretName].Data
				return nil
			})
			return err
		})
	}

	return flow.Parallel(fns...)(ctx)
}

func (b *Botanist) cleanupTunnelSecrets(ctx context.Context, gardenerResourceDataList *gardencorev1alpha1helper.GardenerResourceDataList, secretNames ...string) error {
	// TODO: remove when all Gardener supported versions are >= 1.18
	for _, secret := range secretNames {
		if err := b.K8sSeedClient.Client().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secret, Namespace: b.Shoot.SeedNamespace}}); client.IgnoreNotFound(err) != nil {
			return err
		}
		gardenerResourceDataList.Delete(secret)
	}
	return nil
}

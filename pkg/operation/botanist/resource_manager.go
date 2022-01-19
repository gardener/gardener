// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"strings"
	"time"

	"github.com/gardener/gardener/charts"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/resourcemanager"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	retryutils "github.com/gardener/gardener/pkg/utils/retry"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/component-base/version"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DefaultResourceManager returns an instance of Gardener Resource Manager with defaults configured for being deployed in a Shoot namespace
func (b *Botanist) DefaultResourceManager() (resourcemanager.Interface, error) {
	image, err := b.ImageVector.FindImage(charts.ImageNameGardenerResourceManager)
	if err != nil {
		return nil, err
	}

	repository, tag := image.String(), version.Get().GitVersion
	if image.Tag != nil {
		repository, tag = image.Repository, *image.Tag
	}
	image = &imagevector.Image{Repository: repository, Tag: &tag}

	cfg := resourcemanager.Values{
		AlwaysUpdate:                         pointer.Bool(true),
		ClusterIdentity:                      b.Seed.GetInfo().Status.ClusterIdentity,
		ConcurrentSyncs:                      pointer.Int32(20),
		HealthSyncPeriod:                     utils.DurationPtr(time.Minute),
		MaxConcurrentHealthWorkers:           pointer.Int32(10),
		MaxConcurrentTokenInvalidatorWorkers: pointer.Int32(5),
		MaxConcurrentTokenRequestorWorkers:   pointer.Int32(5),
		MaxConcurrentRootCAPublisherWorkers:  pointer.Int32(5),
		SyncPeriod:                           utils.DurationPtr(time.Minute),
		TargetDiffersFromSourceCluster:       true,
		TargetDisableCache:                   pointer.Bool(true),
		WatchedNamespace:                     pointer.String(b.Shoot.SeedNamespace),
		VPA: &resourcemanager.VPAConfig{
			MinAllowed: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("20m"),
				corev1.ResourceMemory: resource.MustParse("30Mi"),
			},
		},
	}

	return resourcemanager.New(
		b.K8sSeedClient.Client(),
		b.Shoot.SeedNamespace,
		image.String(),
		cfg,
	), nil
}

// TimeoutWaitForGardenerResourceManagerBootstrapping is the maximum time the bootstrap process for the
// gardener-resource-manager may take.
// Exposed for testing.
var TimeoutWaitForGardenerResourceManagerBootstrapping = 2 * time.Minute

// DeployGardenerResourceManager deploys the gardener-resource-manager
func (b *Botanist) DeployGardenerResourceManager(ctx context.Context) error {
	var (
		bootstrapKubeconfigSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-access-gardener-resource-manager-bootstrap",
				Namespace: b.Shoot.SeedNamespace,
			},
		}
		secrets = resourcemanager.Secrets{
			ServerCA: component.Secret{Name: v1beta1constants.SecretNameCACluster, Checksum: b.LoadCheckSum(v1beta1constants.SecretNameCACluster), Data: b.LoadSecret(v1beta1constants.SecretNameCACluster).Data},
			Server:   component.Secret{Name: resourcemanager.SecretNameServer, Checksum: b.LoadCheckSum(resourcemanager.SecretNameServer)},
			RootCA:   &component.Secret{Name: v1beta1constants.SecretNameCACluster, Checksum: b.LoadCheckSum(v1beta1constants.SecretNameCACluster)},
		}
	)

	if b.Shoot.Components.ControlPlane.ResourceManager.GetReplicas() == nil {
		replicaCount, err := b.determineControllerReplicas(ctx, v1beta1constants.DeploymentNameGardenerResourceManager, 3)
		if err != nil {
			return err
		}
		b.Shoot.Components.ControlPlane.ResourceManager.SetReplicas(&replicaCount)
	}

	mustBootstrap, err := b.mustBootstrapGardenerResourceManager(ctx)
	if err != nil {
		return err
	}

	if mustBootstrap {
		if err := b.reconcileGardenerResourceManagerBootstrapKubeconfigSecret(ctx, bootstrapKubeconfigSecret); err != nil {
			return err
		}

		secrets.BootstrapKubeconfig = &component.Secret{Name: bootstrapKubeconfigSecret.Name, Checksum: utils.ComputeSecretChecksum(bootstrapKubeconfigSecret.Data)}
		b.Shoot.Components.ControlPlane.ResourceManager.SetSecrets(secrets)

		if err := b.Shoot.Components.ControlPlane.ResourceManager.Deploy(ctx); err != nil {
			return err
		}

		timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForGardenerResourceManagerBootstrapping)
		defer cancel()

		if err := b.waitUntilGardenerResourceManagerBootstrapped(timeoutCtx); err != nil {
			return err
		}
	}

	if err := b.K8sSeedClient.Client().Delete(ctx, bootstrapKubeconfigSecret); client.IgnoreNotFound(err) != nil {
		return err
	}

	secrets.BootstrapKubeconfig = nil
	b.Shoot.Components.ControlPlane.ResourceManager.SetSecrets(secrets)

	return b.Shoot.Components.ControlPlane.ResourceManager.Deploy(ctx)
}

// ScaleGardenerResourceManagerToOne scales the gardener-resource-manager deployment
func (b *Botanist) ScaleGardenerResourceManagerToOne(ctx context.Context) error {
	return kubernetes.ScaleDeployment(ctx, b.K8sSeedClient.Client(), kutil.Key(b.Shoot.SeedNamespace, v1beta1constants.DeploymentNameGardenerResourceManager), 1)
}

func (b *Botanist) mustBootstrapGardenerResourceManager(ctx context.Context) (bool, error) {
	if pointer.Int32Deref(b.Shoot.Components.ControlPlane.ResourceManager.GetReplicas(), 0) == 0 {
		return false, nil // GRM should not be scaled up, hence no need to bootstrap.
	}

	shootAccessSecret := gutil.NewShootAccessSecret(resourcemanager.SecretNameShootAccess, b.Shoot.SeedNamespace)
	if err := b.K8sSeedClient.Client().Get(ctx, client.ObjectKeyFromObject(shootAccessSecret.Secret), shootAccessSecret.Secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, err
		}
		return true, nil // Shoot access secret does not yet exist.
	}

	renewTimestamp, ok := shootAccessSecret.Secret.Annotations[resourcesv1alpha1.ServiceAccountTokenRenewTimestamp]
	if !ok {
		return true, nil // Shoot access secret was never reconciled yet
	}

	renewTime, err2 := time.Parse(time.RFC3339, renewTimestamp)
	if err2 != nil {
		return false, fmt.Errorf("could not parse renew timestamp: %w", err2)
	}
	if time.Now().UTC().After(renewTime.UTC()) {
		return true, nil // Shoot token was not renewed.
	}

	managedResource := &resourcesv1alpha1.ManagedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourcemanager.ManagedResourceName,
			Namespace: b.Shoot.SeedNamespace,
		},
	}

	if err := b.K8sSeedClient.Client().Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, err
		}
		return true, nil // ManagedResource (containing the RBAC resources) does not yet exist.
	}

	if conditionApplied := v1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied); conditionApplied != nil &&
		conditionApplied.Status == gardencorev1beta1.ConditionFalse &&
		strings.Contains(conditionApplied.Message, `forbidden: User "system:serviceaccount:kube-system:gardener-resource-manager" cannot`) {
		return true, nil // ServiceAccount lost access.
	}

	return false, nil
}

func (b *Botanist) reconcileGardenerResourceManagerBootstrapKubeconfigSecret(ctx context.Context, bootstrapKubeconfigSecret *corev1.Secret) error {
	caCertificateSecret := b.LoadSecret(v1beta1constants.SecretNameCACluster)

	caCertificate, err := secretutils.LoadCertificate(v1beta1constants.SecretNameCACluster, caCertificateSecret.Data[secretutils.DataKeyPrivateKeyCA], caCertificateSecret.Data[secretutils.DataKeyCertificateCA])
	if err != nil {
		return err
	}

	controlPlaneSecret, err := (&secretutils.ControlPlaneSecretConfig{
		CertificateSecretConfig: &secretutils.CertificateSecretConfig{
			Name:         bootstrapKubeconfigSecret.Name,
			CommonName:   "gardener.cloud:system:gardener-resource-manager",
			Organization: []string{user.SystemPrivilegedGroup},
			SigningCA:    caCertificate,
			CertType:     secretutils.ClientCert,
			Validity:     utils.DurationPtr(10 * time.Minute),
		},
		KubeConfigRequests: []secretutils.KubeConfigRequest{{
			ClusterName:   b.Shoot.SeedNamespace,
			APIServerHost: b.Shoot.ComputeInClusterAPIServerAddress(true),
		}},
	}).GenerateControlPlane()
	if err != nil {
		return err
	}

	_, err = controllerutils.GetAndCreateOrMergePatch(ctx, b.K8sSeedClient.Client(), bootstrapKubeconfigSecret, func() error {
		bootstrapKubeconfigSecret.Type = corev1.SecretTypeOpaque
		bootstrapKubeconfigSecret.Data = map[string][]byte{resourcesv1alpha1.DataKeyKubeconfig: controlPlaneSecret.Kubeconfig}
		return nil
	})
	return err
}

func (b *Botanist) waitUntilGardenerResourceManagerBootstrapped(ctx context.Context) error {
	shootAccessSecret := gutil.NewShootAccessSecret(resourcemanager.SecretNameShootAccess, b.Shoot.SeedNamespace)

	if err := retryutils.Until(ctx, 5*time.Second, func(ctx context.Context) (bool, error) {
		if err2 := b.K8sSeedClient.Client().Get(ctx, client.ObjectKeyFromObject(shootAccessSecret.Secret), shootAccessSecret.Secret); err2 != nil {
			if apierrors.IsNotFound(err2) {
				return retryutils.MinorError(err2)
			}
			return retryutils.SevereError(err2)
		}

		renewTimestamp, ok := shootAccessSecret.Secret.Annotations[resourcesv1alpha1.ServiceAccountTokenRenewTimestamp]
		if !ok {
			return retryutils.MinorError(fmt.Errorf("token not yet generated"))
		}

		renewTime, err2 := time.Parse(time.RFC3339, renewTimestamp)
		if err2 != nil {
			return retryutils.SevereError(fmt.Errorf("could not parse renew timestamp: %w", err2))
		}

		if time.Now().UTC().After(renewTime.UTC()) {
			return retryutils.MinorError(fmt.Errorf("token not yet renewed"))
		}

		return retryutils.Ok()
	}); err != nil {
		return err
	}

	return managedresources.WaitUntilHealthy(ctx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace, resourcemanager.ManagedResourceName)
}

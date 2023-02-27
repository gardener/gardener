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

	"github.com/Masterminds/semver"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/component-base/version"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/resourcemanager"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	retryutils "github.com/gardener/gardener/pkg/utils/retry"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// DefaultResourceManager returns an instance of Gardener Resource Manager with defaults configured for being deployed in a Shoot namespace
func (b *Botanist) DefaultResourceManager() (resourcemanager.Interface, error) {
	image, err := b.ImageVector.FindImage(images.ImageNameGardenerResourceManager)
	if err != nil {
		return nil, err
	}

	repository, tag := image.String(), version.Get().GitVersion
	if image.Tag != nil {
		repository, tag = image.Repository, *image.Tag
	}
	image = &imagevector.Image{Repository: repository, Tag: &tag}

	version, err := semver.NewVersion(b.SeedClientSet.Version())
	if err != nil {
		return nil, err
	}

	cfg := resourcemanager.Values{
		AlwaysUpdate:                         pointer.Bool(true),
		ClusterIdentity:                      b.Seed.GetInfo().Status.ClusterIdentity,
		ConcurrentSyncs:                      pointer.Int(20),
		HealthSyncPeriod:                     &metav1.Duration{Duration: time.Minute},
		Image:                                image.String(),
		LogLevel:                             logger.InfoLevel,
		LogFormat:                            logger.FormatJSON,
		MaxConcurrentHealthWorkers:           pointer.Int(10),
		MaxConcurrentTokenInvalidatorWorkers: pointer.Int(5),
		MaxConcurrentTokenRequestorWorkers:   pointer.Int(5),
		MaxConcurrentCSRApproverWorkers:      pointer.Int(5),
		PodTopologySpreadConstraintsEnabled:  true,
		PriorityClassName:                    v1beta1constants.PriorityClassNameShootControlPlane400,
		SchedulingProfile:                    v1beta1helper.ShootSchedulingProfile(b.Shoot.GetInfo()),
		SecretNameServerCA:                   v1beta1constants.SecretNameCACluster,
		SyncPeriod:                           &metav1.Duration{Duration: time.Minute},
		SystemComponentTolerations:           gardenerutils.ExtractSystemComponentsTolerations(b.Shoot.GetInfo().Spec.Provider.Workers),
		TargetDiffersFromSourceCluster:       true,
		TargetDisableCache:                   pointer.Bool(true),
		KubernetesVersion:                    version,
		VPA: &resourcemanager.VPAConfig{
			MinAllowed: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("30Mi"),
			},
		},
		WatchedNamespace: pointer.String(b.Shoot.SeedNamespace),
	}

	return resourcemanager.New(
		b.SeedClientSet.Client(),
		b.Shoot.SeedNamespace,
		b.SecretsManager,
		cfg,
	), nil
}

// TimeoutWaitForGardenerResourceManagerBootstrapping is the maximum time the bootstrap process for the
// gardener-resource-manager may take.
// Exposed for testing.
var TimeoutWaitForGardenerResourceManagerBootstrapping = 2 * time.Minute

// DeployGardenerResourceManager deploys the gardener-resource-manager
func (b *Botanist) DeployGardenerResourceManager(ctx context.Context) error {
	var secrets resourcemanager.Secrets

	if b.Shoot.Components.ControlPlane.ResourceManager.GetReplicas() == nil {
		replicaCount, err := b.determineControllerReplicas(ctx, v1beta1constants.DeploymentNameGardenerResourceManager, 2, false)
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
		bootstrapKubeconfigSecret, err := b.reconcileGardenerResourceManagerBootstrapKubeconfigSecret(ctx)
		if err != nil {
			return err
		}

		secrets.BootstrapKubeconfig = &component.Secret{Name: bootstrapKubeconfigSecret.Name}
		b.Shoot.Components.ControlPlane.ResourceManager.SetSecrets(secrets)

		if err := b.Shoot.Components.ControlPlane.ResourceManager.Deploy(ctx); err != nil {
			return err
		}

		timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForGardenerResourceManagerBootstrapping)
		defer cancel()

		if err := b.waitUntilGardenerResourceManagerBootstrapped(timeoutCtx); err != nil {
			return err
		}

		if err := b.SeedClientSet.Client().Delete(ctx, bootstrapKubeconfigSecret); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	secrets.BootstrapKubeconfig = nil
	b.Shoot.Components.ControlPlane.ResourceManager.SetSecrets(secrets)

	return b.Shoot.Components.ControlPlane.ResourceManager.Deploy(ctx)
}

// ScaleGardenerResourceManagerToOne scales the gardener-resource-manager deployment
func (b *Botanist) ScaleGardenerResourceManagerToOne(ctx context.Context) error {
	return kubernetes.ScaleDeployment(ctx, b.SeedClientSet.Client(), kubernetesutils.Key(b.Shoot.SeedNamespace, v1beta1constants.DeploymentNameGardenerResourceManager), 1)
}

func (b *Botanist) mustBootstrapGardenerResourceManager(ctx context.Context) (bool, error) {
	if pointer.Int32Deref(b.Shoot.Components.ControlPlane.ResourceManager.GetReplicas(), 0) == 0 {
		return false, nil // GRM should not be scaled up, hence no need to bootstrap.
	}

	shootAccessSecret := gardenerutils.NewShootAccessSecret(resourcemanager.SecretNameShootAccess, b.Shoot.SeedNamespace)
	if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(shootAccessSecret.Secret), shootAccessSecret.Secret); err != nil {
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

	if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, err
		}
		return true, nil // ManagedResource (containing the RBAC resources) does not yet exist.
	}

	if conditionApplied := v1beta1helper.GetCondition(managedResource.Status.Conditions, resourcesv1alpha1.ResourcesApplied); conditionApplied != nil &&
		conditionApplied.Status == gardencorev1beta1.ConditionFalse &&
		(strings.Contains(conditionApplied.Message, `forbidden: User "system:serviceaccount:kube-system:gardener-resource-manager" cannot`) ||
			strings.Contains(conditionApplied.Message, ": Unauthorized")) {
		return true, nil // ServiceAccount lost access.
	}

	return false, nil
}

func (b *Botanist) reconcileGardenerResourceManagerBootstrapKubeconfigSecret(ctx context.Context) (*corev1.Secret, error) {
	caBundleSecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !found {
		return nil, fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
	}

	return b.SecretsManager.Generate(ctx, &secretsutils.ControlPlaneSecretConfig{
		Name: "shoot-access-gardener-resource-manager-bootstrap",
		CertificateSecretConfig: &secretsutils.CertificateSecretConfig{
			CommonName:                  "gardener.cloud:system:gardener-resource-manager",
			Organization:                []string{user.SystemPrivilegedGroup},
			CertType:                    secretsutils.ClientCert,
			Validity:                    pointer.Duration(10 * time.Minute),
			SkipPublishingCACertificate: true,
		},
		KubeConfigRequests: []secretsutils.KubeConfigRequest{{
			ClusterName:   b.Shoot.SeedNamespace,
			APIServerHost: b.Shoot.ComputeInClusterAPIServerAddress(true),
			CAData:        caBundleSecret.Data[secretsutils.DataKeyCertificateBundle],
		}},
	}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCAClient))
}

func (b *Botanist) waitUntilGardenerResourceManagerBootstrapped(ctx context.Context) error {
	shootAccessSecret := gardenerutils.NewShootAccessSecret(resourcemanager.SecretNameShootAccess, b.Shoot.SeedNamespace)

	if err := retryutils.Until(ctx, 5*time.Second, func(ctx context.Context) (bool, error) {
		if err2 := b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(shootAccessSecret.Secret), shootAccessSecret.Secret); err2 != nil {
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

	return managedresources.WaitUntilHealthy(ctx, b.SeedClientSet.Client(), b.Shoot.SeedNamespace, resourcemanager.ManagedResourceName)
}

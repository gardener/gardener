// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package operation

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/gardenlet/operation/garden"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// Initialize creates a new Operation from the given inputs.
// It is called by the gardenlet `Shoot` reconciler and by `gardenadm` so that both share the same `Operation`
// construction logic.
func Initialize(
	ctx context.Context,
	log logr.Logger,
	gardenAPIReader client.Reader,
	gardenClient client.Client,
	seedClientSet kubernetes.Interface,
	shootClientMap clientmap.ClientMap,
	config *gardenletconfigv1alpha1.GardenletConfiguration,
	gardenerInfo *gardencorev1beta1.Gardener,
	gardenClusterIdentity string,
	shoot *gardencorev1beta1.Shoot,
	project *gardencorev1beta1.Project,
	cloudProfile *gardencorev1beta1.CloudProfile,
	seed *gardencorev1beta1.Seed,
	exposureClass *gardencorev1beta1.ExposureClass,
) (
	*Operation,
	error,
) {
	var (
		gardenSecrets  map[string]*corev1.Secret
		internalDomain *gardenerutils.Domain
		defaultDomains []*gardenerutils.Domain
		err            error
	)

	if !v1beta1helper.IsShootSelfHosted(shoot.Spec.Provider.Workers) {
		gardenSecrets, err = gardenerutils.ReadGardenSecrets(
			ctx,
			log,
			gardenClient,
			gardenerutils.ComputeGardenNamespace(seed.Name),
		)
		if err != nil {
			return nil, err
		}

		internalDomain, err = gardenerutils.ReadGardenInternalDomain(
			ctx,
			gardenClient,
			gardenerutils.ComputeGardenNamespace(seed.Name),
			true,
			seed.Spec.DNS.Internal,
		)
		if err != nil {
			return nil, err
		}

		defaultDomains, err = gardenerutils.ReadGardenDefaultDomains(
			ctx,
			gardenClient,
			gardenerutils.ComputeGardenNamespace(seed.Name),
			seed.Spec.DNS.Defaults,
		)
		if err != nil {
			return nil, err
		}
	} else {
		// The self-hosted gardenlet only needs the shoot service account issuer secret. The shoot authorizer
		// only permits list/watch with the gardener.cloud/role=shoot-service-account-issuer label selector
		// (enforced via AuthorizeWithSelectors, GA in Kubernetes 1.34), so we scope the list to that role.
		// We use the uncached API reader because the self-hosted gardenlet uses SingleObjectCacheFunc for
		// secrets (Get-by-name only) and the label-selector list would bypass the cache anyway.
		gardenSecrets, err = gardenerutils.ReadGardenSecrets(
			ctx,
			log,
			gardenAPIReader,
			v1beta1constants.GardenNamespace,
			v1beta1constants.GardenRoleShootServiceAccountIssuer,
		)
		if err != nil {
			return nil, err
		}

		// See https://github.com/gardener/gardener/pull/14352
		if config.ETCDConfig.FeatureGates == nil {
			config.ETCDConfig.FeatureGates = make(map[string]bool)
		}
		config.ETCDConfig.FeatureGates["UpgradeEtcdVersion"] = true
	}

	gardenObj, err := garden.
		NewBuilder().
		WithProject(project).
		WithInternalDomain(internalDomain).
		WithDefaultDomains(defaultDomains).
		Build(ctx)
	if err != nil {
		return nil, err
	}

	shootBuilder := shootpkg.
		NewBuilder().
		WithProjectName(project.Name).
		WithCloudProfileObject(cloudProfile).
		WithShootObject(shoot).
		WithShootCredentialsFrom(gardenClient).
		WithExposureClassObject(exposureClass).
		WithInternalDomain(gardenObj.InternalDomain).
		WithDefaultDomains(gardenObj.DefaultDomains).
		WithServiceAccountIssuerHostname(gardenSecrets[v1beta1constants.GardenRoleShootServiceAccountIssuer])

	opBuilder := NewBuilder().
		WithLogger(log).
		WithConfig(config).
		WithGardenerInfo(gardenerInfo).
		WithGardenClusterIdentity(gardenClusterIdentity).
		WithSecrets(gardenSecrets).
		WithInternalDomain(gardenObj.InternalDomain).
		WithDefaultDomains(gardenObj.DefaultDomains).
		WithGarden(gardenObj)

	if seed != nil {
		seedObj, err := seedpkg.NewBuilder().WithSeedObject(seed).Build(ctx)
		if err != nil {
			return nil, err
		}

		shootBuilder = shootBuilder.WithSeedObject(seed)
		opBuilder = opBuilder.WithSeed(seedObj)
	}

	shootObj, err := shootBuilder.Build(ctx, seedClientSet, gardenClient)
	if err != nil {
		return nil, err
	}

	return opBuilder.WithShoot(shootObj).Build(ctx, gardenClient, seedClientSet, shootClientMap, shoot)
}

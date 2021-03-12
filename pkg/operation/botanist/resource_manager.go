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
	"time"

	"github.com/gardener/gardener/charts"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/resourcemanager"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"

	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// DefaultResourceManager returns an instance of Gardener Resource Manager with defaults configured for being deployed in a Shoot namespace
func (b *Botanist) DefaultResourceManager() (resourcemanager.ResourceManager, error) {
	image, err := b.ImageVector.FindImage(charts.ImageNameGardenerResourceManager, imagevector.RuntimeVersion(b.SeedVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	cfg := resourcemanager.Values{
		AlwaysUpdate:               pointer.BoolPtr(true),
		ClusterIdentity:            b.Seed.Info.Status.ClusterIdentity,
		ConcurrentSyncs:            pointer.Int32Ptr(20),
		HealthSyncPeriod:           utils.DurationPtr(time.Minute),
		MaxConcurrentHealthWorkers: pointer.Int32Ptr(10),
		SyncPeriod:                 utils.DurationPtr(time.Minute),
		TargetDisableCache:         pointer.BoolPtr(true),
		WatchedNamespace:           pointer.StringPtr(b.Shoot.SeedNamespace),
		// We run one GRM per shoot control plane, and the GRM is doing its leader election via configmaps in the seed -
		// by default every 2s. This can lead to a lot of PUT /v1/configmaps requests on the API server, and given that
		// a seed is very busy anyways, we should not unnecessarily stress the API server with this leader election.
		// The GRM's sync period is 1m anyways, so it doesn't matter too much if the leadership determination may take up
		// to one minute.
		LeaseDuration: utils.DurationPtr(time.Second * 40),
		RenewDeadline: utils.DurationPtr(time.Second * 15),
		RetryPeriod:   utils.DurationPtr(time.Second * 10),
	}

	return resourcemanager.New(
		b.K8sSeedClient.Client(),
		b.Shoot.SeedNamespace,
		image.String(),
		b.Shoot.GetReplicas(1),
		cfg,
	), nil
}

// DeployGardenerResourceManager deploys the gardener-resource-manager
func (b *Botanist) DeployGardenerResourceManager(ctx context.Context) error {
	kubeCfg := component.Secret{Name: resourcemanager.SecretName, Checksum: b.CheckSums[resourcemanager.SecretName]}
	b.Shoot.Components.ControlPlane.ResourceManager.SetSecrets(resourcemanager.Secrets{Kubeconfig: kubeCfg})

	// TODO (ialidzhikov): remove in a future version
	deploymentKeys := []client.ObjectKey{
		kutil.Key(b.Shoot.SeedNamespace, v1beta1constants.DeploymentNameGardenerResourceManager),
	}
	if err := common.DeleteDeploymentsHavingDeprecatedRoleLabelKey(ctx, b.K8sSeedClient.Client(), deploymentKeys); err != nil {
		return err
	}

	return b.Shoot.Components.ControlPlane.ResourceManager.Deploy(ctx)
}

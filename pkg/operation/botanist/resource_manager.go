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
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/resourcemanager"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/component-base/version"
	"k8s.io/utils/pointer"
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
		TargetDiffersFromSourceCluster:       true,
		SyncPeriod:                           utils.DurationPtr(time.Minute),
		TargetDisableCache:                   pointer.Bool(true),
		WatchedNamespace:                     pointer.String(b.Shoot.SeedNamespace),
		VPA: &resourcemanager.VPAConfig{
			MinAllowed: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("20m"),
				corev1.ResourceMemory: resource.MustParse("30Mi"),
			},
		},
	}

	// ensure grm is present during hibernation (if the cluster is not hibernated yet) to reconcile any changes to
	// MRs (e.g. caused by extension upgrades) that are necessary for completing the hibernation flow.
	// grm is scaled down later on as part of the HibernateControlPlane step, so we only specify replicas=0 if
	// the shoot is already hibernated.
	replicas := int32(3)
	if b.Shoot.HibernationEnabled && b.Shoot.GetInfo().Status.IsHibernated {
		replicas = 0
	}

	return resourcemanager.New(
		b.K8sSeedClient.Client(),
		b.Shoot.SeedNamespace,
		image.String(),
		replicas,
		cfg,
	), nil
}

// DeployGardenerResourceManager deploys the gardener-resource-manager
func (b *Botanist) DeployGardenerResourceManager(ctx context.Context) error {
	b.Shoot.Components.ControlPlane.ResourceManager.SetSecrets(resourcemanager.Secrets{
		Kubeconfig: component.Secret{Name: resourcemanager.SecretName, Checksum: b.LoadCheckSum(resourcemanager.SecretName)},
		ServerCA:   component.Secret{Name: v1beta1constants.SecretNameCACluster, Checksum: b.LoadCheckSum(v1beta1constants.SecretNameCACluster), Data: b.LoadSecret(v1beta1constants.SecretNameCACluster).Data},
		Server:     component.Secret{Name: resourcemanager.SecretNameServer, Checksum: b.LoadCheckSum(resourcemanager.SecretNameServer)},
		RootCA:     &component.Secret{Name: v1beta1constants.SecretNameCACluster, Checksum: b.LoadCheckSum(v1beta1constants.SecretNameCACluster)},
	})

	return b.Shoot.Components.ControlPlane.ResourceManager.Deploy(ctx)
}

// ScaleGardenerResourceManagerToOne scales the gardener-resource-manager deployment
func (b *Botanist) ScaleGardenerResourceManagerToOne(ctx context.Context) error {
	return kubernetes.ScaleDeployment(ctx, b.K8sSeedClient.Client(), kutil.Key(b.Shoot.SeedNamespace, v1beta1constants.DeploymentNameGardenerResourceManager), 1)
}

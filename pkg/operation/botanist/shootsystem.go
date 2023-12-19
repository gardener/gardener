// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/gardener/gardener/pkg/component/shootsystem"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// DefaultShootSystem returns a deployer for the shoot system resources.
func (b *Botanist) DefaultShootSystem() shootsystem.Interface {
	extensions := make([]string, 0, len(b.Shoot.Components.Extensions.Extension.Extensions()))
	for extensionType := range b.Shoot.Components.Extensions.Extension.Extensions() {
		extensions = append(extensions, extensionType)
	}
	slices.Sort(extensions)

	values := shootsystem.Values{
		Extensions:            extensions,
		ExternalClusterDomain: b.Shoot.ExternalClusterDomain,
		IsWorkerless:          b.Shoot.IsWorkerless,
		KubernetesVersion:     b.Shoot.KubernetesVersion,
		Object:                b.Shoot.GetInfo(),
		PodNetworkCIDR:        b.Shoot.Networks.Pods.String(),
		ServiceNetworkCIDR:    b.Shoot.Networks.Services.String(),
		ProjectName:           b.Garden.Project.Name,
		EncryptedResources:    append(sets.List(gardenerutils.DefaultResourcesForEncryption()), b.Shoot.ResourcesToEncrypt...),
	}

	return shootsystem.New(b.SeedClientSet.Client(), b.Shoot.SeedNamespace, values)
}

// DeployShootSystem deploys the shoot system resources.
func (b *Botanist) DeployShootSystem(ctx context.Context) error {
	_, apiResourceList, err := b.ShootClientSet.Kubernetes().Discovery().ServerGroupsAndResources()
	if err != nil {
		return fmt.Errorf("failed to discover the API: %w", err)
	}

	b.Shoot.Components.SystemComponents.Resources.SetAPIResourceList(apiResourceList)
	return b.Shoot.Components.SystemComponents.Resources.Deploy(ctx)
}

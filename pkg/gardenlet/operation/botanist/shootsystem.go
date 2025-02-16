// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"

	shootsystem "github.com/gardener/gardener/pkg/component/shoot/system"
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
		ProjectName:           b.Garden.Project.Name,
		EncryptedResources:    append(sets.List(gardenerutils.DefaultResourcesForEncryption()), b.Shoot.ResourcesToEncrypt...),
	}

	return shootsystem.New(b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, values)
}

// DeployShootSystem deploys the shoot system resources.
func (b *Botanist) DeployShootSystem(ctx context.Context) error {
	apiResourceList, err := b.ShootClientSet.Kubernetes().Discovery().ServerPreferredResources()
	if err != nil {
		groupLookupFailures, isLookupFailure := discovery.GroupDiscoveryFailedErrorGroups(err)
		if !isLookupFailure {
			return fmt.Errorf("failed to discover the API: %w", err)
		}

		b.Logger.Info("API discovery for read-only ClusterRole failed for some groups, continuing nevertheless")
		for group, failureErr := range groupLookupFailures {
			b.Logger.Info("API discovery failure", "group", group.String(), "discoveryFailureReason", failureErr.Error())
		}
	}

	b.Shoot.Components.SystemComponents.Resources.SetAPIResourceList(apiResourceList)
	b.Shoot.Components.SystemComponents.Resources.SetPodNetworkCIDRs(b.Shoot.Networks.Pods)
	b.Shoot.Components.SystemComponents.Resources.SetServiceNetworkCIDRs(b.Shoot.Networks.Services)
	b.Shoot.Components.SystemComponents.Resources.SetNodeNetworkCIDRs(b.Shoot.Networks.Nodes)
	return b.Shoot.Components.SystemComponents.Resources.Deploy(ctx)
}

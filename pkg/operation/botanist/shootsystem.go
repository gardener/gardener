// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"slices"

	"github.com/gardener/gardener/pkg/component/shootsystem"
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

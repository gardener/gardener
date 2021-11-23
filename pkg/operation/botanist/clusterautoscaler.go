// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/gardener/gardener/charts"
	"github.com/gardener/gardener/pkg/operation/botanist/component/clusterautoscaler"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultClusterAutoscaler returns a deployer for the cluster-autoscaler.
func (b *Botanist) DefaultClusterAutoscaler() (clusterautoscaler.Interface, error) {
	image, err := b.ImageVector.FindImage(charts.ImageNameClusterAutoscaler, imagevector.RuntimeVersion(b.SeedVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	return clusterautoscaler.New(
		b.K8sSeedClient.Client(),
		b.Shoot.SeedNamespace,
		image.String(),
		b.Shoot.GetReplicas(1),
		b.Shoot.GetInfo().Spec.Kubernetes.ClusterAutoscaler,
	), nil
}

// DeployClusterAutoscaler deploys the Kubernetes cluster-autoscaler.
func (b *Botanist) DeployClusterAutoscaler(ctx context.Context) error {
	if b.Shoot.WantsClusterAutoscaler {
		b.Shoot.Components.ControlPlane.ClusterAutoscaler.SetNamespaceUID(b.SeedNamespaceObject.UID)
		b.Shoot.Components.ControlPlane.ClusterAutoscaler.SetMachineDeployments(b.Shoot.Components.Extensions.Worker.MachineDeployments())

		return b.Shoot.Components.ControlPlane.ClusterAutoscaler.Deploy(ctx)
	}

	return b.Shoot.Components.ControlPlane.ClusterAutoscaler.Destroy(ctx)
}

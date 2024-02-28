// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/plutono"
	"github.com/gardener/gardener/pkg/component/shared"
)

// DefaultPlutono returns a deployer for Plutono.
func (b *Botanist) DefaultPlutono() (plutono.Interface, error) {
	return shared.NewPlutono(
		b.SeedClientSet.Client(),
		b.Shoot.SeedNamespace,
		b.SecretsManager,
		component.ClusterTypeShoot,
		b.Shoot.GetReplicas(1),
		"",
		b.ComputePlutonoHost(),
		v1beta1constants.PriorityClassNameShootControlPlane100,
		b.ShootUsesDNS(),
		b.Shoot.IsWorkerless,
		false,
		b.Shoot.NodeLocalDNSEnabled,
		b.Shoot.VPNHighAvailabilityEnabled,
		b.Shoot.WantsVerticalPodAutoscaler,
		nil,
	)
}

// DeployPlutono deploys the plutono in the Seed cluster.
func (b *Botanist) DeployPlutono(ctx context.Context) error {
	// disable plutono if no observability components are needed
	if !b.Operation.WantsObservabilityComponents() {
		return b.Shoot.Components.ControlPlane.Plutono.Destroy(ctx)
	}

	if b.ControlPlaneWildcardCert != nil {
		b.Operation.Shoot.Components.ControlPlane.Plutono.SetWildcardCertName(ptr.To(b.ControlPlaneWildcardCert.GetName()))
	}

	return b.Shoot.Components.ControlPlane.Plutono.Deploy(ctx)
}

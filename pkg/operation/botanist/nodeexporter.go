// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package botanist

import (
	"context"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component/nodeexporter"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultNodeExporter returns a deployer for the NodeExporter.
func (b *Botanist) DefaultNodeExporter() (nodeexporter.Interface, error) {
	image, err := b.ImageVector.FindImage(imagevector.ImageNameNodeExporter, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	values := nodeexporter.Values{
		Image:       image.String(),
		VPAEnabled:  b.Shoot.WantsVerticalPodAutoscaler,
		PSPDisabled: b.Shoot.PSPDisabled,
	}

	return nodeexporter.New(
		b.SeedClientSet.Client(),
		b.Shoot.SeedNamespace,
		values,
	), nil
}

// ReconcileNodeExporter deploys or destroys the node-exporter component depending on whether shoot monitoring is enabled or not.
func (b *Botanist) ReconcileNodeExporter(ctx context.Context) error {
	if !b.IsShootMonitoringEnabled() {
		return b.Shoot.Components.SystemComponents.NodeExporter.Destroy(ctx)
	}

	return b.Shoot.Components.SystemComponents.NodeExporter.Deploy(ctx)
}

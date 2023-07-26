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

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component/blackboxexporter"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultBlackboxExporter returns a deployer for the blackbox-exporter.
func (b *Botanist) DefaultBlackboxExporter() (blackboxexporter.Interface, error) {
	image, err := imagevector.ImageVector().FindImage(imagevector.ImageNameBlackboxExporter, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	return blackboxexporter.New(
		b.SeedClientSet.Client(),
		b.Shoot.SeedNamespace,
		blackboxexporter.Values{
			Image:             image.String(),
			VPAEnabled:        b.Shoot.WantsVerticalPodAutoscaler,
			KubernetesVersion: b.Shoot.KubernetesVersion,
		},
	), nil
}

// ReconcileBlackboxExporter deploys or destroys the blackbox-exporter component depending on whether shoot monitoring is enabled or not.
func (b *Botanist) ReconcileBlackboxExporter(ctx context.Context) error {
	if b.Operation.IsShootMonitoringEnabled() {
		return b.Shoot.Components.SystemComponents.BlackboxExporter.Deploy(ctx)
	}

	return b.Shoot.Components.SystemComponents.BlackboxExporter.Destroy(ctx)
}

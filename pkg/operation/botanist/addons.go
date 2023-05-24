// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/charts"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

// DeployManagedResourceForAddons deploys all the ManagedResource CRDs for the gardener-resource-manager.
func (b *Botanist) DeployManagedResourceForAddons(ctx context.Context) error {
	renderedChart, err := b.generateCoreAddonsChart()
	if err != nil {
		return fmt.Errorf("error rendering shoot-core chart: %w", err)
	}

	return managedresources.CreateForShoot(ctx, b.SeedClientSet.Client(), b.Shoot.SeedNamespace, "shoot-core", managedresources.LabelValueGardener, false, renderedChart.AsSecretData())
}

// generateCoreAddonsChart renders the gardener-resource-manager configuration for the core addons. After that it
// creates a ManagedResource CRD that references the rendered manifests and creates it.
func (b *Botanist) generateCoreAddonsChart() (*chartrenderer.RenderedChart, error) {
	var (
		global = map[string]interface{}{
			"vpaEnabled":  b.Shoot.WantsVerticalPodAutoscaler,
			"pspDisabled": b.Shoot.PSPDisabled,
			"hasWorkers":  !b.Shoot.IsWorkerless,
		}
		podSecurityPolicies = map[string]interface{}{
			"allowPrivilegedContainers": pointer.BoolDeref(b.Shoot.GetInfo().Spec.Kubernetes.AllowPrivilegedContainers, false),
		}
		nodeExporterConfig = map[string]interface{}{}
	)

	nodeExporter, err := b.InjectShootShootImages(nodeExporterConfig, images.ImageNameNodeExporter)
	if err != nil {
		return nil, err
	}

	values := map[string]interface{}{
		"global": global,
		"monitoring": common.GenerateAddonConfig(map[string]interface{}{
			"node-exporter": nodeExporter,
		}, b.Operation.IsShootMonitoringEnabled()),
		"podsecuritypolicies": common.GenerateAddonConfig(podSecurityPolicies, !b.Shoot.PSPDisabled && !b.Shoot.IsWorkerless),
	}

	return b.ShootClientSet.ChartRenderer().Render(filepath.Join(charts.Path, "shoot-core", "components"), "shoot-core", metav1.NamespaceSystem, values)
}

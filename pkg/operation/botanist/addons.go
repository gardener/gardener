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
	"embed"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/utils/managedresources"
)

var (
	//go:embed charts/shoot-core/components
	chartPSPs     embed.FS
	chartPathPSPs = filepath.Join("charts", "shoot-core", "components")
)

// DeployManagedResourceForAddons deploys all the ManagedResource CRDs for the gardener-resource-manager.
func (b *Botanist) DeployManagedResourceForAddons(ctx context.Context) error {
	values := map[string]interface{}{
		"podsecuritypolicies": map[string]interface{}{
			"enabled":                   !b.Shoot.PSPDisabled && !b.Shoot.IsWorkerless,
			"allowPrivilegedContainers": pointer.BoolDeref(b.Shoot.GetInfo().Spec.Kubernetes.AllowPrivilegedContainers, false),
		},
	}

	renderedChart, err := b.ShootClientSet.ChartRenderer().RenderEmbeddedFS(chartPSPs, chartPathPSPs, "shoot-core", metav1.NamespaceSystem, values)
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, b.SeedClientSet.Client(), b.Shoot.SeedNamespace, "shoot-core", managedresources.LabelValueGardener, false, renderedChart.AsSecretData())
}

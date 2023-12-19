// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

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

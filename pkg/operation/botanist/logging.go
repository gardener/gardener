// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"path/filepath"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/common"
)

// DeploySeedLogging will install the Helm release "seed-bootstrap/charts/loki" in the Seed clusters.
func (b *Botanist) DeploySeedLogging(ctx context.Context) error {
	if b.Shoot.GetPurpose() == gardencorev1beta1.ShootPurposeTesting || !gardenletfeatures.FeatureGate.Enabled(features.Logging) {
		return common.DeleteLoggingStack(ctx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace)
	}

	images, err := b.InjectSeedSeedImages(map[string]interface{}{},
		common.LokiImageName,
	)
	if err != nil {
		return err
	}

	lokiValues := map[string]interface{}{
		"global":   images,
		"replicas": b.Shoot.GetReplicas(1),
	}

	return b.K8sSeedClient.ChartApplier().Apply(ctx, filepath.Join(common.ChartPath, "seed-bootstrap", "charts", "loki"), b.Shoot.SeedNamespace, fmt.Sprintf("%s-logging", b.Shoot.SeedNamespace), kubernetes.Values(lokiValues))
}

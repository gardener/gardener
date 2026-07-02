// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"fmt"
	"os"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultEtcdDruid creates a new deployer for etcd-druid.
func (b *Botanist) DefaultEtcdDruid() (component.DeployWaiter, error) {
	var componentImageVectors imagevectorutils.ComponentImageVectors
	if path := os.Getenv(imagevectorutils.ComponentOverrideEnv); path != "" {
		var err error
		componentImageVectors, err = imagevectorutils.ReadComponentOverwriteFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed reading component-specific image vector override: %w", err)
		}
	}

	// See https://github.com/gardener/gardener/pull/14352
	// TODO(rfranzke): Remove this from here once `gardenadm` and the shoot gardenlet construct the Operation object in
	//  the same way.
	b.Config.ETCDConfig.FeatureGates = map[string]bool{"UpgradeEtcdVersion": true}

	return sharedcomponent.NewEtcdDruid(
		b.SeedClientSet.Client(),
		v1beta1constants.GardenNamespace,
		b.Shoot.KubernetesVersion,
		componentImageVectors,
		b.Config.ETCDConfig,
		b.SecretsManager,
		v1beta1constants.SecretNameCACluster,
		v1beta1constants.PriorityClassNameSeedSystem800,
		false,
		true,
	)
}

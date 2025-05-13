// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"os"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// DeployEtcdDruid deploys the etcd-druid component.
func (b *AutonomousBotanist) DeployEtcdDruid(ctx context.Context) error {
	var componentImageVectors imagevectorutils.ComponentImageVectors
	if path := os.Getenv(imagevectorutils.ComponentOverrideEnv); path != "" {
		var err error
		componentImageVectors, err = imagevectorutils.ReadComponentOverwriteFile(path)
		if err != nil {
			return fmt.Errorf("failed reading component-specific image vector override: %w", err)
		}
	}

	gardenletConfig := &gardenletconfigv1alpha1.GardenletConfiguration{}
	gardenletconfigv1alpha1.SetObjectDefaults_GardenletConfiguration(gardenletConfig)

	deployer, err := sharedcomponent.NewEtcdDruid(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		b.Shoot.KubernetesVersion,
		componentImageVectors,
		gardenletConfig.ETCDConfig,
		b.SecretsManager,
		v1beta1constants.SecretNameCACluster,
		v1beta1constants.PriorityClassNameSeedSystem800,
	)
	if err != nil {
		return fmt.Errorf("failed creating etcd-druid deployer: %w", err)
	}

	return deployer.Deploy(ctx)
}

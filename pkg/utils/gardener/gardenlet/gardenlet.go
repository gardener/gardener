// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenlet

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// SeedIsGarden returns 'true' if the cluster is registered as a Garden cluster.
func SeedIsGarden(ctx context.Context, seedClient client.Reader) (bool, error) {
	seedIsGarden, err := kubernetesutils.ResourcesExist(ctx, seedClient, &operatorv1alpha1.GardenList{}, operatorclient.RuntimeScheme)
	if err != nil {
		if !meta.IsNoMatchError(err) {
			return false, err
		}
		seedIsGarden = false
	}
	return seedIsGarden, nil
}

// SetDefaultGardenClusterAddress sets the default garden cluster address in the given gardenlet configuration if it is not already set.
func SetDefaultGardenClusterAddress(log logr.Logger, gardenletConfigRaw runtime.RawExtension, gardenClusterAddress string) (runtime.RawExtension, error) {
	gardenletConfig, err := encoding.DecodeGardenletConfiguration(&gardenletConfigRaw, false)
	if err != nil {
		return runtime.RawExtension{}, fmt.Errorf("failed to decode gardenlet configuration: %w", err)
	}

	if gardenletConfig == nil {
		return gardenletConfigRaw, nil
	}

	if gardenletConfig.GardenClientConnection == nil {
		gardenletConfig.GardenClientConnection = &gardenletconfigv1alpha1.GardenClientConnection{}
	}

	if gardenletConfig.GardenClientConnection.GardenClusterAddress == nil {
		log.Info("Setting default garden cluster address", "gardenClusterAddress", gardenClusterAddress)
		gardenletConfig.GardenClientConnection.GardenClusterAddress = &gardenClusterAddress
	}

	newGardenletConfigRaw, err := encoding.EncodeGardenletConfiguration(gardenletConfig)
	if err != nil {
		return runtime.RawExtension{}, fmt.Errorf("failed to encode gardenlet configuration: %w", err)
	}

	return *newGardenletConfigRaw, nil
}

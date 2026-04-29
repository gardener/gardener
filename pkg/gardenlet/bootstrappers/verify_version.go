// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrappers

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-base/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

// GetCurrentVersion returns the current version. Exposed for testing.
var GetCurrentVersion = version.Get

// VerifyGardenerVersion verifies that the operator's version is not lower and not more than one version higher than
// the version last operated on a Garden.
func VerifyGardenerVersion(ctx context.Context, log logr.Logger, reader client.Reader) error {
	configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "gardener-info", Namespace: gardencorev1beta1.GardenerSystemPublicNamespace}}
	if err := reader.Get(ctx, client.ObjectKeyFromObject(configMap), configMap); err != nil {
		return fmt.Errorf("failed reading ConfigMap %s from garden cluster: %w", client.ObjectKeyFromObject(configMap), err)
	}

	gardenerAPIServerInfo := gardenerutils.APIServerInfo{}
	if err := yaml.Unmarshal([]byte(configMap.Data[v1beta1constants.GardenerInfoConfigMapDataKeyGardenerAPIServer]), &gardenerAPIServerInfo); err != nil {
		return fmt.Errorf("failed unmarshalling the gardener-apiserver information structure: %w", err)
	}

	gardenerAPIServerVersion, err := semver.NewVersion(gardenerAPIServerInfo.Version)
	if err != nil {
		return fmt.Errorf("failed parsing version of gardener-apiserver %q: %w", gardenerAPIServerInfo.Version, err)
	}
	gardenletVersion, err := semver.NewVersion(GetCurrentVersion().GitVersion)
	if err != nil {
		return fmt.Errorf("failed parsing version of gardenlet %q: %w", GetCurrentVersion().GitVersion, err)
	}

	if gardenletVersionTooHigh, err := versionutils.CompareVersions(gardenletVersion.String(), ">", gardenerAPIServerVersion.String()); err != nil {
		return fmt.Errorf("failed comparing versions: %w", err)
	} else if gardenletVersionTooHigh {
		return fmt.Errorf("gardenlet version must not be newer than gardener-apiserver version (gardener-apiserver version: %s, gardenlet version: %s), please consult https://gardener.cloud/docs/gardener/deployment/version_skew_policy/#version-skew-policy", gardenerAPIServerVersion, gardenletVersion)
	}

	// IncMinor implicitly sets the patch version to '0'.
	if gardenletVersionTooLow, err := versionutils.CompareVersions(gardenletVersion.IncMinor().IncMinor().String(), "<", fmt.Sprintf("%d.%d.0", gardenerAPIServerVersion.Major(), gardenerAPIServerVersion.Minor())); err != nil {
		return fmt.Errorf("failed comparing versions: %w", err)
	} else if gardenletVersionTooLow {
		return fmt.Errorf("gardenlet version must not be older than two minor gardener-apiserver versions (gardener-apiserver version: %s, gardenlet version: %s), please consult https://gardener.cloud/docs/gardener/deployment/version_skew_policy/#version-skew-policy", gardenerAPIServerVersion, gardenletVersion)
	}

	log.Info("Successfully verified Gardener version skew", "gardener-apiserver", gardenerAPIServerVersion.String(), "gardenlet", gardenletVersion.String())
	return nil
}

// VerifyShootGardenletVersion verifies that the seed gardenlet's version is not higher than the shoot gardenlet's
// version (the shoot gardenlet must always be updated first), and that the seed gardenlet is at most three minor
// versions behind the shoot gardenlet.
func VerifyShootGardenletVersion(log logr.Logger, shoot *gardencorev1beta1.Shoot) error {
	shootGardenletVersion, err := semver.NewVersion(shoot.Status.Gardener.Version)
	if err != nil {
		return fmt.Errorf("failed parsing version of shoot gardenlet %q: %w", shoot.Status.Gardener.Version, err)
	}

	seedGardenletVersion, err := semver.NewVersion(GetCurrentVersion().GitVersion)
	if err != nil {
		return fmt.Errorf("failed parsing version of seed gardenlet %q: %w", GetCurrentVersion().GitVersion, err)
	}

	if seedGardenletTooHigh, err := versionutils.CompareVersions(seedGardenletVersion.String(), ">", shootGardenletVersion.String()); err != nil {
		return fmt.Errorf("failed comparing versions: %w", err)
	} else if seedGardenletTooHigh {
		return fmt.Errorf("seed gardenlet version must not be newer than shoot gardenlet version (shoot gardenlet version: %s, seed gardenlet version: %s)", shootGardenletVersion, seedGardenletVersion)
	}

	// IncMinor implicitly sets the patch version to '0'.
	if seedGardenletTooLow, err := versionutils.CompareVersions(seedGardenletVersion.IncMinor().IncMinor().IncMinor().String(), "<", fmt.Sprintf("%d.%d.0", shootGardenletVersion.Major(), shootGardenletVersion.Minor())); err != nil {
		return fmt.Errorf("failed comparing versions: %w", err)
	} else if seedGardenletTooLow {
		return fmt.Errorf("seed gardenlet version must not be more than three minor versions behind the shoot gardenlet version (shoot gardenlet version: %s, seed gardenlet version: %s)", shootGardenletVersion, seedGardenletVersion)
	}

	log.Info("Successfully verified shoot gardenlet version skew", "shootGardenlet", shootGardenletVersion.String(), "seedGardenlet", seedGardenletVersion.String())
	return nil
}

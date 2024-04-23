// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrappers

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	"k8s.io/component-base/version"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

// GetCurrentVersion returns the current version. Exposed for testing.
var GetCurrentVersion = version.Get

// VerifyGardenerVersion verifies that the operator's version is not lower and not more than one version higher than
// the version last operated on a Garden.
func VerifyGardenerVersion(ctx context.Context, log logr.Logger, client client.Reader) error {
	gardenList := &operatorv1alpha1.GardenList{}
	if err := client.List(ctx, gardenList); err != nil {
		return fmt.Errorf("failed listing Gardens: %w", err)
	}

	if length := len(gardenList.Items); length == 0 {
		return nil
	} else if length > 1 {
		return fmt.Errorf("expected at most one Garden but got %d", length)
	}

	garden := gardenList.Items[0]
	if garden.Status.Gardener == nil {
		return nil
	}

	oldGardenerVersion, err := semver.NewVersion(garden.Status.Gardener.Version)
	if err != nil {
		return fmt.Errorf("failed parsing old Garden version %q: %w", garden.Status.Gardener.Version, err)
	}
	currentGardenerVersion := GetCurrentVersion().GitVersion

	if downgrade, err := versionutils.CompareVersions(currentGardenerVersion, "<", oldGardenerVersion.String()); err != nil {
		return fmt.Errorf("failed comparing versions for downgrade check: %w", err)
	} else if downgrade {
		return fmt.Errorf("downgrading Gardener is not supported (old version was %s, my version is %s), please consult https://github.com/gardener/gardener/blob/master/docs/deployment/version_skew_policy.md", oldGardenerVersion.String(), currentGardenerVersion)
	}

	minorVersionSkew := oldGardenerVersion.IncMinor().IncMinor()
	if upgradeMoreThanOneVersion, err := versionutils.CompareVersions(currentGardenerVersion, ">=", minorVersionSkew.String()); err != nil {
		return fmt.Errorf("failed comparing versions for upgrade check: %w", err)
	} else if upgradeMoreThanOneVersion {
		return fmt.Errorf("skipping Gardener versions is unsupported (old version was %s, my version is %s), please consult https://github.com/gardener/gardener/blob/master/docs/deployment/version_skew_policy.md", oldGardenerVersion.String(), currentGardenerVersion)
	}

	log.Info("Successfully verified Gardener version skew")
	return nil
}

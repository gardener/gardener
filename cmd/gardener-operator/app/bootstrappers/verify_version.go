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

package bootstrappers

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver"
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

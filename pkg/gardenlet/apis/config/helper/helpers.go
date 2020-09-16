// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
)

// SeedNameFromSeedConfig returns an empty string if the given seed config is nil, or the
// name inside the seed config.
func SeedNameFromSeedConfig(seedConfig *config.SeedConfig) string {
	if seedConfig == nil {
		return ""
	}
	return seedConfig.Seed.Name
}

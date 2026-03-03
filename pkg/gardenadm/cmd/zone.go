// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"slices"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// DetermineZone determines the effective zone for the node based on the shoot specification.
func DetermineZone(worker gardencorev1beta1.Worker, providedZone string) (string, error) {
	switch len(worker.Zones) {
	case 0:
		if providedZone != "" {
			return "", fmt.Errorf("worker %q has no zones configured, but zone %q was provided", worker.Name, providedZone)
		}
		return "", nil

	case 1:
		if providedZone == "" {
			return worker.Zones[0], nil
		}
		if providedZone != worker.Zones[0] {
			return "", fmt.Errorf("provided zone %q does not match the configured zones %v for worker %q", providedZone, worker.Zones, worker.Name)
		}
		return providedZone, nil

	default:
		if providedZone == "" {
			return "", fmt.Errorf("worker %q has multiple zones configured %v, --zone flag is required", worker.Name, worker.Zones)
		}
		if !slices.Contains(worker.Zones, providedZone) {
			return "", fmt.Errorf("provided zone %q does not match the configured zones %v for worker %q", providedZone, worker.Zones, worker.Name)
		}
		return providedZone, nil
	}
}

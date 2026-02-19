// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"slices"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// ValidateZone validates the provided zone against the zones configured for the given worker pool.
func ValidateZone(worker gardencorev1beta1.Worker, providedZone string) (string, error) {
	workerZones := worker.Zones

	switch len(workerZones) {
	case 0:
		if providedZone != "" {
			return "", fmt.Errorf("worker %q has no zones configured, but zone %q was provided", worker.Name, providedZone)
		}
		return "", nil

	case 1:
		if providedZone == "" {
			return workerZones[0], nil
		}
		if providedZone != workerZones[0] {
			return "", fmt.Errorf("provided zone %q does not match the configured zones %v for worker %q", providedZone, workerZones, worker.Name)
		}
		return providedZone, nil

	default:
		if providedZone == "" {
			return "", fmt.Errorf("worker %q has multiple zones configured %v, --zone flag is required", worker.Name, workerZones)
		}
		if !slices.Contains(workerZones, providedZone) {
			return "", fmt.Errorf("provided zone %q does not match the configured zones %v for worker %q", providedZone, workerZones, worker.Name)
		}
		return providedZone, nil
	}
}

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"slices"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// ValidateAndDetermineControlPlaneZone validates the provided zone against the Shoot's control plane worker pool and returns the
// effective zone. For managed-infrastructure Shoots the zone must be empty. Otherwise, the zone is validated and
// auto-applied via DetermineZone.
func ValidateAndDetermineControlPlaneZone(shoot *gardencorev1beta1.Shoot, providedZone string) (string, error) {
	if shoot == nil {
		return "", fmt.Errorf("zone validation failed, shoot resource is missing in the manifests")
	}

	if v1beta1helper.HasManagedInfrastructure(shoot) {
		if providedZone != "" {
			return "", fmt.Errorf("zone can't be configured for shoot with managed infrastructure")
		}
		return "", nil
	}

	// This command is only for control plane nodes, therefore we look for the control plane pool.
	controlPlanePool := v1beta1helper.ControlPlaneWorkerPoolForShoot(shoot.Spec.Provider.Workers)
	if controlPlanePool == nil {
		return "", fmt.Errorf("zone validation failed, shoot doesn't have a control plane worker pool configured")
	}

	effectiveZone, err := DetermineZone(*controlPlanePool, providedZone)
	if err != nil {
		return "", fmt.Errorf("failed determining zone for control plane worker pool %q: %w", controlPlanePool.Name, err)
	}

	return effectiveZone, nil
}

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

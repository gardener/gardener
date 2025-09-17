// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	api "github.com/gardener/gardener/pkg/provider-local/apis/local"
	"github.com/gardener/gardener/pkg/provider-local/apis/local/helper"
)

// UpdateMachineImagesStatus implements genericactuator.WorkerDelegate.
func (w *workerDelegate) UpdateMachineImagesStatus(ctx context.Context) error {
	if w.machineImages == nil {
		if err := w.generateMachineConfig(ctx); err != nil {
			return fmt.Errorf("unable to generate the machine config: %w", err)
		}
	}

	// Decode the current worker provider status.
	workerStatus, err := w.decodeWorkerProviderStatus()
	if err != nil {
		return fmt.Errorf("unable to decode the worker provider status: %w", err)
	}

	workerStatus.MachineImages = w.machineImages
	if err := w.updateWorkerProviderStatus(ctx, workerStatus); err != nil {
		return fmt.Errorf("unable to update worker provider status: %w", err)
	}

	return nil
}

func (w *workerDelegate) selectMachineImageForWorkerPool(name, version string, machineCapabilities gardencorev1beta1.Capabilities) (*api.MachineImage, error) {
	selectedMachineImage := &api.MachineImage{
		Name:    name,
		Version: version,
	}

	if image, err := helper.FindImageFromCloudProfile(w.cloudProfileConfig, name, version, machineCapabilities, w.cluster.CloudProfile.Spec.MachineCapabilities); err == nil {
		selectedMachineImage.Image = image.Image
		selectedMachineImage.Capabilities = image.Capabilities
		return selectedMachineImage, nil
	}

	// Try to look up machine image in worker provider status as it was not found in CloudProfile.
	if providerStatus := w.worker.Status.ProviderStatus; providerStatus != nil {
		workerStatus := &api.WorkerStatus{}
		if _, _, err := w.decoder.Decode(providerStatus.Raw, nil, workerStatus); err != nil {
			return nil, fmt.Errorf("could not decode worker status of worker '%s': %w", client.ObjectKeyFromObject(w.worker), err)
		}

		for _, machineImage := range workerStatus.MachineImages {
			if machineImage.Name == name && machineImage.Version == version {
				// If no capabilityDefinitions are specified, return the (legacy) image field as no image capabilityFlavors are used.
				if len(w.cluster.CloudProfile.Spec.MachineCapabilities) == 0 {
					selectedMachineImage.Image = machineImage.Image
					selectedMachineImage.Capabilities = gardencorev1beta1.Capabilities{}
					return selectedMachineImage, nil
				}

				if machineImage.Name == name && machineImage.Version == version && v1beta1helper.AreCapabilitiesCompatible(machineImage.Capabilities, machineCapabilities, w.cluster.CloudProfile.Spec.MachineCapabilities) {
					return &machineImage, nil
				}
			}
		}
	}

	return nil, worker.ErrorMachineImageNotFound(name, version)
}

func appendMachineImage(machineImages []api.MachineImage, machineImage api.MachineImage, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition) []api.MachineImage {
	// support for cloudprofile machine images without capabilities
	if len(capabilityDefinitions) == 0 {
		for _, image := range machineImages {
			if image.Name == machineImage.Name && image.Version == machineImage.Version {
				// If the image already exists without capabilities, we can just return the existing list.
				return machineImages
			}
		}
		return append(machineImages, api.MachineImage{
			Name:    machineImage.Name,
			Version: machineImage.Version,
			Image:   machineImage.Image,
		})
	}

	defaultedCapabilities := v1beta1helper.GetCapabilitiesWithAppliedDefaults(machineImage.Capabilities, capabilityDefinitions)

	for _, existingMachineImage := range machineImages {
		existingDefaultedCapabilities := v1beta1helper.GetCapabilitiesWithAppliedDefaults(existingMachineImage.Capabilities, capabilityDefinitions)
		if existingMachineImage.Name == machineImage.Name &&
			existingMachineImage.Version == machineImage.Version &&
			v1beta1helper.AreCapabilitiesEqual(defaultedCapabilities, existingDefaultedCapabilities) {
			// If the image already exists with the same capabilities return the existing list.
			return machineImages
		}
	}

	// If the image does not exist, we create a new machine image entry with the capabilities.
	machineImages = append(machineImages, api.MachineImage{
		Name:         machineImage.Name,
		Version:      machineImage.Version,
		Image:        machineImage.Image,
		Capabilities: machineImage.Capabilities,
	})

	return machineImages
}

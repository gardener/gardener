// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	api "github.com/gardener/gardener/pkg/provider-local/apis/local"
	"github.com/gardener/gardener/pkg/provider-local/apis/local/helper"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
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

type machineImageWithCapabilities struct {
	// Name is the logical name of the machine image.
	Name string
	// Version is the logical version of the machine image.
	Version string
	// Image is the image for the machine image.
	Image string
	// Capabilities that are supported by the machine image.
	Capabilities core.Capabilities
}

func (w *workerDelegate) selectMachineImageForWorkerPool(name, version string, machineCapabilities gardencorev1beta1.Capabilities) (*machineImageWithCapabilities, error) {
	capabilitySet, err := helper.FindImageFromCloudProfile(w.cloudProfileConfig, name, version, machineCapabilities, w.cluster.CloudProfile.Spec.Capabilities)
	returnMachineImage := &machineImageWithCapabilities{
		Name:         name,
		Version:      version,
		Image:        "",
		Capabilities: core.Capabilities{},
	}

	if err == nil {
		returnMachineImage.Image = capabilitySet.Image
		returnMachineImage.Capabilities = capabilitySet.Capabilities
		return returnMachineImage, nil
	}

	// Try to look up machine image in worker provider status as it was not found in CloudProfile.
	if providerStatus := w.worker.Status.ProviderStatus; providerStatus != nil {
		workerStatus := &api.WorkerStatus{}
		if _, _, err := w.decoder.Decode(providerStatus.Raw, nil, workerStatus); err != nil {
			return nil, fmt.Errorf("could not decode worker status of worker '%s': %w", client.ObjectKeyFromObject(w.worker), err)
		}

		for _, machineImage := range workerStatus.MachineImages {
			if machineImage.Name == name && machineImage.Version == version {
				// If no capabilitiesDefinitions are specified, return the (legacy) image field as no capabilitySets are used.
				if len(w.cluster.CloudProfile.Spec.Capabilities) == 0 {
					returnMachineImage.Image = machineImage.Image
					return returnMachineImage, nil
				}

				bestMatch, err := helper.FindBestCapabilitySet(machineImage.CapabilitySets, machineCapabilities, w.cluster.CloudProfile.Spec.Capabilities)
				if err != nil {
					return nil, fmt.Errorf("no machine image found for image %q with version %q and capabilities %v: %w", name, version, machineCapabilities, err)
				}

				returnMachineImage.Image = bestMatch.Image
				returnMachineImage.Capabilities = bestMatch.Capabilities
				return returnMachineImage, nil
			}
		}
	}

	return nil, worker.ErrorMachineImageNotFound(name, version)
}

func appendMachineImage(machineImages []api.MachineImage, machineImage machineImageWithCapabilities, capabilitiesDefinitions []core.CapabilityDefinition) []api.MachineImage {
	// support for cloudprofile machine images without capabilities
	if len(capabilitiesDefinitions) == 0 {
		for _, image := range machineImages {
			if image.Name == machineImage.Name && image.Version == machineImage.Version {
				// If the image already exists without capability sets, we can just return the existing list.
				return machineImages
			}
		}
		return append(machineImages, api.MachineImage{
			Name:           machineImage.Name,
			Version:        machineImage.Version,
			Image:          machineImage.Image,
			CapabilitySets: []api.CapabilitySet{},
		})
	}

	defaultedCapabilities := gardencorehelper.GetCapabilitiesWithAppliedDefaults(machineImage.Capabilities, capabilitiesDefinitions)

	imageExists := false
	for i, existingMachineImage := range machineImages {
		if existingMachineImage.Name == machineImage.Name && existingMachineImage.Version == machineImage.Version {
			imageExists = true
			capabilitiesExist := false
			for _, existingCapabilitySet := range existingMachineImage.CapabilitySets {
				existingDefaultedCapabilities := gardencorehelper.GetCapabilitiesWithAppliedDefaults(existingCapabilitySet.Capabilities, capabilitiesDefinitions)
				if gardenerutils.AreCapabilitiesEqual(defaultedCapabilities, existingDefaultedCapabilities) {
					capabilitiesExist = true
				}
			}
			if !capabilitiesExist {
				machineImages[i].CapabilitySets = append(machineImages[i].CapabilitySets, api.CapabilitySet{Image: machineImage.Image, Capabilities: machineImage.Capabilities})
			}
			break // No need to continue iterating once the image is found
		}
	}

	if !imageExists {
		machineImages = append(machineImages, api.MachineImage{
			Name:    machineImage.Name,
			Version: machineImage.Version,
			Image:   machineImage.Image,
			CapabilitySets: []api.CapabilitySet{
				{
					Image:        machineImage.Image,
					Capabilities: machineImage.Capabilities,
				},
			},
		})
	}

	return machineImages
}

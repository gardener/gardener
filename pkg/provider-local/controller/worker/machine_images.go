// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package worker

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	api "github.com/gardener/gardener/pkg/provider-local/apis/local"
	"github.com/gardener/gardener/pkg/provider-local/apis/local/helper"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// UpdateMachineImagesStatus implements genericactuator.WorkerDelegate.
func (w *workerDelegate) UpdateMachineImagesStatus(ctx context.Context) error {
	if w.machineImages == nil {
		if err := w.generateMachineConfig(); err != nil {
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

func (w *workerDelegate) findMachineImage(name, version string) (string, error) {
	ami, err := helper.FindImageFromCloudProfile(w.cloudProfileConfig, name, version)
	if err == nil {
		return ami, nil
	}

	// Try to look up machine image in worker provider status as it was not found in CloudProfile.
	if providerStatus := w.worker.Status.ProviderStatus; providerStatus != nil {
		workerStatus := &api.WorkerStatus{}
		if _, _, err := w.Decoder().Decode(providerStatus.Raw, nil, workerStatus); err != nil {
			return "", fmt.Errorf("could not decode worker status of worker '%s': %w", kubernetesutils.ObjectName(w.worker), err)
		}

		machineImage, err := helper.FindMachineImage(workerStatus.MachineImages, name, version)
		if err != nil {
			return "", worker.ErrorMachineImageNotFound(name, version)
		}

		return machineImage.Image, nil
	}

	return "", worker.ErrorMachineImageNotFound(name, version)
}

func appendMachineImage(machineImages []api.MachineImage, machineImage api.MachineImage) []api.MachineImage {
	if _, err := helper.FindMachineImage(machineImages, machineImage.Name, machineImage.Version); err != nil {
		return append(machineImages, machineImage)
	}
	return machineImages
}

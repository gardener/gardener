// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeinit

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
)

func getBootstrapConfiguration(worker gardencorev1beta1.Worker) (*nodeagentconfigv1alpha1.BootstrapConfiguration, error) {
	bootstrapConfiguration := &nodeagentconfigv1alpha1.BootstrapConfiguration{}

	var err error
	bootstrapConfiguration.KubeletDataVolumeSize, err = getKubeletDataVolumeSize(worker)
	if err != nil {
		return nil, fmt.Errorf("failed getting kubelet data volume size: %w", err)
	}

	return bootstrapConfiguration, nil
}

func getKubeletDataVolumeSize(worker gardencorev1beta1.Worker) (*int64, error) {
	if len(worker.DataVolumes) == 0 || worker.KubeletDataVolumeName == nil {
		return nil, nil
	}

	for _, dv := range worker.DataVolumes {
		if dv.Name != *worker.KubeletDataVolumeName {
			continue
		}

		parsed, err := resource.ParseQuantity(dv.VolumeSize)
		if err != nil {
			return nil, fmt.Errorf("failed parsing kubelet data volume size %q as quantity: %w", dv.VolumeSize, err)
		}

		if sizeInBytes, ok := parsed.AsInt64(); ok {
			return &sizeInBytes, nil
		}
	}

	return nil, fmt.Errorf("failed finding data volume for kubelet in worker with name %q", *worker.KubeletDataVolumeName)
}

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

package nodeinit

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
)

func getBootstrapConfiguration(worker gardencorev1beta1.Worker) (*nodeagentv1alpha1.BootstrapConfiguration, error) {
	bootstrapConfiguration := &nodeagentv1alpha1.BootstrapConfiguration{}

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

// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shared

import (
	"github.com/Masterminds/semver"
	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/logging/fluentoperator"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

// NewFluentOperatorCustomResources instantiates a new `Fluent Operator Custom Resources` component.
func NewFluentOperatorCustomResources(
	c client.Client,
	gardenNamespaceName string,
	runtimeVersion *semver.Version,
	imageVector imagevector.ImageVector,
	enabled bool,
	priorityClassName string,
	additionalInputs []*fluentbitv1alpha2.ClusterInput,
	additionalFilters []*fluentbitv1alpha2.ClusterFilter,
	additionalParsers []*fluentbitv1alpha2.ClusterParser,
) (
	deployer component.DeployWaiter,
	err error,
) {
	fluentBitImage, err := imageVector.FindImage(images.ImageNameFluentBit)
	if err != nil {
		return nil, err
	}

	fluentBitInitImage, err := imageVector.FindImage(images.ImageNameFluentBitPluginInstaller)
	if err != nil {
		return nil, err
	}

	deployer = fluentoperator.NewCustomResources(
		c,
		gardenNamespaceName,
		fluentoperator.CustomResourcesValues{
			FluentBit: fluentoperator.FluentBit{
				Image:              fluentBitImage.String(),
				InitContainerImage: fluentBitInitImage.String(),
				PriorityClass:      priorityClassName,
			},
		},
		additionalInputs,
		additionalFilters,
		additionalParsers,
	)

	if !enabled {
		deployer = component.OpDestroyAndWait(deployer)
	}

	return deployer, nil
}

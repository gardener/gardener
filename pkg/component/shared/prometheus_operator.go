// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheusoperator"
)

// NewPrometheusOperator instantiates a new prometheus-operator component.
func NewPrometheusOperator(
	c client.Client,
	gardenNamespaceName string,
	priorityClassName string,
) (
	deployer component.DeployWaiter,
	err error,
) {
	operatorImage, err := imagevector.ImageVector().FindImage(imagevector.ImageNamePrometheusOperator)
	if err != nil {
		return nil, err
	}

	reloaderImage, err := imagevector.ImageVector().FindImage(imagevector.ImageNameConfigmapReloader)
	if err != nil {
		return nil, err
	}

	return prometheusoperator.New(
		c,
		gardenNamespaceName,
		prometheusoperator.Values{
			Image:               operatorImage.String(),
			ImageConfigReloader: reloaderImage.String(),
			PriorityClassName:   priorityClassName,
		},
	), nil
}

// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"github.com/gardener/gardener/pkg/component/hvpa"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// NewHVPA instantiates a new `hvpa-controller` component.
func NewHVPA(
	c client.Client,
	gardenNamespaceName string,
	imageVector imagevectorutils.ImageVector,
	enabled bool,
	priorityClassName string,
) (
	deployer component.DeployWaiter,
	err error,
) {
	image, err := imageVector.FindImage(imagevector.ImageNameHvpaController)
	if err != nil {
		return nil, err
	}

	deployer = hvpa.New(c, gardenNamespaceName, hvpa.Values{
		Image:             image.String(),
		PriorityClassName: priorityClassName,
	})

	if !enabled {
		deployer = component.OpDestroyWithoutWait(deployer)
	}

	return deployer, nil
}

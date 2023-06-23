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

package shared

import (
	"github.com/Masterminds/semver"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/nginxingress"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

// NewNginxIngress returns a deployer for nginx-ingress-controller.
func NewNginxIngress(
	c client.Client,
	imageVector imagevector.ImageVector,
	kubernetesVersion *semver.Version,
	ingressClass string,
	config map[string]string,
	loadBalancerAnnotations map[string]string,
	gardenNamespaceName string,
	priorityClassName string,
) (
	component.DeployWaiter,
	error,
) {
	imageController, err := imageVector.FindImage(images.ImageNameNginxIngressControllerSeed, imagevector.TargetVersion(kubernetesVersion.String()))
	if err != nil {
		return nil, err
	}
	imageDefaultBackend, err := imageVector.FindImage(images.ImageNameIngressDefaultBackend, imagevector.TargetVersion(kubernetesVersion.String()))
	if err != nil {
		return nil, err
	}

	values := nginxingress.Values{
		ImageController:         imageController.String(),
		ImageDefaultBackend:     imageDefaultBackend.String(),
		IngressClass:            ingressClass,
		ConfigData:              config,
		LoadBalancerAnnotations: loadBalancerAnnotations,
		PriorityClassName:       priorityClassName,
	}

	return nginxingress.New(c, gardenNamespaceName, values), nil
}

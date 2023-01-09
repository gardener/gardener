// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"context"
	"fmt"

	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/operation/botanist/addons/nginxingress"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultNginxIngress returns a deployer for the nginxingress.
func (b *Botanist) DefaultNginxIngress() (
	component.DeployWaiter,
	error,
) {
	imageController, err := b.ImageVector.FindImage(images.ImageNameNginxIngressController, imagevector.RuntimeVersion(b.ShootVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}
	imageDefaultBackend, err := b.ImageVector.FindImage(images.ImageNameIngressDefaultBackend, imagevector.RuntimeVersion(b.ShootVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	values := nginxingress.Values{
		ImageController:          imageController.String(),
		ImageDefaultBackend:      imageDefaultBackend.String(),
		KubernetesVersion:        b.Shoot.KubernetesVersion,
		ConfigData:               b.Shoot.GetInfo().Spec.Addons.NginxIngress.Config,
		LoadBalancerSourceRanges: b.Shoot.GetInfo().Spec.Addons.NginxIngress.LoadBalancerSourceRanges,
		ExternalTrafficPolicy:    *b.Shoot.GetInfo().Spec.Addons.NginxIngress.ExternalTrafficPolicy,
		VPAEnabled:               b.Shoot.WantsVerticalPodAutoscaler,
	}

	if b.APIServerSNIEnabled() {
		values.KubeAPIServerHost = b.outOfClusterAPIServerFQDN()
	}

	return nginxingress.New(b.SeedClientSet.Client(), b.Shoot.SeedNamespace, values), nil
}

// DeployNginxIngressAddon deploys the NginxIngress Addon component.
func (b *Botanist) DeployNginxIngressAddon(ctx context.Context) error {
	if !gardencorev1beta1helper.NginxIngressEnabled(b.Shoot.GetInfo().Spec.Addons) {
		return b.Shoot.Components.Addons.NginxIngress.Destroy(ctx)
	}

	return b.Shoot.Components.Addons.NginxIngress.Deploy(ctx)
}

// outOfClusterAPIServerFQDN returns the Fully Qualified Domain Name of the apiserver
// with dot "." suffix. It'll prevent extra requests to the DNS in case the record is not
// available.
func (b *Botanist) outOfClusterAPIServerFQDN() string {
	return fmt.Sprintf("%s.", b.Shoot.ComputeOutOfClusterAPIServerAddress(b.APIServerAddress, true))
}

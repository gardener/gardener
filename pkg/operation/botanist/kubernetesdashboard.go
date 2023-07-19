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

package botanist

import (
	"context"

	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/imagevector"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/component/kubernetesdashboard"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultKubernetesDashboard returns a deployer for kubernetes-dashboard.
func (b *Botanist) DefaultKubernetesDashboard() (kubernetesdashboard.Interface, error) {
	image, err := b.ImageVector.FindImage(imagevector.ImageNameKubernetesDashboard, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	scraperImage, err := b.ImageVector.FindImage(imagevector.ImageNameKubernetesDashboardMetricsScraper, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	values := kubernetesdashboard.Values{
		Image:               image.String(),
		MetricsScraperImage: scraperImage.String(),
		VPAEnabled:          b.Shoot.WantsVerticalPodAutoscaler,
		KubernetesVersion:   b.Shoot.KubernetesVersion,
	}

	if b.ShootUsesDNS() {
		values.APIServerHost = pointer.String(b.outOfClusterAPIServerFQDN())
	}

	if b.Shoot.GetInfo().Spec.Addons.KubernetesDashboard.AuthenticationMode != nil {
		values.AuthenticationMode = *b.Shoot.GetInfo().Spec.Addons.KubernetesDashboard.AuthenticationMode
	}

	return kubernetesdashboard.New(b.SeedClientSet.Client(), b.Shoot.SeedNamespace, values), nil
}

// DeployKubernetesDashboard deploys the Kubernetes Dashboard component.
func (b *Botanist) DeployKubernetesDashboard(ctx context.Context) error {
	if !v1beta1helper.KubernetesDashboardEnabled(b.Shoot.GetInfo().Spec.Addons) {
		return b.Shoot.Components.Addons.KubernetesDashboard.Destroy(ctx)
	}

	return b.Shoot.Components.Addons.KubernetesDashboard.Deploy(ctx)
}

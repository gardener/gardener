// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/imagevector"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	kubernetesdashboard "github.com/gardener/gardener/pkg/component/kubernetes/dashboard"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultKubernetesDashboard returns a deployer for kubernetes-dashboard.
func (b *Botanist) DefaultKubernetesDashboard() (kubernetesdashboard.Interface, error) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameKubernetesDashboard, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	scraperImage, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameKubernetesDashboardMetricsScraper, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	values := kubernetesdashboard.Values{
		Image:               image.String(),
		MetricsScraperImage: scraperImage.String(),
		VPAEnabled:          b.Shoot.WantsVerticalPodAutoscaler,
	}

	if b.ShootUsesDNS() {
		values.APIServerHost = ptr.To(b.outOfClusterAPIServerFQDN())
	}

	if b.Shoot.GetInfo().Spec.Addons.KubernetesDashboard.AuthenticationMode != nil {
		values.AuthenticationMode = *b.Shoot.GetInfo().Spec.Addons.KubernetesDashboard.AuthenticationMode
	}

	return kubernetesdashboard.New(b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, values), nil
}

// DeployKubernetesDashboard deploys the Kubernetes Dashboard component.
func (b *Botanist) DeployKubernetesDashboard(ctx context.Context) error {
	if !v1beta1helper.KubernetesDashboardEnabled(b.Shoot.GetInfo().Spec.Addons) {
		return b.Shoot.Components.Addons.KubernetesDashboard.Destroy(ctx)
	}

	return b.Shoot.Components.Addons.KubernetesDashboard.Deploy(ctx)
}

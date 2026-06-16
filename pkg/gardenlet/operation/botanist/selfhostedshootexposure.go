// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	extensionsselfhostedshootexposure "github.com/gardener/gardener/pkg/component/extensions/selfhostedshootexposure"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// DefaultSelfHostedShootExposure creates the default deployer for the SelfHostedShootExposure resource.
// The actual endpoints are populated at deploy time by the caller via Values.Endpoints.
func (b *Botanist) DefaultSelfHostedShootExposure() *extensionsselfhostedshootexposure.SelfHostedShootExposure {
	var (
		shoot  = b.Shoot.GetInfo()
		pool   = v1beta1helper.ControlPlaneWorkerPoolForShoot(shoot.Spec.Provider.Workers)
		values = &extensionsselfhostedshootexposure.Values{
			Name:      shoot.Name,
			Namespace: b.Shoot.ControlPlaneNamespace,
			Type:      *pool.ControlPlane.Exposure.Extension.Type,
		}
	)

	// For unmanaged infrastructure no CredentialsRef is set; the provider must handle nil.
	// For managed infrastructure DeployCloudProviderSecret always materializes the cloudprovider Secret
	// in the control plane namespace (also when the shoot references a WorkloadIdentity), so referencing
	// that Secret here works uniformly.
	if v1beta1helper.HasManagedInfrastructure(shoot) {
		values.CredentialsRef = &corev1.ObjectReference{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
			Name:       v1beta1constants.SecretNameCloudProvider,
			Namespace:  b.Shoot.ControlPlaneNamespace,
		}
	}

	return extensionsselfhostedshootexposure.New(
		b.Logger,
		b.SeedClientSet.Client(),
		values,
	)
}

// DeploySelfHostedShootExposure populates the SelfHostedShootExposure spec with the addresses of all control-plane
// nodes and deploys the resource. It waits for the provisioning to be complete, i.e. `.status.ingress` to be populated.
func (b *Botanist) DeploySelfHostedShootExposure(ctx context.Context) error {
	nodes, err := b.ListControlPlaneNodes(ctx)
	if err != nil {
		return fmt.Errorf("failed listing control plane nodes: %w", err)
	}

	endpoints, err := gardenerutils.ControlPlaneEndpointsFromNodes(nodes)
	if err != nil {
		return err
	}

	b.Shoot.Components.Extensions.SelfHostedShootExposure.Values.Endpoints = endpoints

	return component.OpWait(b.Shoot.Components.Extensions.SelfHostedShootExposure).Deploy(ctx)
}

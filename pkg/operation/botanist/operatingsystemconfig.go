// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/extensions/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/operation/botanist/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/gardener/gardener/pkg/utils/secrets"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DefaultOperatingSystemConfig creates the default deployer for the OperatingSystemConfig custom resource.
func (b *Botanist) DefaultOperatingSystemConfig(seedClient client.Client) (operatingsystemconfig.Interface, error) {
	images, err := imagevector.FindImages(b.ImageVector, []string{common.HyperkubeImageName, common.PauseContainerImageName}, imagevector.RuntimeVersion(b.ShootVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	clusterDNSAddress := b.Shoot.Networks.CoreDNS.String()
	if b.Shoot.NodeLocalDNSEnabled && b.Shoot.IPVSEnabled() {
		// If IPVS is enabled then instruct the kubelet to create pods resolving DNS to the `nodelocaldns` network
		// interface link-local ip address. For more information checkout the usage documentation under
		// https://kubernetes.io/docs/tasks/administer-cluster/nodelocaldns/.
		clusterDNSAddress = NodeLocalIPVSAddress
	}

	return operatingsystemconfig.New(
		b.Logger,
		seedClient,
		&operatingsystemconfig.Values{
			Namespace:         b.Shoot.SeedNamespace,
			KubernetesVersion: b.Shoot.KubernetesVersion,
			Workers:           b.Shoot.Info.Spec.Provider.Workers,
			DownloaderValues: operatingsystemconfig.DownloaderValues{
				APIServerURL: fmt.Sprintf("https://%s", b.Shoot.ComputeOutOfClusterAPIServerAddress(b.APIServerAddress, true)),
			},
			OriginalValues: operatingsystemconfig.OriginalValues{
				ClusterDNSAddress:       clusterDNSAddress,
				ClusterDomain:           gardencorev1beta1.DefaultDomain,
				Images:                  images,
				KubeletConfigParameters: components.KubeletConfigParametersFromCoreV1beta1KubeletConfig(b.Shoot.Info.Spec.Kubernetes.Kubelet),
				KubeletCLIFlags:         components.KubeletCLIFlagsFromCoreV1beta1KubeletConfig(b.Shoot.Info.Spec.Kubernetes.Kubelet),
				MachineTypes:            b.Shoot.CloudProfile.Spec.MachineTypes,
			},
		},
		operatingsystemconfig.DefaultInterval,
		operatingsystemconfig.DefaultSevereThreshold,
		operatingsystemconfig.DefaultTimeout,
	), nil
}

// DeployOperatingSystemConfig deploys the OperatingSystemConfig custom resource and triggers the restore operation in
// case the Shoot is in the restore phase of the control plane migration.
func (b *Botanist) DeployOperatingSystemConfig(ctx context.Context) error {
	b.Shoot.Components.Extensions.OperatingSystemConfig.SetCABundle(b.getOperatingSystemConfigCABundle())
	b.Shoot.Components.Extensions.OperatingSystemConfig.SetKubeletCACertificate(string(b.Secrets[v1beta1constants.SecretNameCAKubelet].Data[secrets.DataKeyCertificateCA]))
	b.Shoot.Components.Extensions.OperatingSystemConfig.SetSSHPublicKey(string(b.Secrets[v1beta1constants.SecretNameSSHKeyPair].Data[secrets.DataKeySSHAuthorizedKeys]))

	if b.isRestorePhase() {
		return b.Shoot.Components.Extensions.OperatingSystemConfig.Restore(ctx, b.ShootState)
	}

	return b.Shoot.Components.Extensions.OperatingSystemConfig.Deploy(ctx)
}

func (b *Botanist) getOperatingSystemConfigCABundle() *string {
	var caBundle string

	if cloudProfileCaBundle := b.Shoot.CloudProfile.Spec.CABundle; cloudProfileCaBundle != nil {
		caBundle = *cloudProfileCaBundle
	}

	if caCert, ok := b.Secrets[v1beta1constants.SecretNameCACluster].Data[secrets.DataKeyCertificateCA]; ok && len(caCert) != 0 {
		caBundle = fmt.Sprintf("%s\n%s", caBundle, caCert)
	}

	if caBundle == "" {
		return nil
	}
	return &caBundle
}

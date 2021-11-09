// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/gardener/gardener/charts"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubescheduler"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultKubeScheduler returns a deployer for the kube-scheduler.
func (b *Botanist) DefaultKubeScheduler() (kubescheduler.Interface, error) {
	image, err := b.ImageVector.FindImage(charts.ImageNameKubeScheduler, imagevector.RuntimeVersion(b.SeedVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	return kubescheduler.New(
		b.K8sSeedClient.Client(),
		b.Shoot.SeedNamespace,
		b.Shoot.KubernetesVersion,
		image.String(),
		b.Shoot.GetReplicas(1),
		b.Shoot.GetInfo().Spec.Kubernetes.KubeScheduler,
	), nil
}

// DeployKubeScheduler deploys the Kubernetes scheduler.
func (b *Botanist) DeployKubeScheduler(ctx context.Context) error {
	b.Shoot.Components.ControlPlane.KubeScheduler.SetSecrets(kubescheduler.Secrets{
		Server: component.Secret{Name: kubescheduler.SecretNameServer, Checksum: b.LoadCheckSum(kubescheduler.SecretNameServer)},
	})

	return b.Shoot.Components.ControlPlane.KubeScheduler.Deploy(ctx)
}

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

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeproxy"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"

	"k8s.io/apimachinery/pkg/runtime"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
)

// DefaultKubeProxy returns a deployer for the kube-proxy.
func (b *Botanist) DefaultKubeProxy() (kubeproxy.Interface, error) {
	return kubeproxy.New(
		b.K8sSeedClient.Client(),
		b.Shoot.SeedNamespace,
		kubeproxy.Values{},
	), nil
}

// DeployKubeProxy deploys the kube-proxy.
func (b *Botanist) DeployKubeProxy(ctx context.Context) error {
	kubeconfig, err := runtime.Encode(clientcmdlatest.Codec, kutil.NewKubeconfig(
		b.Shoot.SeedNamespace,
		b.Shoot.ComputeOutOfClusterAPIServerAddress(b.APIServerAddress, true),
		b.LoadSecret(v1beta1constants.SecretNameCACluster).Data[secrets.DataKeyCertificateCA],
		clientcmdv1.AuthInfo{TokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token"},
	))
	if err != nil {
		return err
	}

	// TODO In a subsequent commit: Move computation of WorkerPools from addons.go to this place.

	b.Shoot.Components.SystemComponents.KubeProxy.SetKubeconfig(kubeconfig)

	return b.Shoot.Components.SystemComponents.KubeProxy.Deploy(ctx)
}

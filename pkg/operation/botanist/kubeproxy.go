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
	"fmt"

	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/imagevector"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/component/kubeproxy"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"
)

// DefaultKubeProxy returns a deployer for the kube-proxy.
func (b *Botanist) DefaultKubeProxy() (kubeproxy.Interface, error) {
	imageAlpine, err := imagevector.ImageVector().FindImage(imagevector.ImageNameAlpine, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	var featureGates map[string]bool
	if kubeProxyConfig := b.Shoot.GetInfo().Spec.Kubernetes.KubeProxy; kubeProxyConfig != nil {
		featureGates = kubeProxyConfig.FeatureGates
	}

	return kubeproxy.New(
		b.SeedClientSet.Client(),
		b.Shoot.SeedNamespace,
		kubeproxy.Values{
			IPVSEnabled:    b.Shoot.IPVSEnabled(),
			FeatureGates:   featureGates,
			ImageAlpine:    imageAlpine.String(),
			PodNetworkCIDR: pointer.String(b.Shoot.Networks.Pods.String()),
			VPAEnabled:     b.Shoot.WantsVerticalPodAutoscaler,
			PSPDisabled:    b.Shoot.PSPDisabled,
		},
	), nil
}

// DeployKubeProxy deploys the kube-proxy.
func (b *Botanist) DeployKubeProxy(ctx context.Context) error {
	caSecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
	}

	kubeconfig, err := runtime.Encode(clientcmdlatest.Codec, kubernetesutils.NewKubeconfig(
		b.Shoot.SeedNamespace,
		clientcmdv1.Cluster{
			Server:                   b.Shoot.ComputeOutOfClusterAPIServerAddress(b.APIServerAddress, true),
			CertificateAuthorityData: caSecret.Data[secrets.DataKeyCertificateBundle],
		},
		clientcmdv1.AuthInfo{TokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token"},
	))
	if err != nil {
		return err
	}

	workerPools, err := b.computeWorkerPoolsForKubeProxy(ctx)
	if err != nil {
		return err
	}

	b.Shoot.Components.SystemComponents.KubeProxy.SetKubeconfig(kubeconfig)
	b.Shoot.Components.SystemComponents.KubeProxy.SetWorkerPools(workerPools)

	return b.Shoot.Components.SystemComponents.KubeProxy.Deploy(ctx)
}

func (b *Botanist) computeWorkerPoolsForKubeProxy(ctx context.Context) ([]kubeproxy.WorkerPool, error) {
	poolKeyToPoolInfo := make(map[string]kubeproxy.WorkerPool)

	for _, worker := range b.Shoot.GetInfo().Spec.Provider.Workers {
		kubernetesVersion, err := v1beta1helper.CalculateEffectiveKubernetesVersion(b.Shoot.KubernetesVersion, worker.Kubernetes)
		if err != nil {
			return nil, err
		}

		image, err := imagevector.ImageVector().FindImage(imagevector.ImageNameKubeProxy, imagevectorutils.RuntimeVersion(kubernetesVersion.String()), imagevectorutils.TargetVersion(kubernetesVersion.String()))
		if err != nil {
			return nil, err
		}

		key := workerPoolKey(worker.Name, kubernetesVersion.String())
		poolKeyToPoolInfo[key] = kubeproxy.WorkerPool{
			Name:              worker.Name,
			KubernetesVersion: kubernetesVersion,
			Image:             image.String(),
		}
	}

	nodeList := &corev1.NodeList{}
	if err := b.ShootClientSet.Client().List(ctx, nodeList); err != nil {
		return nil, err
	}

	for _, node := range nodeList.Items {
		poolName, ok1 := node.Labels[v1beta1constants.LabelWorkerPool]
		kubernetesVersionString, ok2 := node.Labels[v1beta1constants.LabelWorkerKubernetesVersion]
		if !ok1 || !ok2 {
			continue
		}
		kubernetesVersion, err := semver.NewVersion(kubernetesVersionString)
		if err != nil {
			return nil, err
		}

		image, err := imagevector.ImageVector().FindImage(imagevector.ImageNameKubeProxy, imagevectorutils.RuntimeVersion(kubernetesVersionString), imagevectorutils.TargetVersion(kubernetesVersionString))
		if err != nil {
			return nil, err
		}

		key := workerPoolKey(poolName, kubernetesVersionString)
		poolKeyToPoolInfo[key] = kubeproxy.WorkerPool{
			Name:              poolName,
			KubernetesVersion: kubernetesVersion,
			Image:             image.String(),
		}
	}

	var workerPools []kubeproxy.WorkerPool
	for _, poolInfo := range poolKeyToPoolInfo {
		workerPools = append(workerPools, poolInfo)
	}

	return workerPools, nil
}

func workerPoolKey(poolName, kubernetesVersion string) string {
	return poolName + "@" + kubernetesVersion
}

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

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/controlplane/kubeapiserver/sni"
)

// DefaultKubeAPIServerSNI returns a deployer for kube-apiserver SNI.
func (b *Botanist) DefaultKubeAPIServerSNI() component.DeployWaiter {
	return component.OpDestroy(sni.NewKubeAPIServerSNI(
		&sni.KubeAPIServerSNIValues{
			Name: v1beta1constants.DeploymentNameKubeAPIServer,
			IstioIngressGateway: sni.IstioIngressGateway{
				Namespace: *b.Config.SNI.Ingress.Namespace,
				Labels:    b.Config.SNI.Ingress.Labels,
			},
		},
		b.Shoot.SeedNamespace,
		b.K8sSeedClient.ChartApplier(),
		b.ChartsRootPath,
	))
}

// DeployKubeAPIServerSNI deploys the kube-apiserver-sni chart.
func (b *Botanist) DeployKubeAPIServerSNI(ctx context.Context) error {
	return b.Shoot.Components.ControlPlane.KubeAPIServerSNI.Deploy(ctx)
}

// SNIPhase returns the current phase of the SNI enablement of kube-apiserver's service.
func (b *Botanist) SNIPhase(ctx context.Context) (component.Phase, error) {
	var (
		svc        = &corev1.Service{}
		sniEnabled = b.APIServerSNIEnabled()
	)

	if err := b.K8sSeedClient.DirectClient().Get(
		ctx,
		client.ObjectKey{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: b.Shoot.SeedNamespace},
		svc,
	); client.IgnoreNotFound(err) != nil {
		return component.PhaseUnknown, err
	}

	switch {
	case svc.Spec.Type == corev1.ServiceTypeLoadBalancer && sniEnabled:
		return component.PhaseEnabling, nil
	case svc.Spec.Type == corev1.ServiceTypeClusterIP && sniEnabled:
		return component.PhaseEnabled, nil
	case svc.Spec.Type == corev1.ServiceTypeClusterIP && !sniEnabled:
		return component.PhaseDisabling, nil
	default:
		if sniEnabled {
			// initial cluster creation with SNI enabled (enabling only relevant for migration).
			return component.PhaseEnabled, nil
		}
		// initial cluster creation with SNI disabled.
		return component.PhaseDisabled, nil
	}
}

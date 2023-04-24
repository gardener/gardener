// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserverexposure"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

func (b *Botanist) newKubeAPIServiceServiceComponent(sniPhase component.Phase) component.DeployWaiter {
	return kubeapiserverexposure.NewService(
		b.Logger,
		b.SeedClientSet.Client(),
		&kubeapiserverexposure.ServiceValues{
			AnnotationsFunc:             func() map[string]string { return b.IstioLoadBalancerAnnotations() },
			SNIPhase:                    sniPhase,
			TopologyAwareRoutingEnabled: b.Shoot.TopologyAwareRoutingEnabled,
		},
		func() client.ObjectKey {
			return client.ObjectKey{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: b.Shoot.SeedNamespace}
		},
		func() client.ObjectKey {
			return client.ObjectKey{Name: b.IstioServiceName(), Namespace: b.IstioNamespace()}
		},
		nil,
		b.setAPIServerServiceClusterIP,
		func(address string) {
			b.APIServerAddress = address
			b.newDNSComponentsTargetingAPIServerAddress()
		},
		features.DefaultFeatureGate.Enabled(features.FullNetworkPoliciesInRuntimeCluster),
		"",
	)
}

// DefaultKubeAPIServerService returns a deployer for the kube-apiserver service.
func (b *Botanist) DefaultKubeAPIServerService(sniPhase component.Phase) component.DeployWaiter {
	return b.newKubeAPIServiceServiceComponent(sniPhase)
}

// DeployKubeAPIService deploys the kube-apiserver service.
func (b *Botanist) DeployKubeAPIService(ctx context.Context, sniPhase component.Phase) error {
	return b.newKubeAPIServiceServiceComponent(sniPhase).Deploy(ctx)
}

// APIServerSNIEnabled returns true if APIServerSNI feature gate is enabled and the shoot uses internal and external
// DNS.
func (b *Botanist) APIServerSNIEnabled() bool {
	return features.DefaultFeatureGate.Enabled(features.APIServerSNI) && b.NeedsInternalDNS() && b.NeedsExternalDNS()
}

// DefaultKubeAPIServerSNI returns a deployer for the kube-apiserver SNI.
func (b *Botanist) DefaultKubeAPIServerSNI() component.DeployWaiter {
	return component.OpDestroyWithoutWait(kubeapiserverexposure.NewSNI(
		b.SeedClientSet.Client(),
		b.SeedClientSet.Applier(),
		b.Shoot.SeedNamespace,
		func() *kubeapiserverexposure.SNIValues {
			return &kubeapiserverexposure.SNIValues{
				IstioIngressGateway: kubeapiserverexposure.IstioIngressGateway{
					Namespace: b.IstioNamespace(),
					Labels:    b.IstioLabels(),
				},
				APIServerInternalDNSName: b.outOfClusterAPIServerFQDN(),
			}
		},
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

	if err := b.SeedClientSet.APIReader().Get(
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

func (b *Botanist) setAPIServerServiceClusterIP(clusterIP string) {
	if b.Shoot.Components.ControlPlane.KubeAPIServerSNIPhase == component.PhaseDisabled {
		return
	}

	b.APIServerClusterIP = clusterIP
	b.Shoot.Components.ControlPlane.KubeAPIServerSNI = kubeapiserverexposure.NewSNI(
		b.SeedClientSet.Client(),
		b.SeedClientSet.Applier(),
		b.Shoot.SeedNamespace,
		func() *kubeapiserverexposure.SNIValues {
			return &kubeapiserverexposure.SNIValues{
				APIServerClusterIP: clusterIP,
				NamespaceUID:       b.SeedNamespaceObject.UID,
				Hosts: []string{
					gardenerutils.GetAPIServerDomain(*b.Shoot.ExternalClusterDomain),
					gardenerutils.GetAPIServerDomain(b.Shoot.InternalClusterDomain),
				},
				IstioIngressGateway: kubeapiserverexposure.IstioIngressGateway{
					Namespace: b.IstioNamespace(),
					Labels:    b.IstioLabels(),
				},
				APIServerInternalDNSName: b.outOfClusterAPIServerFQDN(),
			}
		},
	)
}

// DefaultKubeAPIServerIngress returns a deployer for the kube-apiserver ingress.
func (b *Botanist) DefaultKubeAPIServerIngress() (component.Deployer, error) {
	ingressClass, err := gardenerutils.ComputeNginxIngressClassForSeed(b.Seed.GetInfo(), b.Seed.GetInfo().Status.KubernetesVersion)
	if err != nil {
		return nil, err
	}

	return kubeapiserverexposure.NewIngress(
		b.SeedClientSet.Client(),
		b.Shoot.SeedNamespace,
		kubeapiserverexposure.IngressValues{
			ServiceName:      v1beta1constants.DeploymentNameKubeAPIServer,
			Host:             b.ComputeKubeAPIServerHost(),
			IngressClassName: &ingressClass,
		}), nil
}

// DeployKubeAPIServerIngress deploys the ingress for the kube-apiserver.
func (b *Botanist) DeployKubeAPIServerIngress(ctx context.Context) error {
	// Do not deploy ingress if there is no wildcard certificate
	if b.ControlPlaneWildcardCert == nil {
		return b.Shoot.Components.ControlPlane.KubeAPIServerIngress.Destroy(ctx)
	}
	return b.Shoot.Components.ControlPlane.KubeAPIServerIngress.Deploy(ctx)
}

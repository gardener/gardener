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

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	kubeapiserverexposure "github.com/gardener/gardener/pkg/component/kubernetes/apiserverexposure"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// DefaultKubeAPIServerService returns a deployer for the kube-apiserver service.
func (b *Botanist) DefaultKubeAPIServerService() component.DeployWaiter {
	return kubeapiserverexposure.NewService(
		b.Logger,
		b.SeedClientSet.Client(),
		b.Shoot.SeedNamespace,
		&kubeapiserverexposure.ServiceValues{
			AnnotationsFunc:             func() map[string]string { return b.IstioLoadBalancerAnnotations() },
			TopologyAwareRoutingEnabled: b.Shoot.TopologyAwareRoutingEnabled,
			RuntimeKubernetesVersion:    b.Seed.KubernetesVersion,
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
		"",
	)
}

// ShootUsesDNS returns true if the shoot uses internal and external DNS.
func (b *Botanist) ShootUsesDNS() bool {
	return b.NeedsInternalDNS() && b.NeedsExternalDNS()
}

// DefaultKubeAPIServerSNI returns a deployer for the kube-apiserver SNI.
func (b *Botanist) DefaultKubeAPIServerSNI() component.DeployWaiter {
	return component.OpDestroyWithoutWait(kubeapiserverexposure.NewSNI(
		b.SeedClientSet.Client(),
		v1beta1constants.DeploymentNameKubeAPIServer,
		b.Shoot.SeedNamespace,
		func() *kubeapiserverexposure.SNIValues {
			return &kubeapiserverexposure.SNIValues{
				IstioIngressGateway: kubeapiserverexposure.IstioIngressGateway{
					Namespace: b.IstioNamespace(),
					Labels:    b.IstioLabels(),
				},
			}
		},
	))
}

// DeployKubeAPIServerSNI deploys the kube-apiserver SNI resources.
func (b *Botanist) DeployKubeAPIServerSNI(ctx context.Context) error {
	return b.Shoot.Components.ControlPlane.KubeAPIServerSNI.Deploy(ctx)
}

func (b *Botanist) setAPIServerServiceClusterIP(clusterIP string) {
	if b.Shoot.Networks.Services.IP.To4() != nil {
		b.APIServerClusterIP = clusterIP
	} else {
		// "64:ff9b:1::" is a well known prefix for address translation for use
		// in local networks.
		b.APIServerClusterIP = "64:ff9b:1::" + clusterIP
	}
	b.Shoot.Components.ControlPlane.KubeAPIServerSNI = kubeapiserverexposure.NewSNI(
		b.SeedClientSet.Client(),
		v1beta1constants.DeploymentNameKubeAPIServer,
		b.Shoot.SeedNamespace,
		func() *kubeapiserverexposure.SNIValues {
			return &kubeapiserverexposure.SNIValues{
				Hosts: []string{
					gardenerutils.GetAPIServerDomain(*b.Shoot.ExternalClusterDomain),
					gardenerutils.GetAPIServerDomain(b.Shoot.InternalClusterDomain),
				},
				APIServerProxy: &kubeapiserverexposure.APIServerProxy{
					APIServerClusterIP: b.APIServerClusterIP,
					NamespaceUID:       b.SeedNamespaceObject.UID,
				},
				IstioIngressGateway: kubeapiserverexposure.IstioIngressGateway{
					Namespace: b.IstioNamespace(),
					Labels:    b.IstioLabels(),
				},
			}
		},
	)
}

// DefaultKubeAPIServerIngress returns a deployer for the kube-apiserver ingress.
func (b *Botanist) DefaultKubeAPIServerIngress() component.Deployer {
	return kubeapiserverexposure.NewIngress(
		b.SeedClientSet.Client(),
		b.Shoot.SeedNamespace,
		kubeapiserverexposure.IngressValues{
			ServiceName: v1beta1constants.DeploymentNameKubeAPIServer,
			Host:        b.ComputeKubeAPIServerHost(),
			IstioIngressGatewayLabelsFunc: func() map[string]string {
				return b.DefaultIstioLabels()
			},
			IstioIngressGatewayNamespaceFunc: func() string {
				return b.DefaultIstioNamespace()
			},
		})
}

// DeployKubeAPIServerIngress deploys the ingress for the kube-apiserver.
func (b *Botanist) DeployKubeAPIServerIngress(ctx context.Context) error {
	// Do not deploy ingress if there is no wildcard certificate
	if b.ControlPlaneWildcardCert == nil {
		return b.Shoot.Components.ControlPlane.KubeAPIServerIngress.Destroy(ctx)
	}
	return b.Shoot.Components.ControlPlane.KubeAPIServerIngress.Deploy(ctx)
}

// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"net"

	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/component"
	kubeapiserverexposure "github.com/gardener/gardener/pkg/component/kubernetes/apiserverexposure"
	"github.com/gardener/gardener/pkg/features"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// DefaultKubeAPIServerService returns a deployer for the kube-apiserver service.
func (b *Botanist) DefaultKubeAPIServerService() component.DeployWaiter {
	deployer := []component.Deployer{
		b.defaultKubeAPIServerServiceWithSuffix("", true),
	}
	mutualTLSService := b.defaultKubeAPIServerServiceWithSuffix(kubeapiserverexposure.MutualTLSServiceNameSuffix, false)
	if features.DefaultFeatureGate.Enabled(features.IstioTLSTermination) && v1beta1helper.IsShootIstioTLSTerminationEnabled(b.Shoot.GetInfo()) {
		deployer = append(deployer, mutualTLSService)
	} else {
		deployer = append(deployer, component.OpDestroy(mutualTLSService))
	}
	return component.OpWait(deployer...)
}

func (b *Botanist) defaultKubeAPIServerServiceWithSuffix(suffix string, register bool) component.DeployWaiter {
	clusterIPsFunc := b.setAPIServerServiceClusterIPs
	ingressFunc := func(address string) {
		b.APIServerAddress = address
		b.newDNSComponentsTargetingAPIServerAddress()
	}
	if !register {
		clusterIPsFunc = nil
		ingressFunc = nil
	}

	return kubeapiserverexposure.NewService(
		b.Logger,
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		&kubeapiserverexposure.ServiceValues{
			AnnotationsFunc:             func() map[string]string { return b.IstioLoadBalancerAnnotations() },
			TopologyAwareRoutingEnabled: b.Shoot.TopologyAwareRoutingEnabled,
			RuntimeKubernetesVersion:    b.Seed.KubernetesVersion,
			NameSuffix:                  suffix,
		},
		func() client.ObjectKey {
			return client.ObjectKey{Name: b.IstioServiceName(), Namespace: b.IstioNamespace()}
		},
		nil,
		clusterIPsFunc,
		ingressFunc,
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
		b.Shoot.ControlPlaneNamespace,
		b.SecretsManager,
		func() *kubeapiserverexposure.SNIValues {
			var wildcardHost *string

			if b.ControlPlaneWildcardCert != nil {
				wildcardHost = ptr.To(b.ComputeKubeAPIServerHost())
			}

			return &kubeapiserverexposure.SNIValues{
				IstioIngressGateway: kubeapiserverexposure.IstioIngressGateway{
					Namespace: b.IstioNamespace(),
					Labels:    b.IstioLabels(),
				},
				IstioTLSTermination: features.DefaultFeatureGate.Enabled(features.IstioTLSTermination) && v1beta1helper.IsShootIstioTLSTerminationEnabled(b.Shoot.GetInfo()),
				WildcardHost:        wildcardHost,
				WildcardTLSSecret:   b.ControlPlaneWildcardCert,
			}
		},
	))
}

// DeployKubeAPIServerSNI deploys the kube-apiserver SNI resources.
func (b *Botanist) DeployKubeAPIServerSNI(ctx context.Context) error {
	return b.Shoot.Components.ControlPlane.KubeAPIServerSNI.Deploy(ctx)
}

func (b *Botanist) setAPIServerServiceClusterIPs(clusterIPs []string) {
	clusterIPv4 := net.ParseIP(clusterIPs[0]).To4()

	if clusterIPv4 != nil {
		if b.Shoot.GetInfo().Spec.Networking.IPFamilies[0] == gardencorev1beta1.IPFamilyIPv6 {
			// "64:ff9b:1::" is a well known prefix for address translation for use
			// in local networks.
			b.APIServerClusterIP = "64:ff9b:1::" + clusterIPs[0]
		} else {
			b.APIServerClusterIP = mapToReservedKubeApiServerRange(clusterIPv4)
		}
	} else {
		if gardencorev1beta1.IsIPv4SingleStack(b.Shoot.GetInfo().Spec.Networking.IPFamilies) && len(clusterIPs) > 1 {
			clusterIPv4 = net.ParseIP(clusterIPs[1]).To4()
			b.APIServerClusterIP = mapToReservedKubeApiServerRange(clusterIPv4)
		} else {
			// regular ipv6 cluster ip
			b.APIServerClusterIP = clusterIPs[0]
		}
	}
	b.Shoot.Components.ControlPlane.KubeAPIServerSNI = kubeapiserverexposure.NewSNI(
		b.SeedClientSet.Client(),
		v1beta1constants.DeploymentNameKubeAPIServer,
		b.Shoot.ControlPlaneNamespace,
		b.SecretsManager,
		func() *kubeapiserverexposure.SNIValues {
			var wildcardHost *string

			if b.ControlPlaneWildcardCert != nil {
				wildcardHost = ptr.To(b.ComputeKubeAPIServerHost())
			}

			values := &kubeapiserverexposure.SNIValues{
				Hosts: []string{
					gardenerutils.GetAPIServerDomain(*b.Shoot.ExternalClusterDomain),
					gardenerutils.GetAPIServerDomain(b.Shoot.InternalClusterDomain),
				},
				APIServerProxy: &kubeapiserverexposure.APIServerProxy{
					APIServerClusterIP: b.APIServerClusterIP,
				},
				IstioIngressGateway: kubeapiserverexposure.IstioIngressGateway{
					Namespace: b.IstioNamespace(),
					Labels:    b.IstioLabels(),
				},
				IstioTLSTermination: features.DefaultFeatureGate.Enabled(features.IstioTLSTermination) && v1beta1helper.IsShootIstioTLSTerminationEnabled(b.Shoot.GetInfo()),
				WildcardHost:        wildcardHost,
				WildcardTLSSecret:   b.ControlPlaneWildcardCert,
			}

			if features.DefaultFeatureGate.Enabled(features.RemoveAPIServerProxyLegacyPort) {
				values.APIServerProxy = nil
			}

			return values
		},
	)
}

func mapToReservedKubeApiServerRange(ip net.IP) string {
	// prevent leakage of real cluster ip to shoot. we use the reserved range 240.0.0.0/8 as prefix instead.
	// e.g. cluster ip in seed:  192.168.102.23 => ip in shoot:  240.168.102.23
	prefixIp, _, _ := net.ParseCIDR(v1beta1constants.ReservedKubeApiServerMappingRange)
	prefix := prefixIp.To4()
	return net.IPv4(prefix[0], ip[1], ip[2], ip[3]).String()
}

// DefaultKubeAPIServerIngress returns a deployer for the kube-apiserver ingress.
// TODO(oliver-goetz): Remove this method when Gardener v1.115.0 is released.
func (b *Botanist) DefaultKubeAPIServerIngress() component.Deployer {
	return kubeapiserverexposure.NewIngress(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
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
// TODO(oliver-goetz): Remove this method when Gardener v1.115.0 is released.
func (b *Botanist) DeployKubeAPIServerIngress(ctx context.Context) error {
	// This is now part of the SNI deployer in kubeapiserverexposure.
	return b.Shoot.Components.ControlPlane.KubeAPIServerIngress.Destroy(ctx)
}

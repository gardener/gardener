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

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/controlplane/kubeapiserver/service"
	"github.com/gardener/gardener/pkg/operation/botanist/controlplane/kubeapiserver/sni"
	"github.com/gardener/gardener/pkg/operation/botanist/extensions/dns"
	"github.com/gardener/gardener/pkg/operation/common"
)

// DefaultKubeAPIServerService returns a deployer for kube-apiserver service.
func (b *Botanist) DefaultKubeAPIServerService(sniPhase component.Phase) component.DeployWaiter {
	return b.kubeAPIService(sniPhase)
}

// DeployKubeAPIService deploys for kube-apiserver service.
func (b *Botanist) DeployKubeAPIService(ctx context.Context, sniPhase component.Phase) error {
	return b.kubeAPIService(sniPhase).Deploy(ctx)
}

func (b *Botanist) kubeAPIService(sniPhase component.Phase) component.DeployWaiter {
	return service.NewKubeAPIService(
		&service.KubeAPIServiceValues{
			Annotations:               b.Seed.LoadBalancerServiceAnnotations,
			KonnectivityTunnelEnabled: b.Shoot.KonnectivityTunnelEnabled,
			SNIPhase:                  sniPhase,
		},
		client.ObjectKey{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: b.Shoot.SeedNamespace},
		client.ObjectKey{Name: *b.Config.SNI.Ingress.ServiceName, Namespace: *b.Config.SNI.Ingress.Namespace},
		b.K8sSeedClient.ChartApplier(),
		b.ChartsRootPath,
		b.Logger,
		b.K8sSeedClient.DirectClient(),
		nil,
		b.setAPIServerServiceClusterIP,
		func(address string) { b.setAPIServerAddress(address, b.K8sSeedClient.DirectClient()) },
	)
}

func (b *Botanist) setAPIServerServiceClusterIP(clusterIP string) {
	if b.Shoot.Components.ControlPlane.KubeAPIServerSNIPhase == component.PhaseDisabled {
		return
	}

	b.APIServerClusterIP = clusterIP

	b.Shoot.Components.ControlPlane.KubeAPIServerSNI = sni.NewKubeAPIServerSNI(
		&sni.KubeAPIServerSNIValues{
			ApiserverClusterIP: clusterIP,
			NamespaceUID:       b.SeedNamespaceObject.UID,
			Hosts: []string{
				common.GetAPIServerDomain(*b.Shoot.ExternalClusterDomain),
				common.GetAPIServerDomain(b.Shoot.InternalClusterDomain),
			},
			Name: v1beta1constants.DeploymentNameKubeAPIServer,
			IstioIngressGateway: sni.IstioIngressGateway{
				Namespace: *b.Config.SNI.Ingress.Namespace,
				Labels:    b.Config.SNI.Ingress.Labels,
			},
		},
		b.Shoot.SeedNamespace,
		b.K8sSeedClient.ChartApplier(),
		b.ChartsRootPath,
	)
}

// setAPIServerAddress sets the IP address of the API server's LoadBalancer.
func (b *Botanist) setAPIServerAddress(address string, seedClient client.Client) {
	b.Operation.APIServerAddress = address

	if b.NeedsInternalDNS() {
		ownerID := *b.Shoot.Info.Status.ClusterIdentity + "-" + DNSInternalName
		b.Shoot.Components.Extensions.DNS.InternalOwner = dns.NewDNSOwner(
			&dns.OwnerValues{
				Name:    DNSInternalName,
				Active:  true,
				OwnerID: ownerID,
			},
			b.Shoot.SeedNamespace,
			b.K8sSeedClient.ChartApplier(),
			b.ChartsRootPath,
			seedClient,
		)
		b.Shoot.Components.Extensions.DNS.InternalEntry = dns.NewDNSEntry(
			&dns.EntryValues{
				Name:    DNSInternalName,
				DNSName: common.GetAPIServerDomain(b.Shoot.InternalClusterDomain),
				Targets: []string{b.APIServerAddress},
				OwnerID: ownerID,
				TTL:     *b.Config.Controllers.Shoot.DNSEntryTTLSeconds,
			},
			b.Shoot.SeedNamespace,
			b.K8sSeedClient.ChartApplier(),
			b.ChartsRootPath,
			b.Logger,
			seedClient,
			nil,
		)
	}

	if b.NeedsExternalDNS() {
		ownerID := *b.Shoot.Info.Status.ClusterIdentity + "-" + DNSExternalName
		b.Shoot.Components.Extensions.DNS.ExternalOwner = dns.NewDNSOwner(
			&dns.OwnerValues{
				Name:    DNSExternalName,
				Active:  true,
				OwnerID: ownerID,
			},
			b.Shoot.SeedNamespace,
			b.K8sSeedClient.ChartApplier(),
			b.ChartsRootPath,
			seedClient,
		)
		b.Shoot.Components.Extensions.DNS.ExternalEntry = dns.NewDNSEntry(
			&dns.EntryValues{
				Name:    DNSExternalName,
				DNSName: common.GetAPIServerDomain(*b.Shoot.ExternalClusterDomain),
				Targets: []string{b.APIServerAddress},
				OwnerID: ownerID,
				TTL:     *b.Config.Controllers.Shoot.DNSEntryTTLSeconds,
			},
			b.Shoot.SeedNamespace,
			b.K8sSeedClient.ChartApplier(),
			b.ChartsRootPath,
			b.Logger,
			seedClient,
			nil,
		)
	}
}

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
	"strconv"

	"github.com/gardener/gardener/charts"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component/nodelocaldns"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultNodeLocalDNS returns a deployer for the node-local-dns.
func (b *Botanist) DefaultNodeLocalDNS() (nodelocaldns.Interface, error) {
	image, err := b.ImageVector.FindImage(charts.ImageNameNodeLocalDns, imagevector.RuntimeVersion(b.ShootVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	// The node-local-dns interface cannot bind the kube-dns cluster IP since the interface
	// used for IPVS load-balancing already uses this address.
	clusterDNS := "__PILLAR__CLUSTER__DNS__"
	dnsServer := ""
	if b.Shoot.IPVSEnabled() {
		clusterDNS = b.Shoot.Networks.CoreDNS.String()
	} else {
		dnsServer = b.Shoot.Networks.CoreDNS.String()
	}

	nodeLocalDNSForceTcpToClusterDNS := true
	if forceTcp, err := strconv.ParseBool(b.Shoot.GetInfo().Annotations[v1beta1constants.AnnotationNodeLocalDNSForceTcpToClusterDns]); err == nil {
		nodeLocalDNSForceTcpToClusterDNS = forceTcp
	}

	nodeLocalDNSForceTcpToUpstreamDNS := true
	if forceTcp, err := strconv.ParseBool(b.Shoot.GetInfo().Annotations[v1beta1constants.AnnotationNodeLocalDNSForceTcpToUpstreamDns]); err == nil {
		nodeLocalDNSForceTcpToUpstreamDNS = forceTcp
	}

	return nodelocaldns.New(
		b.K8sSeedClient.Client(),
		b.Shoot.SeedNamespace,
		nodelocaldns.Values{
			Image:                 image.String(),
			VPAEnabled:            b.Shoot.WantsVerticalPodAutoscaler,
			ForceTcpToClusterDNS:  nodeLocalDNSForceTcpToClusterDNS,
			ForceTcpToUpstreamDNS: nodeLocalDNSForceTcpToUpstreamDNS,
			ClusterDNS:            clusterDNS,
			DNSServer:             dnsServer,
		},
	), nil
}

// ReconcileNodeLocalDNS deploys or destroys the node-local-dns component depending on whether it is enabled for the Shoot.
func (b *Botanist) ReconcileNodeLocalDNS(ctx context.Context) error {
	if b.Shoot.NodeLocalDNSEnabled {
		return b.Shoot.Components.SystemComponents.NodeLocalDNS.Deploy(ctx)
	}
	return b.Shoot.Components.SystemComponents.NodeLocalDNS.Destroy(ctx)
}

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
	"github.com/gardener/gardener/charts"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/operation/botanist/component/coredns"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultCoreDNS returns a deployer for the CoreDNS.
func (b *Botanist) DefaultCoreDNS() (coredns.Interface, error) {
	image, err := b.ImageVector.FindImage(charts.ImageNameCoredns, imagevector.RuntimeVersion(b.ShootVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	return coredns.New(
		b.K8sSeedClient.Client(),
		b.Shoot.SeedNamespace,
		coredns.Values{
			ClusterDomain: gardencorev1beta1.DefaultDomain, // resolve conformance test issue (https://github.com/kubernetes/kubernetes/blob/master/test/e2e/network/dns.go#L44) before changing:
			ClusterIP:     b.Shoot.Networks.CoreDNS.String(),
			Image:         image.String(),
		},
	), nil
}

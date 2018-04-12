// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package awsbotanist

import (
	"path/filepath"

	"github.com/gardener/gardener/pkg/operation/common"
)

// ApplyCreateHook updates the AWS ELB health check to SSL and deploys the aws-lb-readvertiser.
// https://github.com/gardener/aws-lb-readvertiser
func (b *AWSBotanist) ApplyCreateHook() error {
	var (
		name          = "aws-lb-readvertiser"
		defaultValues = map[string]interface{}{
			"domain": b.APIServerAddress,
		}
	)

	values, err := b.InjectImages(defaultValues, b.K8sSeedClient.Version(), map[string]string{name: name})
	if err != nil {
		return err
	}

	if err := b.ApplyChartSeed(filepath.Join(common.ChartPath, "seed-controlplane", "charts", name), name, b.Shoot.SeedNamespace, nil, values); err != nil {
		return err
	}

	// Change ELB health check to SSL (avoid TLS handshake errors because of AWS ELB health checks)
	loadBalancerName := b.APIServerAddress[:32]
	elb, err := b.AWSClient.GetELB(loadBalancerName)
	if err != nil {
		return err
	}
	targetPort := (*elb.LoadBalancerDescriptions[0].HealthCheck.Target)[4:]
	return b.AWSClient.UpdateELBHealthCheck(loadBalancerName, targetPort)
}

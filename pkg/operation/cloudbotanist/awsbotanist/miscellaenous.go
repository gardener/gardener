// Copyright 2018 The Gardener Authors.
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
	"fmt"
	"path/filepath"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/gardener/gardener/pkg/operation/common"
)

// ApplyCreateHook updates the AWS ELB health check to SSL and deploys the readvertiser.
// https://github.com/gardener/aws-lb-readvertiser
func (b *AWSBotanist) ApplyCreateHook() error {
	imagePullSecrets := b.GetImagePullSecretsMap()

	defaultValues := map[string]interface{}{
		"domain":           b.APIServerAddress,
		"imagePullSecrets": imagePullSecrets,
	}

	values, err := b.InjectImages(defaultValues, b.K8sSeedClient.Version(), map[string]string{"readvertiser": "aws-lb-readvertiser"})
	if err != nil {
		return err
	}

	if err := b.ApplyChartSeed(filepath.Join(common.ChartPath, "seed-controlplane", "charts", "readvertiser"), "readvertiser", b.Shoot.SeedNamespace, nil, values); err != nil {
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

// ApplyDeleteHook does currently nothing for AWS.
func (b *AWSBotanist) ApplyDeleteHook() error {
	return nil
}

// CheckIfClusterGetsScaled checks whether the Shoot cluster gets currently scaled.
// It returns a boolean value and the number of instances which are healthy from the perspective
// of AWS (those which have the InService state).
func (b *AWSBotanist) CheckIfClusterGetsScaled() (bool, int, error) {
	var (
		currentlyScaling = false
		healthyInstances = 0
	)

	groupList := []*string{}
	for i := range b.Shoot.Info.Spec.Cloud.AWS.Zones {
		for _, worker := range b.Shoot.Info.Spec.Cloud.AWS.Workers {
			groupList = append(groupList, awssdk.String(fmt.Sprintf("%s-nodes-%s-z%d", b.Shoot.SeedNamespace, worker.Name, i)))
		}
	}
	groups, err := b.AWSClient.GetAutoScalingGroups(groupList)
	if err != nil {
		return false, 0, err
	}

	for _, group := range groups.AutoScalingGroups {
		desired := group.DesiredCapacity
		instances := group.Instances
		if *desired != int64(len(instances)) {
			return true, 0, nil
		}
		for _, instance := range instances {
			if *instance.LifecycleState != "InService" {
				currentlyScaling = true
			} else {
				healthyInstances++
			}
		}
	}
	return currentlyScaling, healthyInstances, nil
}

// GetASGs returns the set of AutoScalingGroups used for a Shoot cluster.
func (b *AWSBotanist) GetASGs() []map[string]interface{} {
	var (
		autoscalingGroups = []map[string]interface{}{}
		zones             = b.Shoot.Info.Spec.Cloud.AWS.Zones
		zoneCount         = len(zones)
	)

	for i := range zones {
		for _, worker := range b.Shoot.Info.Spec.Cloud.AWS.Workers {
			autoscalingGroups = append(autoscalingGroups, map[string]interface{}{
				"name":    fmt.Sprintf("%s-nodes-%s-z%d", b.Shoot.SeedNamespace, worker.Name, i),
				"minSize": common.DistributeOverZones(i, worker.AutoScalerMin, zoneCount),
				"maxSize": common.DistributeOverZones(i, worker.AutoScalerMax, zoneCount),
			})
		}
	}

	return autoscalingGroups
}

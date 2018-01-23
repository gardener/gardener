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
	"strings"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/terraformer"
	"github.com/gardener/gardener/pkg/utils"
)

// DeployKube2IAMResources creates the respective IAM roles which have been specified in the Shoot manifest
// addon section. Moreover, some default IAM roles will be created.
func (b *AWSBotanist) DeployKube2IAMResources() error {
	values, err := b.generateTerraformKube2IAMConfig(b.Shoot.Info.Spec.Addons.Kube2IAM.Roles)
	if err != nil {
		return err
	}

	return terraformer.
		New(b.Operation, common.TerraformerPurposeKube2IAM).
		SetVariablesEnvironment(b.generateTerraformInfraVariablesEnvironment()).
		DefineConfig("aws-kube2iam", values).
		Apply()
}

// DestroyKube2IAMResources destroy the kube2iam resources created by Terraform. This comprises IAM roles and
// policies.
func (b *AWSBotanist) DestroyKube2IAMResources() error {
	return terraformer.
		New(b.Operation, common.TerraformerPurposeKube2IAM).
		SetVariablesEnvironment(b.generateTerraformInfraVariablesEnvironment()).
		Destroy()
}

// generateTerraformKube2IAMConfig creates the Terraform variables and the Terraform config (for kube2iam)
// and returns them (these values will be stored as a ConfigMap and a Secret in the Garden cluster.
func (b *AWSBotanist) generateTerraformKube2IAMConfig(kube2iamRoles []gardenv1beta1.Kube2IAMRole) (map[string]interface{}, error) {
	nodesRoleARN := "nodes_role_arn"
	stateVariables, err := terraformer.New(b.Operation, common.TerraformerPurposeInfra).GetStateOutputVariables(nodesRoleARN)
	if err != nil {
		return nil, err
	}

	roles, err := b.createKube2IAMRoles(kube2iamRoles)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"nodesRoleARN": stateVariables[nodesRoleARN],
		"roles":        roles,
	}, nil
}

// createKube2IAMRoles creates the policy documents for AWS IAM in order to allow applying the respective access
// permissions. It returns the (JSON) policy document as a string.
func (b *AWSBotanist) createKube2IAMRoles(customRoles []gardenv1beta1.Kube2IAMRole) ([]gardenv1beta1.Kube2IAMRole, error) {
	var (
		tmpRoles, roles []gardenv1beta1.Kube2IAMRole
	)

	awsAccountID, err := b.AWSClient.GetAccountID()
	if err != nil {
		return nil, err
	}

	// If the ClusterAutoScaler addon is enabled, then we have to create an appropriate instance profile for it such that
	// it can scale up/down the cluster nodes.
	if b.Shoot.Info.Spec.Addons.ClusterAutoscaler.Enabled {
		stateConfigMap, err := terraformer.New(b.Operation, common.TerraformerPurposeInfra).GetState()
		if err != nil {
			return nil, err
		}
		state := utils.ConvertJSONToMap(stateConfigMap)
		var autoScalingARNs []string

		for i := range b.Shoot.Info.Spec.Cloud.AWS.Zones {
			for j := range b.Shoot.Info.Spec.Cloud.AWS.Workers {
				value, err := state.String("modules", "0", "outputs", fmt.Sprintf("asg_nodes_pool%d_z%d", j, i), "value")
				if err != nil {
					return nil, err
				}

				autoScalingARNs = append(autoScalingARNs, `"`+value+`"`)
			}
		}

		tmpRoles = append(tmpRoles, gardenv1beta1.Kube2IAMRole{
			Name:        "autoscaling",
			Description: "Allow access to scale AutoScaling Groups up/down",
			Policy: fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "autoscaling:DescribeAutoScalingGroups",
        "autoscaling:DescribeAutoScalingInstances"
      ],
      "Effect": "Allow",
      "Resource": "*"
    },
    {
      "Action": [
        "autoscaling:SetDesiredCapacity",
        "autoscaling:TerminateInstanceInAutoScalingGroup"
      ],
      "Effect": "Allow",
      "Resource": [%s]
    }
  ]
}`, strings.Join(autoScalingARNs, ", ")),
		})
	}

	// Add the roles defined in the Shoot manifest (.spec.addons.kube2iam.roles) to the list of roles we will
	// create for Kube2IAM.
	for _, customRole := range customRoles {
		customRole.Policy = strings.Replace(customRole.Policy, "${region}", b.Shoot.Info.Spec.Cloud.Region, -1)
		customRole.Policy = strings.Replace(customRole.Policy, "${account_id}", awsAccountID, -1)
		tmpRoles = append(tmpRoles, customRole)
	}

	for _, role := range tmpRoles {
		role.Name = fmt.Sprintf("%s-%s", b.Shoot.SeedNamespace, role.Name)
		roles = append(roles, role)
	}
	return roles, nil
}

// GenerateKube2IAMConfig generates the values which are required to render the chart of kube2iam properly.
func (b *AWSBotanist) GenerateKube2IAMConfig() (map[string]interface{}, error) {
	var (
		enabled = b.Shoot.Info.Spec.Addons.Kube2IAM.Enabled
		values  map[string]interface{}
	)

	if enabled {
		awsAccountID, err := b.AWSClient.GetAccountID()
		if err != nil {
			return nil, err
		}
		values = map[string]interface{}{
			"extraArgs": map[string]interface{}{
				"base-role-arn": fmt.Sprintf("arn:aws:iam::%s:role/", awsAccountID),
			},
		}
	}

	return common.GenerateAddonConfig(values, enabled), nil
}

// GenerateClusterAutoscalerConfig generates the values which are required to render the chart of
// aws-cluster-autosclaer properly.
func (b *AWSBotanist) GenerateClusterAutoscalerConfig() (map[string]interface{}, error) {
	var (
		podAnnotations = map[string]interface{}{}
		enabled        = b.Shoot.Info.Spec.Addons.ClusterAutoscaler.Enabled
		values         map[string]interface{}
	)

	if enabled {
		autoscalingGroups := b.GetASGs()

		if b.Shoot.Info.Spec.Addons.Kube2IAM.Enabled {
			podAnnotations["iam.amazonaws.com/role"] = fmt.Sprintf("%s-autoscaling", b.Shoot.SeedNamespace)
		}
		values = map[string]interface{}{
			"awsRegion":         b.Shoot.Info.Spec.Cloud.Region,
			"autoscalingGroups": autoscalingGroups,
			"podAnnotations":    podAnnotations,
			"waitForKube2IAM":   b.Shoot.Info.Spec.Addons.Kube2IAM.Enabled,
		}
	}

	return common.GenerateAddonConfig(values, enabled), nil
}

// GenerateAdmissionControlConfig generates values which are required to render the chart admissions-controls properly.
func (b *AWSBotanist) GenerateAdmissionControlConfig() (map[string]interface{}, error) {
	return map[string]interface{}{
		"StorageClasses": []map[string]interface{}{
			{
				"Name":           "default",
				"IsDefaultClass": true,
				"Provisioner":    "kubernetes.io/aws-ebs",
				"Parameters": map[string]interface{}{
					"type": "gp2",
				},
			},
			{
				"Name":           "gp2",
				"IsDefaultClass": false,
				"Provisioner":    "kubernetes.io/aws-ebs",
				"Parameters": map[string]interface{}{
					"type": "gp2",
				},
			},
		},
	}, nil
}

// GenerateCalicoConfig generates values which are required to render the chart calico properly.
func (b *AWSBotanist) GenerateCalicoConfig() (map[string]interface{}, error) {
	return map[string]interface{}{
		"cloudProvider": b.Shoot.CloudProvider,
		"enabled":       true,
	}, nil
}

// GenerateNginxIngressConfig generates values which are required to render the chart nginx-ingress properly.
func (b *AWSBotanist) GenerateNginxIngressConfig() (map[string]interface{}, error) {
	// Use common.GenerateAddonConfig for error-handling.
	return common.GenerateAddonConfig(map[string]interface{}{
		"controller": map[string]interface{}{
			"config": map[string]interface{}{
				"use-proxy-protocol": "true",
			},
		},
	}, b.Shoot.Info.Spec.Addons.NginxIngress.Enabled), nil
}

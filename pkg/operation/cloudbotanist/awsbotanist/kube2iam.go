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
	"fmt"
	"strings"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	awsclient "github.com/gardener/gardener/pkg/client/aws"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/operation/terraformer"

	awsv1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-aws/pkg/apis/aws/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// DISCLAIMER:
// The whole file is deprecated. We are keeping kube2iam for a short amount of time for backwards compatibility reasons,
// however, end-users are asked to deploy kube2iam on their own if they want to use it. Also, we haven't updated the version
// since long and won't do that anymore.
// Everything related to kube2iam will be removed in the future.

const terraformerPurposeKube2IAM = "kube2iam"

// DeployKube2IAMResources creates the respective IAM roles which have been specified in the Shoot manifest
// addon section. Moreover, some default IAM roles will be created.
// +deprecated
func DeployKube2IAMResources(o *operation.Operation) error {
	if !o.Shoot.Kube2IAMEnabled() {
		return DestroyKube2IAMResources(o)
	}

	values, err := generateTerraformKube2IAMConfig(o.Shoot)
	if err != nil {
		return err
	}

	tf, err := o.NewShootTerraformer(terraformerPurposeKube2IAM)
	if err != nil {
		return err
	}

	return tf.
		SetVariablesEnvironment(generateTerraformKube2IAMVariablesEnvironment(o.Shoot.Secret)).
		InitializeWith(o.ChartInitializer("aws-kube2iam", values)).
		Apply()
}

// DestroyKube2IAMResources destroy the kube2iam resources created by Terraform. This comprises IAM roles and
// policies.
// +deprecated
func DestroyKube2IAMResources(o *operation.Operation) error {
	tf, err := o.NewShootTerraformer(terraformerPurposeKube2IAM)
	if err != nil {
		return err
	}
	return tf.
		SetVariablesEnvironment(generateTerraformKube2IAMVariablesEnvironment(o.Shoot.Secret)).
		Destroy()
}

// GenerateKube2IAMConfig generates the values which are required to render the chart of kube2iam properly.
// +deprecated
func GenerateKube2IAMConfig(o *operation.Operation) (map[string]interface{}, error) {
	var (
		enabled = o.Shoot.Kube2IAMEnabled()
		values  map[string]interface{}
	)

	if enabled {
		awsClient := createAWSClient(o.Shoot.Secret, o.Shoot.Info.Spec.Cloud.Region)

		awsAccountID, err := awsClient.GetAccountID()
		if err != nil {
			return nil, err
		}

		v, err := o.InjectShootShootImages(map[string]interface{}{
			"extraArgs": map[string]interface{}{
				"base-role-arn": fmt.Sprintf("arn:aws:iam::%s:role/", awsAccountID),
			},
		}, "kube2iam")
		if err != nil {
			return nil, err
		}
		values = v
	}

	return common.GenerateAddonConfig(values, enabled), nil
}

// generateTerraformKube2IAMConfig creates the Terraform variables and the Terraform config (for kube2iam)
// and returns them (these values will be stored as a ConfigMap and a Secret in the Garden cluster.
func generateTerraformKube2IAMConfig(shoot *shoot.Shoot) (map[string]interface{}, error) {
	// This code will only exist temporarily until we have completed the Extensibility epic. After it, kube2iam
	// will no longer be supported by Gardener.
	if shoot.InfrastructureStatus == nil {
		return nil, fmt.Errorf("no infrastructure status found")
	}
	infrastructureStatus, err := infrastructureStatusFromInfrastructure(shoot.InfrastructureStatus)
	if err != nil {
		return nil, err
	}

	nodesRole, err := findRoleByPurpose(infrastructureStatus.IAM.Roles, awsv1alpha1.PurposeNodes)
	if err != nil {
		return nil, err
	}

	roles, err := createKube2IAMRoles(shoot, shoot.Info.Spec.Addons.Kube2IAM.Roles)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"nodesRoleARN": nodesRole.ARN,
		"roles":        roles,
	}, nil
}

// generateTerraformKube2IAMVariablesEnvironment generates the environment containing the credentials which
// are required to validate/apply/destroy the Terraform configuration. These environment must contain
// Terraform variables which are prefixed with TF_VAR_.
func generateTerraformKube2IAMVariablesEnvironment(secret *corev1.Secret) map[string]string {
	return terraformer.GenerateVariablesEnvironment(secret, map[string]string{
		"ACCESS_KEY_ID":     AccessKeyID,
		"SECRET_ACCESS_KEY": SecretAccessKey,
	})
}

// createKube2IAMRoles creates the policy documents for AWS IAM in order to allow applying the respective access
// permissions. It returns the (JSON) policy document as a string.
func createKube2IAMRoles(shoot *shoot.Shoot, customRoles []gardenv1beta1.Kube2IAMRole) ([]gardenv1beta1.Kube2IAMRole, error) {
	var (
		tmpRoles, roles []gardenv1beta1.Kube2IAMRole

		awsClient = createAWSClient(shoot.Secret, shoot.Info.Spec.Cloud.Region)
	)

	awsAccountID, err := awsClient.GetAccountID()
	if err != nil {
		return nil, err
	}

	// Add the roles defined in the Shoot manifest (.spec.addons.kube2iam.roles) to the list of roles we will
	// create for Kube2IAM.
	for _, customRole := range customRoles {
		customRole.Policy = strings.Replace(customRole.Policy, "${region}", shoot.Info.Spec.Cloud.Region, -1)
		customRole.Policy = strings.Replace(customRole.Policy, "${account_id}", awsAccountID, -1)
		tmpRoles = append(tmpRoles, customRole)
	}

	for _, role := range tmpRoles {
		role.Name = fmt.Sprintf("%s-%s", shoot.SeedNamespace, role.Name)
		roles = append(roles, role)
	}
	return roles, nil
}

func createAWSClient(secret *corev1.Secret, region string) awsclient.ClientInterface {
	return awsclient.NewClient(string(secret.Data[AccessKeyID]), string(secret.Data[SecretAccessKey]), region)
}

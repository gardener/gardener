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
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/terraformer"

	corev1 "k8s.io/api/core/v1"
)

const (
	// AccessKeyID is a constant for the key in a cloud provider secret and backup secret that holds the AWS access key id.
	AccessKeyID = "accessKeyID"
	// SecretAccessKey is a constant for the key in a cloud provider secret and backup secret that holds the AWS secret access key.
	SecretAccessKey = "secretAccessKey"
)

// DISCLAIMER:
// The whole file is deprecated. We are keeping kube2iam for a short amount of time for backwards compatibility reasons,
// however, end-users are asked to deploy kube2iam on their own if they want to use it. Also, we haven't updated the version
// since long and won't do that anymore.
// Everything related to kube2iam will be removed in the future.

const terraformerPurposeKube2IAM = "kube2iam"

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

// generateTerraformKube2IAMVariablesEnvironment generates the environment containing the credentials which
// are required to validate/apply/destroy the Terraform configuration. These environment must contain
// Terraform variables which are prefixed with TF_VAR_.
func generateTerraformKube2IAMVariablesEnvironment(secret *corev1.Secret) map[string]string {
	return terraformer.GenerateVariablesEnvironment(secret, map[string]string{
		"ACCESS_KEY_ID":     AccessKeyID,
		"SECRET_ACCESS_KEY": SecretAccessKey,
	})
}

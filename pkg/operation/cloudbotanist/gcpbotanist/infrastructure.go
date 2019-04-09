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

package gcpbotanist

import (
	"context"
	"time"

	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/terraformer"
	"github.com/gardener/gardener/pkg/utils/flow"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// DeployInfrastructure kicks off a Terraform job which deploys the infrastructure.
func (b *GCPBotanist) DeployInfrastructure() error {
	var (
		vpcName   = "${google_compute_network.network.name}"
		createVPC = true
	)

	// check if we should use an existing VPC or create a new one
	if b.VPCName != "" {
		vpcName = b.VPCName
		createVPC = false
	}
	tf, err := b.NewShootTerraformer(common.TerraformerPurposeInfra)
	if err != nil {
		return err
	}
	return tf.
		SetVariablesEnvironment(b.generateTerraformInfraVariablesEnvironment()).
		InitializeWith(b.ChartInitializer("gcp-infra", b.generateTerraformInfraConfig(createVPC, vpcName))).
		Apply()
}

// DestroyInfrastructure kicks off a Terraform job which destroys the infrastructure.
func (b *GCPBotanist) DestroyInfrastructure() error {
	tf, err := b.NewShootTerraformer(common.TerraformerPurposeInfra)
	if err != nil {
		return err
	}

	configExists, err := tf.ConfigExists()
	if err != nil {
		return err
	}
	var (
		g = flow.NewGraph("GCP infrastructure destruction")

		destroyKubernetesFirewallRulesStep = g.Add(flow.Task{
			Name: "Destroying Kubernetes firewall rules",
			Fn:   flow.TaskFn(b.destroyKubernetesFirewallRules).RetryUntilTimeout(10*time.Second, 5*time.Minute).DoIf(configExists),
		})

		destroyKubernetesRoutesStep = g.Add(flow.Task{
			Name: "Destroying Kubernetes routes",
			Fn:   flow.TaskFn(b.destroyKubernetesRoutes).RetryUntilTimeout(10*time.Second, 5*time.Minute).DoIf(configExists),
		})

		_ = g.Add(flow.Task{
			Name:         "Destroying Shoot infrastructure",
			Fn:           flow.SimpleTaskFn(tf.SetVariablesEnvironment(b.generateTerraformInfraVariablesEnvironment()).Destroy),
			Dependencies: flow.NewTaskIDs(destroyKubernetesFirewallRulesStep, destroyKubernetesRoutesStep),
		})

		f = g.Compile()
	)

	if err := f.Run(flow.Opts{Logger: b.Logger}); err != nil {
		return flow.Causes(err)
	}
	return nil
}

// generateTerraformInfraVariablesEnvironment generates the environment containing the credentials which
// are required to validate/apply/destroy the Terraform configuration. These environment must contain
// Terraform variables which are prefixed with TF_VAR_.
func (b *GCPBotanist) generateTerraformInfraVariablesEnvironment() map[string]string {
	return map[string]string{
		"TF_VAR_SERVICEACCOUNT": b.MinifiedServiceAccount,
	}
}

// generateTerraformInfraConfig creates the Terraform variables and the Terraform config (for the infrastructure)
// and returns them (these values will be stored as a ConfigMap and a Secret in the Garden cluster.
func (b *GCPBotanist) generateTerraformInfraConfig(createVPC bool, vpcName string) map[string]interface{} {
	var internal string
	if b.Shoot.Info.Spec.Cloud.GCP.Networks.Internal != nil {
		internal = string(*b.Shoot.Info.Spec.Cloud.GCP.Networks.Internal)
	}
	return map[string]interface{}{
		"google": map[string]interface{}{
			"region":  b.Shoot.Info.Spec.Cloud.Region,
			"project": b.Project,
		},
		"create": map[string]interface{}{
			"vpc": createVPC,
		},
		"vpc": map[string]interface{}{
			"name": vpcName,
		},
		"clusterName": b.Shoot.SeedNamespace,
		"networks": map[string]interface{}{
			"pods":     b.Shoot.GetPodNetwork(),
			"services": b.Shoot.GetServiceNetwork(),
			"worker":   b.Shoot.Info.Spec.Cloud.GCP.Networks.Workers[0],
			"internal": internal,
		},
	}
}

func (b *GCPBotanist) getVpcName() (string, error) {
	t, err := b.NewShootTerraformer(common.TerraformerPurposeInfra)
	if err != nil {
		return "", err
	}

	if _, err := t.GetState(); err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}
		return "", err
	}

	vpcNameID := "vpc_name"
	stateVariables, err := t.GetStateOutputVariables(vpcNameID)
	if err != nil {
		return "", err
	}
	return stateVariables[vpcNameID], nil
}

func (b *GCPBotanist) destroyKubernetesFirewallRules(ctx context.Context) error {
	vpcName, err := b.getVpcName()
	if err != nil {
		if terraformer.IsVariablesNotFoundError(err) {
			b.Logger.Infof("Skipping explicit GCP firewall rule deletion because not all variables have been found in the Terraform state.")
			return nil
		}
		return err
	} else if vpcName == "" {
		return nil
	}

	// Find firewall rules which are created by k8s cloudprovider within the shoot network.
	firewallRuleNames, err := b.GCPClient.ListKubernetesFirewallRulesForNetwork(ctx, b.Project, vpcName)
	if err != nil {
		return err
	}

	// Destroy firewall rules.
	for _, firewallRuleName := range firewallRuleNames {
		if err := b.GCPClient.DeleteFirewallRule(ctx, b.Project, firewallRuleName); err != nil {
			return err
		}
	}
	return nil
}

func (b *GCPBotanist) destroyKubernetesRoutes(ctx context.Context) error {
	vpcName, err := b.getVpcName()
	if err != nil {
		if terraformer.IsVariablesNotFoundError(err) {
			b.Logger.Infof("Skipping explicit GCP route deletion because not all variables have been found in the Terraform state.")
			return nil
		}
		return err
	} else if vpcName == "" {
		return nil
	}

	// Find routes which are created by k8s cloudprovider within the shoot network.
	routeNames, err := b.GCPClient.ListKubernetesRoutesForNetwork(ctx, b.Project, vpcName, b.Shoot.SeedNamespace)
	if err != nil {
		return err
	}

	// Destroy route entries.
	for _, routeName := range routeNames {
		if err := b.GCPClient.DeleteRoute(ctx, b.Project, routeName); err != nil {
			return err
		}
	}
	return nil
}

// DeployBackupInfrastructure kicks off a Terraform job which deploys the infrastructure resources for backup.
func (b *GCPBotanist) DeployBackupInfrastructure() error {
	tf, err := b.NewBackupInfrastructureTerraformer()
	if err != nil {
		return err
	}
	return tf.
		SetVariablesEnvironment(b.generateTerraformBackupVariablesEnvironment()).
		InitializeWith(b.ChartInitializer("gcp-backup", b.generateTerraformBackupConfig())).
		Apply()
}

// DestroyBackupInfrastructure kicks off a Terraform job which destroys the infrastructure for backup.
func (b *GCPBotanist) DestroyBackupInfrastructure() error {
	tf, err := b.NewBackupInfrastructureTerraformer()
	if err != nil {
		return err
	}
	return tf.
		SetVariablesEnvironment(b.generateTerraformBackupVariablesEnvironment()).
		Destroy()
}

// generateTerraformBackupVariablesEnvironment generates the environment containing the credentials which
// are required to validate/apply/destroy the Terraform configuration. These environment must contain
// Terraform variables which are prefixed with TF_VAR_.
func (b *GCPBotanist) generateTerraformBackupVariablesEnvironment() map[string]string {
	return map[string]string{
		"TF_VAR_SERVICEACCOUNT": b.MinifiedServiceAccount,
	}
}

// generateTerraformBackupConfig creates the Terraform variables and the Terraform config (for the backup)
// and returns them.
func (b *GCPBotanist) generateTerraformBackupConfig() map[string]interface{} {
	return map[string]interface{}{
		"google": map[string]interface{}{
			"region":  b.Seed.Info.Spec.Cloud.Region,
			"project": b.Project,
		},
		"bucket": map[string]interface{}{
			"name": b.Operation.BackupInfrastructure.Name,
		},
		"clusterName": b.Operation.BackupInfrastructure.Name,
	}
}

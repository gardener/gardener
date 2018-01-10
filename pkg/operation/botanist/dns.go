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

package botanist

import (
	"fmt"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/operation/cloudbotanist/awsbotanist"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/terraformer"
	corev1 "k8s.io/api/core/v1"
)

// DeployInternalDomainDNSRecord deploys the DNS record for the internal cluster domain.
func (b *Botanist) DeployInternalDomainDNSRecord() error {
	return b.DeployDNSRecord(common.TerraformerPurposeInternalDNS, b.Shoot.InternalClusterDomain, b.APIServerAddress, true)
}

// DestroyInternalDomainDNSRecord destroys the DNS record for the internal cluster domain.
func (b *Botanist) DestroyInternalDomainDNSRecord() error {
	return b.DestroyDNSRecord(common.TerraformerPurposeInternalDNS, true)
}

// DeployExternalDomainDNSRecord deploys the DNS record for the external cluster domain.
func (b *Botanist) DeployExternalDomainDNSRecord() error {
	return b.DeployDNSRecord(common.TerraformerPurposeExternalDNS, *(b.Shoot.ExternalClusterDomain), b.Shoot.InternalClusterDomain, false)
}

// DestroyExternalDomainDNSRecord destroys the DNS record for the external cluster domain.
func (b *Botanist) DestroyExternalDomainDNSRecord() error {
	return b.DestroyDNSRecord(common.TerraformerPurposeExternalDNS, false)
}

// DeployDNSRecord kicks off a Terraform job of name <alias> which deploys the DNS record for <name> which
// will point to <target>.
func (b *Botanist) DeployDNSRecord(terraformerPurpose, name, target string, purposeInternalDomain bool) error {
	var (
		tfvarsEnvironment []map[string]interface{}
		values            map[string]interface{}
		err               error
	)

	switch b.determineDNSProvider(purposeInternalDomain) {
	case gardenv1beta1.DNSAWSRoute53:
		tfvarsEnvironment, err = b.GenerateTerraformRoute53VariablesEnvironment(purposeInternalDomain)
		if err != nil {
			return err
		}
		values = b.GenerateTerraformRoute53Config(name, []string{target})
	default:
		return nil
	}

	return terraformer.
		New(b.Operation, terraformerPurpose).
		SetVariablesEnvironment(tfvarsEnvironment).
		DefineConfig("aws-route53", values).
		Apply()
}

// DestroyDNSRecord kicks off a Terraform job which destroys the DNS record.
func (b *Botanist) DestroyDNSRecord(terraformerPurpose string, purposeInternalDomain bool) error {
	var (
		tfvarsEnvironment []map[string]interface{}
		err               error
	)

	switch b.determineDNSProvider(purposeInternalDomain) {
	case gardenv1beta1.DNSAWSRoute53:
		tfvarsEnvironment, err = b.GenerateTerraformRoute53VariablesEnvironment(purposeInternalDomain)
		if err != nil {
			return err
		}
	}

	return terraformer.
		New(b.Operation, terraformerPurpose).
		SetVariablesEnvironment(tfvarsEnvironment).
		Destroy()
}

// GenerateTerraformRoute53VariablesEnvironment generates the environment containing the credentials which
// are required to validate/apply/destroy the Terraform configuration. These environment must contain
// Terraform variables which are prefixed with TF_VAR_.
func (b *Botanist) GenerateTerraformRoute53VariablesEnvironment(purposeInternalDomain bool) ([]map[string]interface{}, error) {
	secret, err := b.getDomainCredentials(purposeInternalDomain, awsbotanist.AccessKeyID, awsbotanist.SecretAccessKey)
	if err != nil {
		return nil, err
	}
	keyValueMap := map[string]string{
		"ACCESS_KEY_ID":     awsbotanist.AccessKeyID,
		"SECRET_ACCESS_KEY": awsbotanist.SecretAccessKey,
	}
	return common.GenerateTerraformVariablesEnvironment(secret, keyValueMap), nil
}

// GenerateTerraformRoute53Config creates the Terraform variables and the Terraform config (for the DNS record)
// and returns them (these values will be stored as a ConfigMap and a Secret in the Garden cluster.
func (b *Botanist) GenerateTerraformRoute53Config(name string, values []string) map[string]interface{} {
	targetType, _ := common.IdentifyAddressType(values[0])

	return map[string]interface{}{
		"record": map[string]interface{}{
			"hostedZoneID": b.Shoot.Info.Spec.DNS.HostedZoneID,
			"name":         name,
			"type":         targetType,
			"values":       values,
		},
	}
}

func (b *Botanist) determineDNSProvider(purposeInternalDomain bool) gardenv1beta1.DNSProvider {
	if purposeInternalDomain {
		return gardenv1beta1.DNSProvider(b.Secrets[common.GardenRoleInternalDomain].Annotations[common.DNSProvider])
	}
	return b.Shoot.Info.Spec.DNS.Provider
}

func (b *Botanist) getDomainCredentials(purposeInternalDomain bool, requiredKeys ...string) (*corev1.Secret, error) {
	var secret *corev1.Secret

	switch {
	case purposeInternalDomain:
		secret = b.Secrets[common.GardenRoleInternalDomain]
	case b.DefaultDomainSecret != nil:
		secret = b.DefaultDomainSecret
	default:
		secret = b.Shoot.Secret
	}

	for _, key := range requiredKeys {
		if _, ok := secret.Data[key]; !ok {
			return nil, fmt.Errorf("cannot use secret '%s' to create the DNS record because key '%s' is missing", secret.Name, key)
		}
	}
	return secret, nil
}

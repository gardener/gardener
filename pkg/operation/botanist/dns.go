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

package botanist

import (
	"fmt"
	"strings"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/operation/cloudbotanist/alicloudbotanist"
	"github.com/gardener/gardener/pkg/operation/cloudbotanist/awsbotanist"
	"github.com/gardener/gardener/pkg/operation/cloudbotanist/gcpbotanist"
	"github.com/gardener/gardener/pkg/operation/cloudbotanist/openstackbotanist"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/terraformer"
	"github.com/gardener/gardener/pkg/utils"
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
		chartName         string
		tfvarsEnvironment map[string]string
		targetType, _     = common.IdentifyAddressType(target)
	)

	// If the DNS record is already registered properly then we skip the reconciliation to avoid running into
	// cloud provider rate limits.
	switch targetType {
	case "hostname":
		cname, err := utils.LookupDNSHostCNAME(name)
		if err != nil {
			b.Logger.Errorf("Something went wrong with DNS lookup for %s, reason: %s", name, err.Error())
		}
		if cname == fmt.Sprintf("%s.", target) {
			b.Logger.Infof("Skipping DNS record registration because '%s' already points to '%s'", name, target)
			return nil
		}
	case "ip":
		values, err := utils.LookupDNSHost(name)
		if err != nil {
			b.Logger.Errorf("Something went wrong with DNS lookup for %s, reason: %s", name, err.Error())
		}

		for _, v := range values {
			if v == target {
				b.Logger.Infof("Skipping DNS record registration because '%s' already points to '%s'", name, target)
				return nil
			}
		}
	}

	b.Logger.Infof("Initiating Terraform validation for domain %s", name)
	tf, err := b.NewShootTerraformer(terraformerPurpose)
	if err != nil {
		return err
	}

	var config map[string]interface{}

	switch b.determineDNSProvider(purposeInternalDomain) {
	case gardenv1beta1.DNSAWSRoute53:
		hostedZoneID, err := b.getHostedZoneID(purposeInternalDomain)
		if err != nil {
			return err
		}
		tfvarsEnvironment, err = b.GenerateTerraformRoute53VariablesEnvironment(purposeInternalDomain)
		if err != nil {
			return err
		}
		chartName = "aws-route53"
		config = b.GenerateTerraformDNSConfig(name, hostedZoneID, targetType, []string{target})
	case gardenv1beta1.DNSGoogleCloudDNS:
		hostedZoneID, err := b.getHostedZoneID(purposeInternalDomain)
		if err != nil {
			return err
		}
		tfvarsEnvironment, err = b.GenerateTerraformCloudDNSVariablesEnvironment(purposeInternalDomain)
		if err != nil {
			return err
		}
		chartName = "gcp-clouddns"
		config = b.GenerateTerraformDNSConfig(name, hostedZoneID, targetType, []string{target})
	case gardenv1beta1.DNSAlicloud:
		tfvarsEnvironment, err = b.GenerateTerraformAlicloudDNSVariablesEnvironment(purposeInternalDomain)
		if err != nil {
			return err
		}
		chartName = "alicloud-dns"
		config, err = b.generateTerraformAlicloudDNSConfig(name, targetType, target, purposeInternalDomain)
		if err != nil {
			return err
		}
	case gardenv1beta1.DNSOpenstackDesignate:
		hostedZoneID, err := b.getHostedZoneID(purposeInternalDomain)
		if err != nil {
			return err
		}
		tfvarsEnvironment, err = b.GenerateTerraformDesignateDNSVariablesEnvironment(purposeInternalDomain)
		if err != nil {
			return err
		}
		chartName = "openstack-designate"
		config = b.GenerateTerraformDNSConfig(name, hostedZoneID, targetType, []string{target})
	default:
		return nil
	}

	return tf.
		SetVariablesEnvironment(tfvarsEnvironment).
		InitializeWith(b.ChartInitializer(chartName, config)).
		Apply()
}

// DestroyDNSRecord kicks off a Terraform job which destroys the DNS record.
func (b *Botanist) DestroyDNSRecord(terraformerPurpose string, purposeInternalDomain bool) error {
	var (
		tfvarsEnvironment map[string]string
		err               error
	)

	switch b.determineDNSProvider(purposeInternalDomain) {
	case gardenv1beta1.DNSAWSRoute53:
		tfvarsEnvironment, err = b.GenerateTerraformRoute53VariablesEnvironment(purposeInternalDomain)
		if err != nil {
			return err
		}
	case gardenv1beta1.DNSGoogleCloudDNS:
		tfvarsEnvironment, err = b.GenerateTerraformCloudDNSVariablesEnvironment(purposeInternalDomain)
		if err != nil {
			return err
		}
	case gardenv1beta1.DNSOpenstackDesignate:
		tfvarsEnvironment, err = b.GenerateTerraformDesignateDNSVariablesEnvironment(purposeInternalDomain)
		if err != nil {
			return err
		}
	case gardenv1beta1.DNSAlicloud:
		tfvarsEnvironment, err = b.GenerateTerraformAlicloudDNSVariablesEnvironment(purposeInternalDomain)
		if err != nil {
			return err
		}
	default:
		return nil
	}
	tf, err := b.NewShootTerraformer(terraformerPurpose)
	if err != nil {
		return err
	}
	return tf.
		SetVariablesEnvironment(tfvarsEnvironment).
		Destroy()
}

// GenerateTerraformRoute53VariablesEnvironment generates the environment containing the credentials which
// are required to validate/apply/destroy the Terraform configuration. These environment must contain
// Terraform variables which are prefixed with TF_VAR_.
func (b *Botanist) GenerateTerraformRoute53VariablesEnvironment(purposeInternalDomain bool) (map[string]string, error) {
	secret, err := b.getDomainCredentials(purposeInternalDomain, awsbotanist.AccessKeyID, awsbotanist.SecretAccessKey)
	if err != nil {
		return nil, err
	}
	keyValueMap := map[string]string{
		"ACCESS_KEY_ID":     awsbotanist.AccessKeyID,
		"SECRET_ACCESS_KEY": awsbotanist.SecretAccessKey,
	}
	return terraformer.GenerateVariablesEnvironment(secret, keyValueMap), nil
}

// GenerateTerraformAlicloudDNSVariablesEnvironment generates the environment containing the credentials which
// are required to validate/apply/destroy the Terraform configuration. These environment must contain
// Terraform variables which are prefixed with TF_VAR_.
func (b *Botanist) GenerateTerraformAlicloudDNSVariablesEnvironment(purposeInternalDomain bool) (map[string]string, error) {
	secret, err := b.getDomainCredentials(purposeInternalDomain, alicloudbotanist.AccessKeyID, alicloudbotanist.AccessKeySecret)
	if err != nil {
		return nil, err
	}

	keyValueMap := map[string]string{
		"ACCESS_KEY_ID":     alicloudbotanist.AccessKeyID,
		"ACCESS_KEY_SECRET": alicloudbotanist.AccessKeySecret,
	}

	return terraformer.GenerateVariablesEnvironment(secret, keyValueMap), nil
}

// GenerateTerraformCloudDNSVariablesEnvironment generates the environment containing the credentials which
// Terraform variables which are prefixed with TF_VAR_.
func (b *Botanist) GenerateTerraformCloudDNSVariablesEnvironment(purposeInternalDomain bool) (map[string]string, error) {
	secret, err := b.getDomainCredentials(purposeInternalDomain, gcpbotanist.ServiceAccountJSON)
	if err != nil {
		return nil, err
	}
	project, err := gcpbotanist.ExtractProjectID(secret.Data[gcpbotanist.ServiceAccountJSON])
	if err != nil {
		return nil, err
	}
	minifiedServiceAccount, err := gcpbotanist.MinifyServiceAccount(secret.Data[gcpbotanist.ServiceAccountJSON])
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"TF_VAR_SERVICEACCOUNT": minifiedServiceAccount,
		"GOOGLE_PROJECT":        project,
	}, nil
}

// GenerateTerraformDesignateDNSVariablesEnvironment generates the environment containing the credentials which
// are required to validate/apply/destroy the Terraform configuration. These environment must contain
// Terraform variables which are prefixed with TF_VAR_.
func (b *Botanist) GenerateTerraformDesignateDNSVariablesEnvironment(purposeInternalDomain bool) (map[string]string, error) {
	secret, err := b.getDomainCredentials(purposeInternalDomain, openstackbotanist.AuthURL, openstackbotanist.DomainName, openstackbotanist.TenantName, openstackbotanist.UserName, openstackbotanist.UserDomainName, openstackbotanist.Password)
	if err != nil {
		return nil, err
	}
	keyValueMap := map[string]string{
		"OS_AUTH_URL":         openstackbotanist.AuthURL,
		"OS_DOMAIN_NAME":      openstackbotanist.DomainName,
		"OS_TENANT_NAME":      openstackbotanist.TenantName,
		"OS_USERNAME":         openstackbotanist.UserName,
		"OS_USER_DOMAIN_NAME": openstackbotanist.UserDomainName,
		"OS_PASSWORD":         openstackbotanist.Password,
	}

	return terraformer.GenerateVariablesEnvironment(secret, keyValueMap), nil
}

// GenerateTerraformDNSConfig creates the Terraform variables and the Terraform config (for the DNS record)
// and returns them (these values will be stored as a ConfigMap and a Secret in the Garden cluster.
func (b *Botanist) GenerateTerraformDNSConfig(name, hostedZoneID, targetType string, values []string) map[string]interface{} {
	return map[string]interface{}{
		"record": map[string]interface{}{
			"hostedZoneID": hostedZoneID,
			"name":         name,
			"type":         targetType,
			"values":       values,
		},
	}
}

// generateTerraformAlicloudDNSConfig To adapt Alicloud Terraform,
// @input Param domain should be split as name and host_record.
// name is registered in Alicloud. It is always like xxx.xxx. If there is an exception, this function should be rewritten !!!
// host_record is the host name in A or CNAME record
func (b *Botanist) generateTerraformAlicloudDNSConfig(domain, targetType string, value string, purposeInternalDomain bool) (map[string]interface{}, error) {
	splits := strings.Split(domain, ".")
	// Shoot validation promises the shoot.spec.dns.domain has at least one ".", and domain has more than one "."
	l := len(splits)
	if l < 2 {
		return nil, fmt.Errorf("Domain %s is not valid", domain)
	}
	name := fmt.Sprintf("%s.%s", splits[l-2], splits[l-1])

	hostRecord := strings.TrimSuffix(domain, fmt.Sprintf(".%s", name))

	return map[string]interface{}{
		"record": map[string]interface{}{
			"name":       name,
			"hostRecord": hostRecord,
			"type":       targetType,
			"value":      value,
		},
	}, nil
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
	case b.Shoot.Info.Spec.DNS.SecretName != nil:
		dnsSecret, err := b.K8sGardenClient.GetSecret(b.Shoot.Info.Namespace, *b.Shoot.Info.Spec.DNS.SecretName)
		if err != nil {
			return nil, err
		}
		secret = dnsSecret
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

func (b *Botanist) getHostedZoneID(purposeInternalDomain bool) (string, error) {
	switch {
	case purposeInternalDomain:
		return b.Secrets[common.GardenRoleInternalDomain].Annotations[common.DNSHostedZoneID], nil
	case b.DefaultDomainSecret != nil:
		return b.DefaultDomainSecret.Annotations[common.DNSHostedZoneID], nil
	case b.Shoot.Info.Spec.DNS.HostedZoneID != nil:
		return *b.Shoot.Info.Spec.DNS.HostedZoneID, nil
	}
	return "", fmt.Errorf("unable to determine the hosted zone id")
}

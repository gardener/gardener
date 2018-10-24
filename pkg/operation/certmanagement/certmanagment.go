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

package certmanagement

import (
	"fmt"

	"github.com/ghodss/yaml"
	corev1 "k8s.io/api/core/v1"
)

var _ DNSProviderConfig = (*Route53Config)(nil)
var _ DNSProviderConfig = (*CloudDNSConfig)(nil)

const (
	// Route53 is a constant string for aws-route53.
	Route53 DNSProvider = "aws-route53"
	// CloudDNS is a constant string for google-clouddns.
	CloudDNS DNSProvider = "google-clouddns"
)

// DNSProvider returns the provider type  in-use.
func (r *Route53Config) DNSProvider() DNSProvider {
	return Route53
}

// ACMEPrivateKey returns the private value that will be used to create the issuer secret.
func (a *ACMEConfig) ACMEPrivateKey() string {
	return a.PrivateKey
}

// AccessKey returns the route53 SecretAccessKey in case route53 provider is used.
func (r *Route53Config) AccessKey() string {
	return r.SecretAccessKey
}

// ProviderName returns the route53 provider name.
func (r *Route53Config) ProviderName() string {
	return r.Name
}

// DomainNames returns the domains this provider manages.
func (r *Route53Config) DomainNames() []string {
	return r.Domains
}

// DNSProvider returns the provider type in-use.
func (c *CloudDNSConfig) DNSProvider() DNSProvider {
	return CloudDNS
}

// AccessKey returns the CloudDNS ServiceAccount in case Google CloudDNS provider is used.
func (c *CloudDNSConfig) AccessKey() string {
	return c.ServiceAccount
}

// ProviderName returns the CloudDNS provider name.
func (c *CloudDNSConfig) ProviderName() string {
	return c.Name
}

// DomainNames returns the domains this provider manages.
func (c *CloudDNSConfig) DomainNames() []string {
	return c.Domains
}

// RetrieveCertificateManagementConfig retrieves the configuration for certificate management via Gardener.
func RetrieveCertificateManagementConfig(certificateManagement *corev1.Secret) (*CertificateManagementConfig, error) {
	certManagement, certManagementOk := certificateManagement.Data["certificate-management-config"]
	if !certManagementOk {
		return nil, fmt.Errorf("Failed to fetch certificate-management config from secret")
	}

	var certManagementConfig CertificateManagementConfig
	if err := yaml.Unmarshal(certManagement, &certManagementConfig); err != nil {
		return nil, err
	}

	return &certManagementConfig, nil
}

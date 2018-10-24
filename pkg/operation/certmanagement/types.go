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

// DNSProviderConfig is an interface that will implemented by cloud provider structs
type DNSProviderConfig interface {
	DNSProvider() DNSProvider
	AccessKey() string
	ProviderName() string
	DomainNames() []string
}

// DNSProvider string type
type DNSProvider string

// ClusterIssuerName represents the name if the ClusterIssuer
type ClusterIssuerName string

// ACMEConfig is the config for acme
type ACMEConfig struct {
	Email      string `json:"email"`
	Server     string `json:"server"`
	PrivateKey string `json:"privateKey"`
}

// Route53Config contains the route53 config
type Route53Config struct {
	Name            string   `json:"name"`
	Domains         []string `json:"domains"`
	Region          string   `json:"region"`
	AccessKeyID     string   `json:"accessKeyID"`
	SecretAccessKey string   `json:"secretAccessKey"`
}

// CloudDNSConfig contains the Google CloudDNS config
type CloudDNSConfig struct {
	Name           string   `json:"name"`
	Domains        []string `json:"domains"`
	Project        string   `json:"project"`
	ServiceAccount string   `json:"serviceAccount"`
}

// Providers are the DNS providers supported
type Providers struct {
	Route53  []Route53Config  `json:"route53"`
	CloudDNS []CloudDNSConfig `json:"clouddns"`
}

// CertificateManagementConfig is the bulk configuration that will be used for certificate management
type CertificateManagementConfig struct {
	ClusterIssuerName ClusterIssuerName `json:"clusterIssuerName"`
	ACME              ACMEConfig        `json:"acme"`
	Providers         Providers         `json:"providers"`
}

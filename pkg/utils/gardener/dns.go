// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardener

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	// DNSProvider is the key for an annotation on a Kubernetes Secret object whose value must point to a valid
	// DNS provider.
	DNSProvider = "dns.gardener.cloud/provider"
	// DNSDomain is the key for an annotation on a Kubernetes Secret object whose value must point to a valid
	// domain name.
	DNSDomain = "dns.gardener.cloud/domain"
	// DNSZone is the key for an annotation on a Kubernetes Secret object whose value must point to a valid
	// DNS hosted zone id.
	DNSZone = "dns.gardener.cloud/zone"
	// DNSIncludeZones is the key for an annotation on a Kubernetes Secret object whose value must point to a list
	// of zones that shall be included.
	DNSIncludeZones = "dns.gardener.cloud/include-zones"
	// DNSExcludeZones is the key for an annotation on a Kubernetes Secret object whose value must point to a list
	// of zones that shall be excluded.
	DNSExcludeZones = "dns.gardener.cloud/exclude-zones"
	// DNSRateLimitRequestsPerDay is the key for an annotation on a Kubernetes Secret object whose value must point to
	// the requestsPerDays value of the optional rate limit
	DNSRateLimitRequestsPerDay = "dns.gardener.cloud/rate-limit-requests-per-day"
	// DNSRateLimitBurst is the key for an annotation on a Kubernetes Secret object whose value must point to
	// the burst value of the optional rate limit
	DNSRateLimitBurst = "dns.gardener.cloud/rate-limit-burst"

	// APIServerFQDNPrefix is the part of a FQDN which will be used to construct the domain name for the kube-apiserver of
	// a Shoot cluster. For example, when a Shoot specifies domain 'cluster.example.com', the apiserver domain would be
	// 'api.cluster.example.com'.
	APIServerFQDNPrefix = "api"
	// OwnerFQDNPrefix is the part of a FQDN which will be used to construct the domain name for the owner of
	// a Shoot cluster. For example, when a Shoot specifies domain 'cluster.example.com', the owner domain would be
	// 'owner.cluster.example.com'.
	OwnerFQDNPrefix = "owner"
	// IngressPrefix is the part of a FQDN which will be used to construct the domain name for an ingress controller of
	// a Shoot cluster. For example, when a Shoot specifies domain 'cluster.example.com', the ingress domain would be
	// '*.<IngressPrefix>.cluster.example.com'.
	IngressPrefix = "ingress"
	// InternalDomainKey is a key which must be present in an internal domain constructed for a Shoot cluster. If the
	// configured internal domain already contains it, it won't be added twice. If it does not contain it, it will be
	// appended.
	InternalDomainKey = "internal"
)

// DomainInfo contains values from domain annotations
type DomainInfo struct {
	Provider     string
	Domain       string
	Zone         string
	IncludeZones []string
	ExcludeZones []string

	RateLimitRequestsPerDay int
	RateLimitBurst          int
}

// GetDomainInfoFromAnnotations returns the provider, domain, and zones that are specified in the given annotations.
func GetDomainInfoFromAnnotations(annotations map[string]string) (*DomainInfo, error) {
	if annotations == nil {
		return nil, fmt.Errorf("domain secret has no annotations")
	}

	var err error
	var info DomainInfo
	if providerAnnotation, ok := annotations[DNSProvider]; ok {
		info.Provider = providerAnnotation
	}

	if domainAnnotation, ok := annotations[DNSDomain]; ok {
		info.Domain = domainAnnotation
	}

	if zoneAnnotation, ok := annotations[DNSZone]; ok {
		info.Zone = zoneAnnotation
	}

	if includeZonesAnnotation, ok := annotations[DNSIncludeZones]; ok {
		info.IncludeZones = strings.Split(includeZonesAnnotation, ",")
	}
	if excludeZonesAnnotation, ok := annotations[DNSExcludeZones]; ok {
		info.ExcludeZones = strings.Split(excludeZonesAnnotation, ",")
	}
	if requestsPerDayStr, ok := annotations[DNSRateLimitRequestsPerDay]; ok {
		info.RateLimitRequestsPerDay, err = strconv.Atoi(requestsPerDayStr)
		if err != nil {
			return nil, fmt.Errorf("invalid value %s for %s", requestsPerDayStr, DNSRateLimitRequestsPerDay)
		}
	}

	if burstStr, ok := annotations[DNSRateLimitBurst]; ok {
		info.RateLimitBurst, err = strconv.Atoi(burstStr)
		if err != nil {
			return nil, fmt.Errorf("invalid value %s for %s", burstStr, DNSRateLimitBurst)
		}
	}

	if len(info.Domain) == 0 {
		return nil, fmt.Errorf("missing dns domain annotation on domain secret")
	}
	if len(info.Provider) == 0 {
		return nil, fmt.Errorf("missing dns provider annotation on domain secret")
	}
	if info.RateLimitRequestsPerDay != 0 {
		if info.RateLimitRequestsPerDay < 0 {
			return nil, fmt.Errorf("invalid value %d for %s (must be >= 0)", info.RateLimitRequestsPerDay, DNSRateLimitRequestsPerDay)
		}
		if info.RateLimitBurst <= 0 {
			return nil, fmt.Errorf("invalid value %d for %s (must be > 0)", info.RateLimitBurst, DNSRateLimitBurst)
		}
	}

	return &info, nil
}

// GetAPIServerDomain returns the fully qualified domain name for the api-server of the Shoot cluster. The
// end result is 'api.<domain>'.
func GetAPIServerDomain(domain string) string {
	return fmt.Sprintf("%s.%s", APIServerFQDNPrefix, domain)
}

// GetOwnerDomain returns the fully qualified domain name for the owner of the Shoot cluster. The
// end result is 'owner.<domain>'.
func GetOwnerDomain(domain string) string {
	return fmt.Sprintf("%s.%s", OwnerFQDNPrefix, domain)
}

// GenerateDNSProviderName creates a name for the dns provider out of the passed `secretName` and `providerType`.
func GenerateDNSProviderName(secretName, providerType string) string {
	switch {
	case secretName != "" && providerType != "":
		return fmt.Sprintf("%s-%s", providerType, secretName)
	case secretName != "":
		return secretName
	case providerType != "":
		return providerType
	default:
		return ""
	}
}

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"errors"
	"fmt"
	"slices"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

const (
	// DNSProvider is the key for an annotation on a Kubernetes Secret object whose value must point to a valid
	// DNS provider.
	DNSProvider = "dns.gardener.cloud/provider"
	// DNSDomain is the key for an annotation on a Kubernetes Secret object whose value must point to a valid
	// domain name.
	DNSDomain = "dns.gardener.cloud/domain"
	// DNSDefaultDomainPriority is the priority of the default domain. In case of multiple default domains
	// the default domain with the highest priority is selected per default for new shoots.
	DNSDefaultDomainPriority = "dns.gardener.cloud/domain-default-priority"
	// DNSZone is the key for an annotation on a Kubernetes Secret object whose value must point to a valid
	// DNS hosted zone id.
	DNSZone = "dns.gardener.cloud/zone"

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

	// AnnotationKeyIPStack is the annotation key to set the IP stack for a DNSRecord.
	// This can be used to create different type of records, e.g. A vs. AAAA records.
	AnnotationKeyIPStack = "dns.gardener.cloud/ip-stack"
	// AnnotationValueIPStackIPv4 is the annotation value for ipv4-only.
	AnnotationValueIPStackIPv4 = "ipv4"
	// AnnotationValueIPStackIPv6 is the annotation value for ipv6-only.
	AnnotationValueIPStackIPv6 = "ipv6"
	// AnnotationValueIPStackIPDualStack is the annotation value for dual-stack, i.e. ipv4 and ipv6.
	AnnotationValueIPStackIPDualStack = "dual-stack"
)

// GetDomainInfoFromAnnotations returns the provider, domain, and zones that are specified in the given annotations.
func GetDomainInfoFromAnnotations(annotations map[string]string) (provider string, domain string, zone string, err error) {
	if annotations == nil {
		return "", "", "", errors.New("domain secret has no annotations")
	}

	if providerAnnotation, ok := annotations[DNSProvider]; ok {
		provider = providerAnnotation
	}

	if domainAnnotation, ok := annotations[DNSDomain]; ok {
		domain = domainAnnotation
	}

	if zoneAnnotation, ok := annotations[DNSZone]; ok {
		zone = zoneAnnotation
	}

	if len(domain) == 0 {
		return "", "", "", fmt.Errorf("missing dns domain annotation on domain secret")
	}
	if len(provider) == 0 {
		return "", "", "", fmt.Errorf("missing dns provider annotation on domain secret")
	}

	return
}

// GetAPIServerDomain returns the fully qualified domain name for the api-server of a Shoot or Virtual Garden cluster. The
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

func getIPStackForFamilies(ipFamilies []gardencorev1beta1.IPFamily) string {
	if gardencorev1beta1.IsIPv4SingleStack(ipFamilies) {
		return AnnotationValueIPStackIPv4
	}
	if gardencorev1beta1.IsIPv6SingleStack(ipFamilies) {
		return AnnotationValueIPStackIPv6
	}
	if len(ipFamilies) == 2 && slices.Contains(ipFamilies, gardencorev1beta1.IPFamilyIPv4) && slices.Contains(ipFamilies, gardencorev1beta1.IPFamilyIPv6) {
		return AnnotationValueIPStackIPDualStack
	}
	// Fall-back to IPv4 per default
	return AnnotationValueIPStackIPv4
}

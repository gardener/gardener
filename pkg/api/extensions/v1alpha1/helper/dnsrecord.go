// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"
	"net"
	"slices"

	corev1 "k8s.io/api/core/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// GetDNSRecordType returns the appropriate DNS record type (A/AAAA or CNAME) for the given address.
func GetDNSRecordType(address string) extensionsv1alpha1.DNSRecordType {
	if ip := net.ParseIP(address); ip != nil {
		if ip.To4() != nil {
			return extensionsv1alpha1.DNSRecordTypeA
		}
		return extensionsv1alpha1.DNSRecordTypeAAAA
	}
	return extensionsv1alpha1.DNSRecordTypeCNAME
}

// GetDNSRecordTTL returns the value of the given ttl, or 120 if nil.
func GetDNSRecordTTL(ttl *int64) int64 {
	if ttl != nil {
		return *ttl
	}
	return 120
}

// DNSValuesFromIngress computes the values and record type for a DNSRecord from the given LoadBalancerIngress entries,
// as reported by an exposure extension. IPs are preferred over hostnames (CNAME); the IPs of the first configured IP
// family with at least one match are used.
func DNSValuesFromIngress(ingress []corev1.LoadBalancerIngress, ipFamilies []gardencorev1beta1.IPFamily) ([]string, extensionsv1alpha1.DNSRecordType, error) {
	if len(ingress) == 0 {
		return nil, "", fmt.Errorf("exposure has no ingress yet")
	}

	var ips, hostnames []string
	for _, i := range ingress {
		if i.IP != "" {
			ips = append(ips, i.IP)
		}
		if i.Hostname != "" {
			hostnames = append(hostnames, i.Hostname)
		}
	}

	for _, family := range ipFamilies {
		recordType := IPFamilyToDNSRecordType(family)
		values := slices.DeleteFunc(slices.Clone(ips), func(ip string) bool {
			return GetDNSRecordType(ip) != recordType
		})
		if len(values) > 0 {
			return values, recordType, nil
		}
	}

	switch {
	case len(ips) > 0:
		return nil, "", fmt.Errorf("none of the IP addresses %v match a configured IP family %v", ips, ipFamilies)
	case len(hostnames) > 0:
		return hostnames[:1], extensionsv1alpha1.DNSRecordTypeCNAME, nil
	default:
		return nil, "", fmt.Errorf("ingress has neither IPs nor hostnames")
	}
}

// DNSValuesFromNodes computes the values and record type for a DNSRecord from the given node addresses: each node
// contributes its first address in the order of addressTypePreference (first family with at least one match wins).
func DNSValuesFromNodes(nodes []corev1.Node, ipFamilies []gardencorev1beta1.IPFamily, addressTypePreference ...corev1.NodeAddressType) ([]string, extensionsv1alpha1.DNSRecordType, error) {
	for i := range nodes {
		if len(nodes[i].Status.Addresses) == 0 {
			return nil, "", fmt.Errorf("node %q has no addresses", nodes[i].Name)
		}
	}

	for _, family := range ipFamilies {
		recordType := IPFamilyToDNSRecordType(family)

		var values []string
		for _, node := range nodes {
			if address := preferredAddress(node.Status.Addresses, recordType, addressTypePreference); address != "" {
				values = append(values, address)
			}
		}

		if len(values) > 0 {
			return values, recordType, nil
		}
	}

	return nil, "", fmt.Errorf("no node address of types %v matches a configured IP family %v", addressTypePreference, ipFamilies)
}

// IPFamilyToDNSRecordType maps a Gardener IP family to the corresponding DNSRecord type.
func IPFamilyToDNSRecordType(family gardencorev1beta1.IPFamily) extensionsv1alpha1.DNSRecordType {
	if family == gardencorev1beta1.IPFamilyIPv6 {
		return extensionsv1alpha1.DNSRecordTypeAAAA
	}
	return extensionsv1alpha1.DNSRecordTypeA
}

// preferredAddress returns the first address of the given record type in the order of addressTypePreference, or "".
func preferredAddress(addresses []corev1.NodeAddress, recordType extensionsv1alpha1.DNSRecordType, addressTypePreference []corev1.NodeAddressType) string {
	for _, addressType := range addressTypePreference {
		for _, address := range addresses {
			if address.Type == addressType && GetDNSRecordType(address.Address) == recordType {
				return address.Address
			}
		}
	}
	return ""
}

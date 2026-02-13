// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnsrecord

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/go-logr/logr"
	"github.com/miekg/dns"
	"k8s.io/utils/ptr"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

const (
	// dnsZone is the DNS zone managed by the local BIND9 server.
	dnsZone = "local.gardener.cloud."
	// dnsServerAddress is the address of the local BIND9 server.
	dnsServerAddress = "ns.local.gardener.cloud:53"
	// defaultTTL is the default TTL for DNS records when none is specified.
	defaultTTL = 120
)

// DNSClient is an interface for sending DNS messages, allowing for testability.
type DNSClient interface {
	Exchange(msg *dns.Msg, addr string) (*dns.Msg, time.Duration, error)
}

// Actuator implements the DNSRecord actuator for the local DNS provider.
type Actuator struct {
	DNSClient DNSClient
}

// Reconcile ensures that the DNS record is correctly represented via RFC 2136 dynamic DNS update.
func (a *Actuator) Reconcile(_ context.Context, _ logr.Logger, dnsRecord *extensionsv1alpha1.DNSRecord, _ *extensionscontroller.Cluster) error {
	header, err := headerForDNSRecord(dnsRecord)
	if err != nil {
		return fmt.Errorf("failed to create header for %s: %w", dnsRecord.Spec.Name, err)
	}

	resourceRecords, err := resourceRecordsForDNSRecord(dnsRecord, header)
	if err != nil {
		return fmt.Errorf("failed to create resource records for %s: %w", dnsRecord.Spec.Name, err)
	}

	msg := (&dns.Msg{}).SetUpdate(dnsZone)
	// Remove the existing record set for this name and type before inserting the new one to ensure removing any orphaned
	// values. This is how RFC 2136 is designed to be used for updating values of existing records.
	msg.RemoveRRset([]dns.RR{&dns.ANY{Hdr: header}})
	// Set the new record values.
	msg.Insert(resourceRecords)

	return a.sendDNSUpdate(msg)
}

// Delete removes the DNS record via RFC 2136 dynamic DNS update.
func (a *Actuator) Delete(_ context.Context, _ logr.Logger, dnsRecord *extensionsv1alpha1.DNSRecord, _ *extensionscontroller.Cluster) error {
	header, err := headerForDNSRecord(dnsRecord)
	if err != nil {
		return fmt.Errorf("failed to create header for %s: %w", dnsRecord.Spec.Name, err)
	}

	msg := (&dns.Msg{}).SetUpdate(dnsZone)
	msg.RemoveRRset([]dns.RR{&dns.ANY{Hdr: header}})

	return a.sendDNSUpdate(msg)
}

// ForceDelete is the same as Delete for the local DNS provider.
func (a *Actuator) ForceDelete(ctx context.Context, log logr.Logger, dnsRecord *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) error {
	return a.Delete(ctx, log, dnsRecord, cluster)
}

// Migrate removes the DNS record if the shoot is not self-hosted.
func (a *Actuator) Migrate(ctx context.Context, log logr.Logger, dnsRecord *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) error {
	if v1beta1helper.IsShootSelfHosted(cluster.Shoot.Spec.Provider.Workers) {
		// Do nothing when migrating DNSRecord of self-hosted shoot with managed infrastructure. The DNS
		// records are still needed for the control plane machines to resolve the kube-apiserver domain.
		return nil
	}

	return a.Delete(ctx, log, dnsRecord, cluster)
}

// Restore is the same as Reconcile for the local DNS provider.
func (a *Actuator) Restore(ctx context.Context, log logr.Logger, dnsRecord *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) error {
	return a.Reconcile(ctx, log, dnsRecord, cluster)
}

func (a *Actuator) sendDNSUpdate(msg *dns.Msg) error {
	resp, _, err := a.DNSClient.Exchange(msg, dnsServerAddress)
	if err != nil {
		return fmt.Errorf("DNS update exchange failed: %w", err)
	}
	if resp.Rcode != dns.RcodeSuccess {
		return fmt.Errorf("DNS update failed with rcode: %s", dns.RcodeToString[resp.Rcode])
	}

	return nil
}

func headerForDNSRecord(dnsRecord *extensionsv1alpha1.DNSRecord) (dns.RR_Header, error) {
	recordType, err := dnsTypeForRecordType(dnsRecord.Spec.RecordType)
	if err != nil {
		return dns.RR_Header{}, err
	}

	return dns.RR_Header{Name: dns.Fqdn(dnsRecord.Spec.Name), Rrtype: recordType, Class: dns.ClassINET}, nil
}

func dnsTypeForRecordType(recordType extensionsv1alpha1.DNSRecordType) (uint16, error) {
	switch recordType {
	case extensionsv1alpha1.DNSRecordTypeA:
		return dns.TypeA, nil
	case extensionsv1alpha1.DNSRecordTypeAAAA:
		return dns.TypeAAAA, nil
	case extensionsv1alpha1.DNSRecordTypeCNAME:
		return dns.TypeCNAME, nil
	}

	return 0, fmt.Errorf("unsupported DNS record type: %s", recordType)
}

func resourceRecordsForDNSRecord(dnsRecord *extensionsv1alpha1.DNSRecord, header dns.RR_Header) ([]dns.RR, error) {
	// header is passed by value (no pointer type) and thus copied for each resource record, so we can safely modify the
	// TTL here without affecting other records.
	// #nosec G115 -- TTL validated at admission time
	header.Ttl = uint32(ptr.Deref(dnsRecord.Spec.TTL, defaultTTL))

	resourceRecords := make([]dns.RR, len(dnsRecord.Spec.Values))
	for i, value := range dnsRecord.Spec.Values {
		resourceRecord, err := resourceRecordForValue(dnsRecord.Spec.RecordType, header, value)
		if err != nil {
			return nil, fmt.Errorf("failed to create resource record for value %s: %w", value, err)
		}
		resourceRecords[i] = resourceRecord
	}

	return resourceRecords, nil
}

func resourceRecordForValue(recordType extensionsv1alpha1.DNSRecordType, header dns.RR_Header, value string) (dns.RR, error) {
	switch recordType {
	case extensionsv1alpha1.DNSRecordTypeA:
		ip := net.ParseIP(value)
		if ip == nil || ip.To4() == nil {
			return nil, fmt.Errorf("invalid IPv4 address: %s", value)
		}
		return &dns.A{Hdr: header, A: ip.To4()}, nil

	case extensionsv1alpha1.DNSRecordTypeAAAA:
		ip := net.ParseIP(value)
		if ip == nil || ip.To16() == nil {
			return nil, fmt.Errorf("invalid IPv6 address: %s", value)
		}
		return &dns.AAAA{Hdr: header, AAAA: ip.To16()}, nil

	case extensionsv1alpha1.DNSRecordTypeCNAME:
		return &dns.CNAME{Hdr: header, Target: dns.Fqdn(value)}, nil
	}

	return nil, fmt.Errorf("unsupported DNS record type: %s", recordType)
}

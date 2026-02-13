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
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
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
	fqdn := dns.Fqdn(dnsRecord.Spec.Name)
	ttl := uint32(ptr.Deref(dnsRecord.Spec.TTL, defaultTTL)) // #nosec G115 -- TTL validated at admission time
	rrType := dnsTypeForRecordType(dnsRecord.Spec.RecordType)

	msg := new(dns.Msg)
	msg.SetUpdate(dnsZone)

	// Remove existing RRset for this name and type (clean slate).
	rrsetRemove := new(dns.RR_Header)
	rrsetRemove.Name = fqdn
	rrsetRemove.Rrtype = rrType
	rrsetRemove.Class = dns.ClassANY
	rrsetRemove.Ttl = 0
	rrsetRemove.Rdlength = 0
	msg.RemoveRRset([]dns.RR{&dns.ANY{Hdr: *rrsetRemove}})

	// Insert new records.
	for _, value := range dnsRecord.Spec.Values {
		rr, err := newRR(fqdn, rrType, ttl, value)
		if err != nil {
			return fmt.Errorf("failed to create RR for %s: %w", fqdn, err)
		}
		msg.Insert([]dns.RR{rr})
	}

	return a.sendDNSUpdate(msg)
}

// Delete removes the DNS record via RFC 2136 dynamic DNS update.
func (a *Actuator) Delete(_ context.Context, _ logr.Logger, dnsRecord *extensionsv1alpha1.DNSRecord, _ *extensionscontroller.Cluster) error {
	fqdn := dns.Fqdn(dnsRecord.Spec.Name)

	msg := new(dns.Msg)
	msg.SetUpdate(dnsZone)
	msg.RemoveName([]dns.RR{&dns.ANY{Hdr: dns.RR_Header{Name: fqdn, Rrtype: dns.TypeANY, Class: dns.ClassANY, Ttl: 0, Rdlength: 0}}})

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

func dnsTypeForRecordType(recordType extensionsv1alpha1.DNSRecordType) uint16 {
	switch recordType {
	case extensionsv1alpha1.DNSRecordTypeA:
		return dns.TypeA
	case extensionsv1alpha1.DNSRecordTypeAAAA:
		return dns.TypeAAAA
	case extensionsv1alpha1.DNSRecordTypeCNAME:
		return dns.TypeCNAME
	default:
		return dns.TypeA
	}
}

func newRR(fqdn string, rrType uint16, ttl uint32, value string) (dns.RR, error) {
	hdr := dns.RR_Header{Name: fqdn, Rrtype: rrType, Class: dns.ClassINET, Ttl: ttl}

	switch rrType {
	case dns.TypeA:
		ip := net.ParseIP(value)
		if ip == nil {
			return nil, fmt.Errorf("invalid IP address: %s", value)
		}
		return &dns.A{Hdr: hdr, A: ip.To4()}, nil
	case dns.TypeAAAA:
		ip := net.ParseIP(value)
		if ip == nil {
			return nil, fmt.Errorf("invalid IP address: %s", value)
		}
		return &dns.AAAA{Hdr: hdr, AAAA: ip.To16()}, nil
	case dns.TypeCNAME:
		return &dns.CNAME{Hdr: hdr, Target: dns.Fqdn(value)}, nil
	default:
		return nil, fmt.Errorf("unsupported DNS record type: %d", rrType)
	}
}

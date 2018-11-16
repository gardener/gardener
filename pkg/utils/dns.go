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

package utils

import (
	"errors"
	"fmt"
	"net"
	"sort"
	"time"

	"github.com/miekg/dns"

	"github.com/gardener/gardener/pkg/logger"
)

// LookupDNSHost performs a DNS lookup for the given <domain>. In case of success, it will return the list
// of records. If the domain is not resolvable, it will return nil.
func LookupDNSHost(domain string) ([]string, error) {
	if net.ParseIP(domain) != nil {
		return nil, fmt.Errorf("Detected misuse of domain lookup, an IP was used instead of a domain A record")
	}

	nsRecords, err := lookUPDNSRecord(domain, dns.TypeA)
	if err != nil {
		return nil, err
	}

	sort.Strings(nsRecords)

	return nsRecords, nil
}

// LookupDNSHostCNAME performs a CNAME DNS lookup for the given <domain>. In case of success, it will return
// the record. If the domain is not resolvable (or is not of type CNAME), it will return an empty string.
func LookupDNSHostCNAME(domain string) (string, error) {
	nsRecords, err := lookUPDNSRecord(domain, dns.TypeCNAME)
	if err != nil {
		return "", err
	}

	// A CNAME Query returns a string slice with a length of 1, so we get the CNAME from the slice
	return nsRecords[0], nil
}

// dnsQuery initializes the client as well as the message for DNS and initiates the DNS Query
func dnsQuery(queryName string, queryType uint16) (*dns.Msg, error) {
	localMessage := &dns.Msg{
		MsgHdr: dns.MsgHdr{
			RecursionDesired: true,
		},
		Question: make([]dns.Question, 1),
	}

	localClient := &dns.Client{
		ReadTimeout: 5 * time.Second,
	}

	conf, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil || conf == nil {
		return nil, fmt.Errorf("config file might be empty, or something went wrong with initiating client: %s", err)
	}

	localMessage.SetQuestion(queryName, queryType)

	for _, server := range conf.Servers {
		r, _, err := localClient.Exchange(localMessage, fmt.Sprintf("%s:%s", server, conf.Port))
		if err != nil {
			return nil, err
		}
		if r == nil || r.Rcode == dns.RcodeNameError || r.Rcode == dns.RcodeSuccess {
			return r, err
		}
	}
	return nil, errors.New("No name server to answer the question")
}

// lookUPDNSRecord filters results from a dnsQuery and returns a slice of valid RR records or error in case of failure
func lookUPDNSRecord(domain string, qtype uint16) ([]string, error) {
	if net.ParseIP(domain) != nil {
		return nil, fmt.Errorf("Detected misuse of domain lookup, an IP was used instead of a domain CNAME")
	}

	r, err := dnsQuery(dns.Fqdn(domain), qtype)
	if err != nil || r == nil {
		return nil, fmt.Errorf("cannot retrieve the list of name servers for %s: %+v", dns.Fqdn(domain), err)
	}

	if qtype == dns.TypeCNAME && r.Rcode == dns.RcodeNameError {
		return nil, fmt.Errorf("no such domain %s", dns.Fqdn(domain))
	}

	if len(r.Answer) == 0 {
		return nil, fmt.Errorf("empty answer query response from server, the lookup type for the records might be incorrect")
	}

	var recordsToReturn []string
	for _, record := range r.Answer {
		switch t := record.(type) {
		case *dns.A:
			recordsToReturn = append(recordsToReturn, t.A.String())
		case *dns.CNAME:
			recordsToReturn = append(recordsToReturn, t.Target)
		default:
			logger.Logger.Warn("Invalid answer record received doing nothing")
		}
	}

	return recordsToReturn, nil
}

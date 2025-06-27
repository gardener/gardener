// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package publicip

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
)

// Detector can detect the system's public IPs.
type Detector interface {
	// DetectPublicIPs returns the system's public IPs (IPv4 and/or IPv6 as available) or an error if none or both can be
	// detected.
	DetectPublicIPs(ctx context.Context, log logr.Logger) ([]net.IP, error)
}

// IpifyDetector implements Detector using https://ipify.org/.
type IpifyDetector struct {
	// Client is the http client to use, defaults to http.DefaultClient.
	Client *http.Client
}

// DetectPublicIPs tries both api4.ipify.org and api6.ipify.org. If both return an error, the result is an empty slice
// and a combined error. Otherwise, it returns either or both detected IP addresses.
func (i IpifyDetector) DetectPublicIPs(ctx context.Context, parentLog logr.Logger) ([]net.IP, error) {
	var (
		ips  []net.IP
		errs error
	)

	for ipFamily, host := range map[string]string{
		"IPv4": "api4.ipify.org",
		"IPv6": "api6.ipify.org",
	} {
		log := parentLog.WithValues("ipFamily", ipFamily)

		if ip, err := i.getPublicIP(ctx, log, ipFamily, host); err != nil {
			log.V(1).Info("No public IP detected or detection failed", "err", err)
			errs = multierror.Append(errs, err)
		} else {
			log.Info("Detected public IP", "ip", ip)
			ips = append(ips, ip)
		}
	}

	// if we couldn't detect any IP address, return the combined error
	// otherwise, ignore a single error in favor of the successfully detected address
	if len(ips) == 0 {
		return nil, errs
	}

	return ips, nil
}

func (i IpifyDetector) getPublicIP(parentCtx context.Context, log logr.Logger, ipFamily, host string) (net.IP, error) {
	ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+host+"/", nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	log.V(1).Info("Trying to determine public IP address", "url", req.URL)

	if i.Client == nil {
		i.Client = http.DefaultClient
	}

	resp, err := i.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error determining public %s address: %w", ipFamily, err)
	}
	defer resp.Body.Close()

	ipBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	ip := net.ParseIP(string(ipBytes))
	if ip == nil {
		return nil, fmt.Errorf("error parsing detected %s address: %q", ipFamily, ipBytes)
	}
	return ip, nil
}

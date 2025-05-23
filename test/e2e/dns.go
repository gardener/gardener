// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"net"
	"time"
)

// UseProviderLocalCoreDNSServer sets the golang DefaultResolver to the CoreDNS server deployed as part of the
// provider-local extension. This is port forwarded to the host on 172.18.255.1:5353. The tests can use this in-cluster
// CoreDNS server for name resolution and can therefore resolve the API endpoint of shoot clusters to the correct Istio
// instance (non-HA shoots are not exposed via 172.18.255.1 but via 172.18.255.{10,11,12}).
func UseProviderLocalCoreDNSServer() {
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			// We use tcp to distinguish easily in-cluster requests (done via udp) and requests from the tests (using
			// tcp). The result for cluster api names differ depending on the source.
			return (&net.Dialer{
				Timeout: time.Duration(5) * time.Second,
			}).DialContext(ctx, "tcp", "172.18.255.1:5353")
		},
	}
}

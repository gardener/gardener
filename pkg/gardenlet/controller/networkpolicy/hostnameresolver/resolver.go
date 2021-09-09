// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package hostnameresolver

import (
	"context"
	"net"
	"net/url"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type resolver struct {
	lock          sync.RWMutex
	upstreamFQDN  string
	upstreamPort  int32
	refreshTicker *time.Ticker
	onUpdate      func()
	log           logrus.FieldLogger
	addrs         []string
	// used for testing
	lookup lookup
}

type noOpResover struct{}

type lookup interface {
	LookupHost(ctx context.Context, host string) (addrs []string, err error)
}

// Provider allows to start and attach callbacks for a specific host
// resolution updates.
type Provider interface {
	HasSynced() bool
	Start(ctx context.Context)
	WithCallback(onUpdate func())
	HostResolver
}

// HostResolver is used for getting endpoint subsets with resolved IPs.
type HostResolver interface {
	Subset() []corev1.EndpointSubset
}

// NewProvider returns a Provider for a specific host and port with resync
// indicating how often the hostname resolution is happening.
func NewProvider(host string, port string, log logrus.FieldLogger, resync time.Duration) Provider {
	return &resolver{
		upstreamFQDN:  host,
		upstreamPort:  intstr.Parse(port).IntVal,
		refreshTicker: time.NewTicker(resync),
		log:           log,
		lookup:        net.DefaultResolver,
	}
}

// HasSynced returns true if ip addresses are exposed.
func (l *resolver) HasSynced() bool {
	l.lock.Lock()
	defer l.lock.Unlock()

	return len(l.addrs) > 0
}

// Start waits for stopCtx to be done and resolves the upstream
// hostname every resync period.
// Updates are send if returned hosts are changed.
func (l *resolver) Start(stopCtx context.Context) {
	updateFunc := func() {
		addresses, err := l.lookup.LookupHost(stopCtx, l.upstreamFQDN)
		if err != nil {
			l.log.WithField("error", err).Errorln("could not resolve upstream hostname")

			return
		}

		sort.Strings(addresses)

		l.lock.Lock()
		updated := !equal(addresses, l.addrs)

		if updated {
			l.addrs = addresses

			l.log.WithField("resolvedIPs", l.addrs).Infoln("updated resolved addresses")

			if l.onUpdate != nil {
				l.onUpdate()
			}
		}

		l.lock.Unlock()
	}

	// start the update in the beginning
	updateFunc()

	for {
		select {
		case <-l.refreshTicker.C:
			updateFunc()
		case <-stopCtx.Done():
			l.refreshTicker.Stop()
			l.log.Infoln("stopping periodic hostname resolution")

			return
		}
	}
}

// Subset returns a slice of resolved ip addresses.
func (l *resolver) Subset() []corev1.EndpointSubset {
	l.lock.RLock()
	defer l.lock.RUnlock()

	subset := []corev1.EndpointSubset{}

	if len(l.addrs) > 0 {
		s := corev1.EndpointSubset{
			Ports: []corev1.EndpointPort{
				{
					Port:     l.upstreamPort,
					Protocol: corev1.ProtocolTCP,
				},
			},
			Addresses: make([]corev1.EndpointAddress, 0, len(l.addrs)),
		}
		for _, addr := range l.addrs {
			s.Addresses = append(s.Addresses, corev1.EndpointAddress{IP: addr})
		}

		subset = append(subset, s)
	}

	return subset
}

// WithCallback calls onUpdate function when resolved IPs are changed.
func (l *resolver) WithCallback(onUpdate func()) {
	l.onUpdate = onUpdate
}

// CreateForCluster tries to use the hostname and port from the client to
// create the provider. If that fails, then tries to use the
// KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT environment variable.
// If that fails it fallbacks to NoOpProvider().
func CreateForCluster(client kubernetes.Interface, logger logrus.FieldLogger) (Provider, error) {
	u, err := url.Parse(client.RESTConfig().Host)
	if err != nil {
		return nil, err
	}

	var (
		serverHostname       = u.Hostname()
		providerLogger       = logger.WithField("hostname", serverHostname)
		envHostname, envPort = os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
		port                 = "443"
	)

	if net.ParseIP(serverHostname) == nil {
		if p := u.Port(); p != "" {
			port = p
		}

		providerLogger.Infoln("using hostname resolver")

		return NewProvider(
			serverHostname,
			port,
			providerLogger,
			time.Second*30,
		), nil
	} else if envHostname != "" &&
		envPort != "" &&
		net.ParseIP(envHostname) == nil {
		providerLogger.Infoln("fallback to environment variable hostname resolver")

		return NewProvider(
			envHostname,
			envPort,
			providerLogger,
			time.Second*30,
		), nil
	}

	providerLogger.Infoln("using no-op hostname resolver")

	return NewNoOpProvider(), nil
}

// NewNoOpProvider returns a no-op Provider.
func NewNoOpProvider() Provider { return &noOpResover{} }

// HasSynced always returns true.
func (*noOpResover) HasSynced() bool { return true }

// Start does nothing.
func (*noOpResover) Start(_ context.Context) {}

// Subset returns an empty slice.
func (*noOpResover) Subset() []corev1.EndpointSubset { return []corev1.EndpointSubset{} }

// WithCallback does nothing.
func (*noOpResover) WithCallback(_ func()) {}

func equal(a, b []string) bool {
	if (a == nil) != (b == nil) {
		return false
	}

	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

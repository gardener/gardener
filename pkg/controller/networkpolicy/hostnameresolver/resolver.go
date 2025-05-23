// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package hostnameresolver

import (
	"context"
	"net"
	"net/url"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/rest"
)

type resolver struct {
	lock          sync.RWMutex
	upstreamFQDN  string
	upstreamPort  int32
	refreshTicker *time.Ticker
	onUpdate      func()
	log           logr.Logger
	addrs         []string
	// used for testing
	lookup lookup
}

type noOpResolver struct{}

type lookup interface {
	LookupHost(ctx context.Context, host string) (addrs []string, err error)
}

// Provider allows to start and attach callbacks for a specific host
// resolution updates.
type Provider interface {
	HasSynced() bool
	Start(ctx context.Context) error
	WithCallback(onUpdate func())
	HostResolver
}

// HostResolver is used for getting endpoint subsets with resolved IPs.
type HostResolver interface {
	HasSynced() bool
	Subset() []corev1.EndpointSubset
}

// NewProvider returns a Provider for a specific host and port with resync
// indicating how often the hostname resolution is happening.
func NewProvider(host string, port string, log logr.Logger, resync time.Duration) Provider {
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
func (l *resolver) Start(stopCtx context.Context) error {
	updateFunc := func() {
		addresses, err := l.lookup.LookupHost(stopCtx, l.upstreamFQDN)
		if err != nil {
			l.log.Error(err, "Could not resolve upstream hostname")
			return
		}

		slices.Sort(addresses)

		l.lock.Lock()
		updated := !equal(addresses, l.addrs)

		if updated {
			l.addrs = addresses
			l.log.Info("Updated resolved addressed", "resolvedIPs", l.addrs)

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
			l.log.Info("Stopping periodic hostname resolution")

			return nil
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
func CreateForCluster(restConfig *rest.Config, log logr.Logger) (Provider, error) {
	u, err := url.Parse(restConfig.Host)
	if err != nil {
		return nil, err
	}

	var (
		serverHostname       = u.Hostname()
		providerLogger       = log.WithValues("hostname", serverHostname)
		envHostname, envPort = os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
		port                 = "443"
	)

	if net.ParseIP(serverHostname) == nil {
		if p := u.Port(); p != "" {
			port = p
		}

		providerLogger.Info("Using hostname resolver")

		return NewProvider(
			serverHostname,
			port,
			providerLogger,
			time.Second*30,
		), nil
	} else if envHostname != "" &&
		envPort != "" &&
		net.ParseIP(envHostname) == nil {
		providerLogger.Info("Fallback to environment variable hostname resolver")

		return NewProvider(
			envHostname,
			envPort,
			providerLogger,
			time.Second*30,
		), nil
	}

	providerLogger.Info("Using no-op hostname resolver")
	return NewNoOpProvider(), nil
}

// NewNoOpProvider returns a no-op Provider.
func NewNoOpProvider() Provider { return &noOpResolver{} }

// HasSynced always returns true.
func (*noOpResolver) HasSynced() bool { return true }

// Start does nothing.
func (*noOpResolver) Start(_ context.Context) error {
	return nil
}

// Subset returns an empty slice.
func (*noOpResolver) Subset() []corev1.EndpointSubset { return []corev1.EndpointSubset{} }

// WithCallback does nothing.
func (*noOpResolver) WithCallback(_ func()) {}

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

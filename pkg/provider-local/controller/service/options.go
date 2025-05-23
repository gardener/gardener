// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"github.com/spf13/pflag"

	"github.com/gardener/gardener/extensions/pkg/controller/cmd"
)

// ControllerOptions are command line options that can be set for controller.Options.
type ControllerOptions struct {
	// MaxConcurrentReconciles are the maximum concurrent reconciles.
	MaxConcurrentReconciles int
	// HostIP is the host ip.
	HostIP string
	// Zone0IP is the IP address to be used for the zone 0 istio ingress gateway.
	Zone0IP string
	// Zone1IP is the IP address to be used for the zone 1 istio ingress gateway.
	Zone1IP string
	// Zone2IP is the IP address to be used for the zone 2 istio ingress gateway.
	Zone2IP string

	config *ControllerConfig
}

// AddFlags implements Flagger.AddFlags.
func (c *ControllerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.IntVar(&c.MaxConcurrentReconciles, cmd.MaxConcurrentReconcilesFlag, c.MaxConcurrentReconciles, "The maximum number of concurrent reconciliations.")
	fs.StringVar(&c.HostIP, "host-ip", c.HostIP, "Overwrite Host IP to use for kube-apiserver service LoadBalancer")
	fs.StringVar(&c.Zone0IP, "zone-0-ip", c.Zone0IP, "Overwrite IP to use for kube-apiserver service LoadBalancer in zone 0")
	fs.StringVar(&c.Zone1IP, "zone-1-ip", c.Zone1IP, "Overwrite IP to use for kube-apiserver service LoadBalancer in zone 1")
	fs.StringVar(&c.Zone2IP, "zone-2-ip", c.Zone2IP, "Overwrite IP to use for kube-apiserver service LoadBalancer in zone 2")
}

// Complete implements Completer.Complete.
func (c *ControllerOptions) Complete() error {
	c.config = &ControllerConfig{c.MaxConcurrentReconciles, c.HostIP, c.Zone0IP, c.Zone1IP, c.Zone2IP}
	return nil
}

// Completed returns the completed ControllerConfig. Only call this if `Complete` was successful.
func (c *ControllerOptions) Completed() *ControllerConfig {
	return c.config
}

// ControllerConfig is a completed controller configuration.
type ControllerConfig struct {
	// MaxConcurrentReconciles is the maximum number of concurrent reconciles.
	MaxConcurrentReconciles int
	// HostIP is the host ip.
	HostIP string
	// Zone0IP is the IP address to be used for the zone 0 istio ingress gateway.
	Zone0IP string
	// Zone1IP is the IP address to be used for the zone 1 istio ingress gateway.
	Zone1IP string
	// Zone2IP is the IP address to be used for the zone 2 istio ingress gateway.
	Zone2IP string
}

// Apply sets the values of this ControllerConfig in the given AddOptions.
func (c *ControllerConfig) Apply(opts *AddOptions) {
	opts.Controller.MaxConcurrentReconciles = c.MaxConcurrentReconciles
	opts.HostIP = c.HostIP
	opts.Zone0IP = c.Zone0IP
	opts.Zone1IP = c.Zone1IP
	opts.Zone2IP = c.Zone2IP
}

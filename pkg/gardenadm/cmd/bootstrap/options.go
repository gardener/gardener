// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/spf13/pflag"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	"github.com/gardener/gardener/pkg/utils/publicip"
)

// Options contains options for this command.
type Options struct {
	*cmd.Options
	cmd.ManifestOptions
	// Kubeconfig is the path to the kubeconfig file pointing to the KinD cluster.
	Kubeconfig string
	// BastionIngressCIDRs is a list of CIDRs to be allowed for accessing the created bastion host.
	// If not given, the system's public IPs are detected using PublicIPDetector.
	BastionIngressCIDRs []string

	// exposed for testing, defaults to publicip.IpifyDetector
	PublicIPDetector publicip.Detector
}

// ParseArgs parses the arguments to the options.
func (o *Options) ParseArgs(args []string) error {
	if o.Kubeconfig == "" {
		o.Kubeconfig = os.Getenv("KUBECONFIG")
	}

	return o.ManifestOptions.ParseArgs(args)
}

// Validate validates the options.
func (o *Options) Validate() error {
	if len(o.Kubeconfig) == 0 {
		return fmt.Errorf("must provide a path to a KinD cluster kubeconfig")
	}

	for _, cidr := range o.BastionIngressCIDRs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("invalid bastion ingress CIDR %q: %w", cidr, err)
		}
	}

	return o.ManifestOptions.Validate()
}

// Complete completes the options.
func (o *Options) Complete() error {
	if len(o.BastionIngressCIDRs) == 0 {
		o.Log.Info("Auto-detecting public IP addresses. If this does not work, set --bastion-ingress-cidr instead")

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		ips, err := o.PublicIPDetector.DetectPublicIPs(ctx, o.Log.WithName("public-ip-detector"))
		if err != nil {
			return fmt.Errorf("error detecting public IP addresses: %w", err)
		}

		for _, ip := range ips {
			cidr := net.IPNet{IP: ip}
			if ip.To4() != nil {
				cidr.Mask = net.CIDRMask(32, 32) // use /32 for IPv4
			} else {
				cidr.Mask = net.CIDRMask(128, 128) // use /128 for IPv6
			}
			o.BastionIngressCIDRs = append(o.BastionIngressCIDRs, cidr.String())
		}

		o.Log.Info("Using auto-detected public IP addresses as bastion ingress CIDRs", "cidrs", o.BastionIngressCIDRs)
	}

	return o.ManifestOptions.Complete()
}

func (o *Options) addFlags(fs *pflag.FlagSet) {
	o.ManifestOptions.AddFlags(fs)
	fs.StringVarP(&o.Kubeconfig, "kubeconfig", "k", "", "Path to the kubeconfig file pointing to the KinD cluster")
	fs.StringSliceVar(&o.BastionIngressCIDRs, "bastion-ingress-cidr", nil, "Restrict bastion host ingress to the given CIDRs. Defaults to your system's public IPs (IPv4 and/or IPv6) as detected using https://ipify.org/.")
}

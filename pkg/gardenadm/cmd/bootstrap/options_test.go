// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrap_test

import (
	"context"
	"fmt"
	"io"
	"net"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	. "github.com/gardener/gardener/pkg/gardenadm/cmd/bootstrap"
	"github.com/gardener/gardener/pkg/logger"
)

var _ = Describe("Options", func() {
	var (
		options *Options

		logBuffer *gbytes.Buffer
		detector  *fakeDetector
	)

	BeforeEach(func() {
		detector = &fakeDetector{
			ips: []net.IP{net.ParseIP("1.2.3.4"), net.ParseIP("2001:db8::1")},
		}

		logBuffer = gbytes.NewBuffer()
		options = &Options{
			Options: &cmd.Options{
				Log: logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(io.MultiWriter(logBuffer, GinkgoWriter))),
			},
			ManifestOptions: cmd.ManifestOptions{
				ConfigDir: "some-path-to-config-dir",
			},
			Kubeconfig:       "some-path-to-kubeconfig",
			PublicIPDetector: detector,
		}
	})

	Describe("#ParseArgs", func() {
		It("should return nil", func() {
			Expect(options.ParseArgs(nil)).To(Succeed())
		})
	})

	Describe("#Validate", func() {
		It("should pass for valid options", func() {
			Expect(options.Validate()).To(Succeed())
		})

		It("should fail because kubeconfig path is not set", func() {
			options.Kubeconfig = ""
			Expect(options.Validate()).To(MatchError(ContainSubstring("must provide a path to a KinD cluster kubeconfig")))
		})

		It("should fail because config dir path is not set", func() {
			options.ConfigDir = ""
			Expect(options.Validate()).To(MatchError(ContainSubstring("must provide a path to a config directory")))
		})

		Describe("bastion ingress CIDRs", func() {
			It("should succeed without CIDRs", func() {
				options.BastionIngressCIDRs = nil
				Expect(options.Validate()).To(Succeed())
			})

			It("should succeed with valid CIDRs", func() {
				options.BastionIngressCIDRs = []string{"4.3.2.1/32", "2001:db8::2/128"}
				Expect(options.Validate()).To(Succeed())
			})

			It("should fail with invalid CIDRs", func() {
				options.BastionIngressCIDRs = []string{"4.3.2.1/32", "1.2.3.4/128"}
				Expect(options.Validate()).To(MatchError(ContainSubstring("invalid CIDR address: 1.2.3.4/128")))

				options.BastionIngressCIDRs = []string{"4.3.2.1/32", "2001:db8::1/invalid"}
				Expect(options.Validate()).To(MatchError(ContainSubstring("invalid CIDR address: 2001:db8::1/invalid")))
			})
		})
	})

	Describe("#Complete", func() {
		It("should return nil", func() {
			Expect(options.Complete()).To(Succeed())
		})

		Describe("bastion ingress CIDRs", func() {
			It("should keep the configured CIDRs", func() {
				options.BastionIngressCIDRs = []string{"4.3.2.1/32", "2001:db8::2/128"}
				Expect(options.Complete()).To(Succeed())
				Expect(options.BastionIngressCIDRs).To(ConsistOf("4.3.2.1/32", "2001:db8::2/128"))
				Consistently(logBuffer).ShouldNot(gbytes.Say("Auto-detecting public IP addresses"))
			})

			It("should use auto-detected IP addresses", func() {
				options.BastionIngressCIDRs = nil
				Expect(options.Complete()).To(Succeed())
				Expect(options.BastionIngressCIDRs).To(ConsistOf("1.2.3.4/32", "2001:db8::1/128"))
				Eventually(logBuffer).Should(gbytes.Say("Using auto-detected public IP addresses"))
			})

			It("should fail if IP address detection fails", func() {
				options.BastionIngressCIDRs = nil
				detector.err = fmt.Errorf("foo")
				Expect(options.Complete()).To(MatchError(ContainSubstring("foo")))
				Eventually(logBuffer).Should(gbytes.Say("Auto-detecting public IP addresses"))
			})
		})
	})
})

type fakeDetector struct {
	ips []net.IP
	err error
}

func (f *fakeDetector) DetectPublicIPs(context.Context, logr.Logger) ([]net.IP, error) {
	return f.ips, f.err
}

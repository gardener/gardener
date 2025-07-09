// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package publicip_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"

	. "github.com/gardener/gardener/pkg/utils/publicip"
)

var _ = Describe("IpifyDetector", func() {
	const (
		ipv4 = "1.2.3.4"
		ipv6 = "2001:db8::1"
	)

	var (
		ctx = context.Background()
		log = logr.Discard()
		api *fakeRoundTripper

		detector IpifyDetector
	)

	BeforeEach(func() {
		api = &fakeRoundTripper{
			ipv4: apiResponse{ip: ipv4},
			ipv6: apiResponse{ip: ipv6},
		}

		detector = IpifyDetector{
			Client: &http.Client{
				Transport: api,
			},
		}
	})

	When("both address families are available", func() {
		It("should return all public IPs", func() {
			Expect(detector.DetectPublicIPs(ctx, log)).To(toStrings(ContainElements(ipv4, ipv6)))
		})
	})

	When("only IPv4 is available", func() {
		BeforeEach(func() {
			api.ipv6.err = syscall.EHOSTUNREACH
		})

		It("should return only the available IP", func() {
			Expect(detector.DetectPublicIPs(ctx, log)).To(toStrings(ContainElements(ipv4)))
		})
	})

	When("only IPv6 is available", func() {
		BeforeEach(func() {
			api.ipv4.err = syscall.EHOSTDOWN
		})

		It("should return only the available IP", func() {
			Expect(detector.DetectPublicIPs(ctx, log)).To(toStrings(ContainElements(ipv6)))
		})
	})

	When("no public IP is available", func() {
		BeforeEach(func() {
			api.ipv4.err = fmt.Errorf("foo")
			api.ipv6.err = context.DeadlineExceeded
		})

		It("should return only the available IP", func() {
			Expect(detector.DetectPublicIPs(ctx, log)).Error().To(MatchError(And(
				MatchRegexp(`error determining public IPv4 address: .*: foo`),
				MatchRegexp(`error determining public IPv6 address: .*: context deadline exceeded`),
			)))
		})
	})

	When("the API returns invalid IPs", func() {
		BeforeEach(func() {
			api.ipv4.ip = "foo"
			api.ipv6.ip = "bar"
		})

		It("should return an error", func() {
			Expect(detector.DetectPublicIPs(ctx, log)).Error().To(MatchError(And(
				ContainSubstring(`detected IPv4 address: "foo"`),
				ContainSubstring(`detected IPv6 address: "bar"`),
			)))
		})
	})
})

type apiResponse struct {
	ip  string
	err error
}

type fakeRoundTripper struct {
	ipv4, ipv6 apiResponse
}

func (f *fakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var res apiResponse
	switch req.URL.String() {
	case "https://api4.ipify.org/":
		res = f.ipv4
	case "https://api6.ipify.org/":
		res = f.ipv6
	default:
		return nil, fmt.Errorf("unexpected URL: %s", req.URL)
	}

	if res.err != nil {
		return nil, res.err
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(res.ip)),
	}, nil
}

func toStrings(delegate gomegatypes.GomegaMatcher) gomegatypes.GomegaMatcher {
	return toStringsMatcher{delegate: delegate}
}

type toStringsMatcher struct {
	delegate gomegatypes.GomegaMatcher
}

func (t toStringsMatcher) toStrings(actualAny any) ([]string, error) {
	actual, ok := actualAny.([]net.IP)
	if !ok {
		return nil, fmt.Errorf("expected []net.IP, got %T", actualAny)
	}

	actualStrings := make([]string, len(actual))
	for i := range actual {
		actualStrings[i] = actual[i].String()
	}
	return actualStrings, nil
}

func (t toStringsMatcher) Match(actual any) (success bool, err error) {
	actualStrings, err := t.toStrings(actual)
	if err != nil {
		return false, err
	}
	return t.delegate.Match(actualStrings)
}

func (t toStringsMatcher) FailureMessage(actual any) (message string) {
	actualStrings, _ := t.toStrings(actual)
	return t.delegate.FailureMessage(actualStrings)
}

func (t toStringsMatcher) NegatedFailureMessage(actual any) (message string) {
	actualStrings, _ := t.toStrings(actual)
	return t.delegate.NegatedFailureMessage(actualStrings)
}

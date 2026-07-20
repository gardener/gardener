// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrappers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/testing"
)

var _ = Describe("#bootstrapCluster", func() {
	var fakeDiscoveryClient *fakediscovery.FakeDiscovery

	BeforeEach(func() {
		fakeDiscoveryClient = &fakediscovery.FakeDiscovery{Fake: &testing.Fake{}}
		fakeDiscoveryClient.FakedServerVersion = &version.Info{GitVersion: "1.32.4"}
	})

	It("should return an error because the garden version cannot be parsed", func() {
		fakeDiscoveryClient.FakedServerVersion.GitVersion = ""
		Expect(bootstrapCluster(fakeDiscoveryClient)).To(MatchError(ContainSubstring("invalid semantic version")))
	})

	It("should return an error because the garden version is too low", func() {
		fakeDiscoveryClient.FakedServerVersion.GitVersion = "1.31.5"
		Expect(bootstrapCluster(fakeDiscoveryClient)).To(MatchError(ContainSubstring("the Kubernetes version of the Garden cluster must be at least 1.32")))
	})

	It("should succeed when garden version meets the minimum requirement", func() {
		Expect(bootstrapCluster(fakeDiscoveryClient)).To(Succeed())
	})
})

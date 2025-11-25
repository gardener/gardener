// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	autoscalingv1 "k8s.io/api/autoscaling/v1"

	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("Dns", func() {
	DescribeTable("#GetDomainInfoFromAnnotations",
		func(annotations map[string]string, expectedProvider, expectedDomain, expectedZone, expectedErr gomegatypes.GomegaMatcher) {
			provider, domain, zone, err := GetDomainInfoFromAnnotations(annotations)
			Expect(provider).To(expectedProvider)
			Expect(domain).To(expectedDomain)
			Expect(zone).To(expectedZone)
			Expect(err).To(expectedErr)
		},

		Entry("no annotations", nil, BeEmpty(), BeEmpty(), BeEmpty(), HaveOccurred()),
		Entry("no domain", map[string]string{
			DNSProvider: "bar",
		}, BeEmpty(), BeEmpty(), BeEmpty(), HaveOccurred()),
		Entry("no provider", map[string]string{
			DNSDomain: "foo",
		}, BeEmpty(), BeEmpty(), BeEmpty(), HaveOccurred()),
		Entry("all present", map[string]string{
			DNSProvider: "bar",
			DNSDomain:   "foo",
			DNSZone:     "zoo",
		}, Equal("bar"), Equal("foo"), Equal("zoo"), Not(HaveOccurred())),
	)

	DescribeTable("#GenerateDNSProviderName",
		func(ref *autoscalingv1.CrossVersionObjectReference, providerType, expectedName string) {
			Expect(GenerateDNSProviderName(ref, providerType)).To(Equal(expectedName))
		},

		Entry("both empty", nil, "", ""),
		Entry("credentialsRef nil", nil, "provider-type", "provider-type"),
		Entry("Secret credentialsRef and providerType empty", &autoscalingv1.CrossVersionObjectReference{Kind: "Secret", Name: "name-secret1"}, "", "secret-name-secret1"),
		Entry("WorkloadIdentity credentialsRef and providerType empty", &autoscalingv1.CrossVersionObjectReference{Kind: "WorkloadIdentity", Name: "name-workload-identity1"}, "", "workloadidentity-name-workload-identity1"),
		Entry("Secret credentialsRef and providerType", &autoscalingv1.CrossVersionObjectReference{Kind: "Secret", Name: "name-secret1"}, "type-1", "type-1-secret-name-secret1"),
		Entry("WorkloadIdentity credentialsRef and providerType", &autoscalingv1.CrossVersionObjectReference{Kind: "WorkloadIdentity", Name: "name-workload-identity1"}, "type-2", "type-2-workloadidentity-name-workload-identity1"),
		Entry("only providerType", nil, "provider-type", "provider-type"),
	)
})

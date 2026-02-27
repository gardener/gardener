// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package containerd_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	containerd "github.com/gardener/gardener/pkg/nodeagent/containerd"
	fakecontainerd "github.com/gardener/gardener/pkg/nodeagent/containerd/fake"
)

var _ = Describe("Containerd Client version tests", func() {

	var (
		ctx    context.Context
		client *fakecontainerd.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = fakecontainerd.NewClient()
	})

	DescribeTable("containerd version greater or equal 2.2", func(version string, result bool) {
		client.SetFakeContainerdVersion(version)
		r, err := containerd.VersionGreaterThanEqual22(ctx, client)
		Expect(err).ToNot(HaveOccurred())
		Expect(r).To(Equal(result))
	},
		Entry("should detect 1.7.23 is lower", "1.7.23", false),
		Entry("should properly parse 1.7.23~ds2 which is and lower", "1.7.23~ds2", false),
		Entry("should detect 2.2.0 is greater or equal", "2.2.0", true),
		Entry("should properly parse 2.3.0~ds2 which is greater or equal", "2.3.0~ds2", true),
		Entry("should detect 2.3.0-foo+bar is greater or equal", "2.3.0-foo+bar", true),
		Entry("should allow and parse the invalid 2.3.0-foo~ds2+bar~ds1", "2.3.0-foo~ds2+bar~ds1", true),
		Entry("should allow and parse the invalid 2.3.0-foo+123.45", "2.3.0-foo+123.45", true),
		Entry("should allow and parse the invalid 2.3.0-foo123.45", "2.3.0-foo+123.45", true),
	)
})

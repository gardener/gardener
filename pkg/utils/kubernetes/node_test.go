// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("Node", func() {
	Describe("IsNodeLabelAllowedForKubelet", func() {
		It("should return false for labels with node-restriction.kubernetes.io/ prefix", func() {
			Expect(IsNodeLabelAllowedForKubelet("node-restriction.kubernetes.io/foo")).To(BeFalse())
			Expect(IsNodeLabelAllowedForKubelet("bar.node-restriction.kubernetes.io/foo")).To(BeFalse())
		})

		It("should return true for kubelet labels", func() {
			var kubeletLabels = []string{
				// see https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#noderestriction
				"kubernetes.io/hostname",
				"kubernetes.io/arch",
				"kubernetes.io/os",
				"beta.kubernetes.io/instance-type",
				"node.kubernetes.io/instance-type",
				"failure-domain.beta.kubernetes.io/region",
				"failure-domain.beta.kubernetes.io/zone",
				"topology.kubernetes.io/region",
				"topology.kubernetes.io/zone",
				// see https://github.com/kubernetes/kubernetes/blob/release-1.26/staging/src/k8s.io/kubelet/pkg/apis/well_known_labels.go#L46-L47
				"beta.kubernetes.io/os",
				"beta.kubernetes.io/arch",
			}

			for _, label := range kubeletLabels {
				Expect(IsNodeLabelAllowedForKubelet(label)).To(BeTrue(), "should return true for %s", label)
			}
		})

		It("should return true for labels with kubelet prefix", func() {
			Expect(IsNodeLabelAllowedForKubelet("kubelet.kubernetes.io/foo")).To(BeTrue())
			Expect(IsNodeLabelAllowedForKubelet("bar.kubelet.kubernetes.io/foo")).To(BeTrue())
			Expect(IsNodeLabelAllowedForKubelet("node.kubernetes.io/foo")).To(BeTrue())
			Expect(IsNodeLabelAllowedForKubelet("bar.node.kubernetes.io/foo")).To(BeTrue())
		})

		It("should return false for other labels in kubernetes.io and k8s.io namespaces", func() {
			Expect(IsNodeLabelAllowedForKubelet("kubernetes.io/foo")).To(BeFalse())
			Expect(IsNodeLabelAllowedForKubelet("bar.kubernetes.io/foo")).To(BeFalse())
			Expect(IsNodeLabelAllowedForKubelet("k8s.io/foo")).To(BeFalse())
			Expect(IsNodeLabelAllowedForKubelet("bar.k8s.io/foo")).To(BeFalse())
		})

		It("should return true for other allowed labels", func() {
			Expect(IsNodeLabelAllowedForKubelet("gardener.cloud/foo")).To(BeTrue())
			Expect(IsNodeLabelAllowedForKubelet("bar.gardener.cloud/foo")).To(BeTrue())
			Expect(IsNodeLabelAllowedForKubelet("example.com/foo")).To(BeTrue())
			Expect(IsNodeLabelAllowedForKubelet("bar.example.com/foo")).To(BeTrue())
			Expect(IsNodeLabelAllowedForKubelet("foo")).To(BeTrue())
			Expect(IsNodeLabelAllowedForKubelet("foo/bar")).To(BeTrue())
		})
	})
})

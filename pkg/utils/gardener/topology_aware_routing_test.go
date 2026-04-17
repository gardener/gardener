// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"

	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("TopologyAwareRouting", func() {
	Describe("#ReconcileTopologyAwareRoutingSettings", func() {
		When("K8s version >= 1.34", func() {
			It("should set traffic distribution field when topology-aware routing is enabled", func() {
				service := &corev1.Service{}
				ReconcileTopologyAwareRoutingSettings(service, true, semver.MustParse("1.34.0"))
				Expect(service.Spec.TrafficDistribution).To(PointTo(Equal(corev1.ServiceTrafficDistributionPreferSameZone)))
			})
		})

		When("K8s version >= 1.32", func() {
			It("should set traffic distribution field when topology-aware routing is enabled", func() {
				service := &corev1.Service{}
				ReconcileTopologyAwareRoutingSettings(service, true, semver.MustParse("1.32.1"))
				Expect(service.Spec.TrafficDistribution).To(PointTo(Equal(corev1.ServiceTrafficDistributionPreferClose)))
			})
		})
	})
})

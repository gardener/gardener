// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package healthcheck_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/clock/testing"

	. "github.com/gardener/gardener/pkg/nodeagent/controller/healthcheck"
)

var _ = Describe("Kubelet", func() {
	var (
		khc   KubeletHealthChecker
		clock *testing.FakeClock
	)

	BeforeEach(func() {
		clock = testing.NewFakeClock(time.Now())
		khc = KubeletHealthChecker{
			KubeletReadinessToggles: []time.Time{},
			Clock:                   clock,
		}
	})

	Describe("#ToggleKubeletState", func() {
		It("should be false when toggling for the first time", func() {
			Expect(khc.ToggleKubeletState()).To(BeFalse())
		})

		It("should be true when toggling for five times", func() {
			Expect(khc.ToggleKubeletState()).To(BeFalse())
			Expect(khc.ToggleKubeletState()).To(BeFalse())
			Expect(khc.ToggleKubeletState()).To(BeFalse())
			Expect(khc.ToggleKubeletState()).To(BeFalse())
			Expect(khc.ToggleKubeletState()).To(BeTrue())
		})

		It("should forget toggles older than 10 minutes", func() {
			Expect(khc.ToggleKubeletState()).To(BeFalse())
			Expect(khc.ToggleKubeletState()).To(BeFalse())
			Expect(khc.ToggleKubeletState()).To(BeFalse())

			clock.Step(3 * time.Minute)

			Expect(khc.ToggleKubeletState()).To(BeFalse())

			clock.Step(8 * time.Minute)

			Expect(khc.ToggleKubeletState()).To(BeFalse())
			Expect(khc.ToggleKubeletState()).To(BeFalse())
			Expect(khc.KubeletReadinessToggles).To(HaveLen(3))

			clock.Step(11 * time.Minute)

			Expect(khc.ToggleKubeletState()).To(BeFalse())
			Expect(khc.KubeletReadinessToggles).To(HaveLen(1))
		})
	})

	Describe("#RevertToggleKubeletState", func() {
		It("should revert a toggle", func() {
			Expect(khc.ToggleKubeletState()).To(BeFalse())
			Expect(khc.KubeletReadinessToggles).To(HaveLen(1))
			khc.RevertToggleKubeletState()
			Expect(khc.KubeletReadinessToggles).To(BeEmpty())
		})

		It("should not fail to revert toggles even when there is no one", func() {
			Expect(khc.KubeletReadinessToggles).To(BeEmpty())
			khc.RevertToggleKubeletState()
			Expect(khc.KubeletReadinessToggles).To(BeEmpty())
		})
	})
})

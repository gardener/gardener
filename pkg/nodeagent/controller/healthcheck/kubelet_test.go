// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
		It("should revert a toogle", func() {
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

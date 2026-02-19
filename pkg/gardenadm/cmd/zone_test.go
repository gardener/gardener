// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/gardenadm/cmd"
)

var _ = Describe("ValidateZone", func() {
	var worker gardencorev1beta1.Worker

	Context("when worker has no zones configured", func() {
		BeforeEach(func() {
			worker = gardencorev1beta1.Worker{
				Name:  "test-worker",
				Zones: []string{},
			}
		})

		It("should return error when zone is provided", func() {
			zone, err := cmd.ValidateZone(worker, "custom-zone")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("worker \"test-worker\" has no zones configured, but zone \"custom-zone\" was provided"))
			Expect(zone).To(BeEmpty())
		})

		It("should return empty zone when no zone is provided", func() {
			zone, err := cmd.ValidateZone(worker, "")
			Expect(err).ToNot(HaveOccurred())
			Expect(zone).To(BeEmpty())
		})
	})

	Context("when worker has a single zone configured", func() {
		BeforeEach(func() {
			worker = gardencorev1beta1.Worker{
				Name:  "test-worker",
				Zones: []string{"zone-1"},
			}
		})

		It("should auto-apply the configured zone when no zone provided", func() {
			zone, err := cmd.ValidateZone(worker, "")
			Expect(err).ToNot(HaveOccurred())
			Expect(zone).To(Equal("zone-1"))
		})

		It("should accept the correct zone when provided", func() {
			zone, err := cmd.ValidateZone(worker, "zone-1")
			Expect(err).ToNot(HaveOccurred())
			Expect(zone).To(Equal("zone-1"))
		})

		It("should reject an incorrect zone when provided", func() {
			zone, err := cmd.ValidateZone(worker, "zone-2")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("provided zone \"zone-2\" does not match the configured zones [zone-1] for worker \"test-worker\""))
			Expect(zone).To(BeEmpty())
		})
	})

	Context("when worker has multiple zones configured", func() {
		BeforeEach(func() {
			worker = gardencorev1beta1.Worker{
				Name:  "test-worker",
				Zones: []string{"zone-1", "zone-2", "zone-3"},
			}
		})

		It("should require zone flag when no zone provided", func() {
			zone, err := cmd.ValidateZone(worker, "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("worker \"test-worker\" has multiple zones configured [zone-1 zone-2 zone-3], --zone flag is required"))
			Expect(zone).To(BeEmpty())
		})

		It("should accept a valid zone when provided", func() {
			zone, err := cmd.ValidateZone(worker, "zone-2")
			Expect(err).ToNot(HaveOccurred())
			Expect(zone).To(Equal("zone-2"))
		})

		It("should reject an invalid zone when provided", func() {
			zone, err := cmd.ValidateZone(worker, "zone-4")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("provided zone \"zone-4\" does not match the configured zones [zone-1 zone-2 zone-3] for worker \"test-worker\""))
			Expect(zone).To(BeEmpty())
		})
	})
})

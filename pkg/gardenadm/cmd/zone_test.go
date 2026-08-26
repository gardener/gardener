// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package cmd_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/gardenadm/cmd"
)

var _ = Describe("Zone", func() {
	Describe("#ValidateAndDetermineControlPlaneZone", func() {
		unmanagedShoot := func(zones []string) *gardencorev1beta1.Shoot {
			return &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{{
							Name:         "control-plane",
							Zones:        zones,
							ControlPlane: &gardencorev1beta1.WorkerControlPlane{},
						}},
					},
				},
			}
		}

		When("the shoot has managed infrastructure", func() {
			managedShoot := &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{
							{
								Name:         "control-plane",
								Zones:        []string{"us-east-1a"},
								ControlPlane: &gardencorev1beta1.WorkerControlPlane{},
							},
						},
					},
					CredentialsBindingName: new("test-credentials"),
				},
			}

			It("should allow a provided zone", func() {
				zone, err := cmd.ValidateAndDetermineControlPlaneZone(managedShoot, "us-east-1a")
				Expect(err).ToNot(HaveOccurred())
				Expect(zone).To(Equal("us-east-1a"))
			})
		})

		It("should fail when the shoot is nil", func() {
			zone, err := cmd.ValidateAndDetermineControlPlaneZone(nil, "")
			Expect(err).To(MatchError(ContainSubstring("shoot resource is missing in the manifests")))
			Expect(zone).To(BeEmpty())
		})

		It("should fail when the shoot has no control plane worker pool", func() {
			shoot := &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{{Name: "worker"}},
					},
				},
			}
			zone, err := cmd.ValidateAndDetermineControlPlaneZone(shoot, "")
			Expect(err).To(MatchError(ContainSubstring("shoot doesn't have a control plane worker pool configured")))
			Expect(zone).To(BeEmpty())
		})

		It("should determine the zone against the control plane worker pool", func() {
			zone, err := cmd.ValidateAndDetermineControlPlaneZone(unmanagedShoot([]string{"zone-1"}), "")
			Expect(err).ToNot(HaveOccurred())
			Expect(zone).To(Equal("zone-1"))
		})

		It("should surface DetermineZone errors for the control plane worker pool", func() {
			zone, err := cmd.ValidateAndDetermineControlPlaneZone(unmanagedShoot([]string{"zone-1", "zone-2"}), "")
			Expect(err).To(MatchError(ContainSubstring(`worker "control-plane" has multiple zones configured [zone-1 zone-2], --zone flag is required`)))
			Expect(zone).To(BeEmpty())
		})
	})

	Describe("#DetermineZone", func() {
		var worker gardencorev1beta1.Worker

		When("worker has no zones configured", func() {
			BeforeEach(func() {
				worker = gardencorev1beta1.Worker{
					Name:  "test-worker",
					Zones: []string{},
				}
			})

			It("should return error when zone is provided", func() {
				zone, err := cmd.DetermineZone(worker, "custom-zone")
				Expect(err).To(MatchError(ContainSubstring(`worker "test-worker" has no zones configured, but zone "custom-zone" was provided`)))
				Expect(zone).To(BeEmpty())
			})

			It("should return empty zone when no zone is provided", func() {
				zone, err := cmd.DetermineZone(worker, "")
				Expect(err).ToNot(HaveOccurred())
				Expect(zone).To(BeEmpty())
			})
		})

		When("worker has a single zone configured", func() {
			BeforeEach(func() {
				worker = gardencorev1beta1.Worker{
					Name:  "test-worker",
					Zones: []string{"zone-1"},
				}
			})

			It("should auto-apply the configured zone when no zone provided", func() {
				zone, err := cmd.DetermineZone(worker, "")
				Expect(err).ToNot(HaveOccurred())
				Expect(zone).To(Equal("zone-1"))
			})

			It("should accept the correct zone when provided", func() {
				zone, err := cmd.DetermineZone(worker, "zone-1")
				Expect(err).ToNot(HaveOccurred())
				Expect(zone).To(Equal("zone-1"))
			})

			It("should reject an incorrect zone when provided", func() {
				zone, err := cmd.DetermineZone(worker, "zone-2")
				Expect(err).To(MatchError(ContainSubstring(`provided zone "zone-2" does not match the configured zones [zone-1] for worker "test-worker"`)))
				Expect(zone).To(BeEmpty())
			})
		})

		When("worker has multiple zones configured", func() {
			BeforeEach(func() {
				worker = gardencorev1beta1.Worker{
					Name:  "test-worker",
					Zones: []string{"zone-1", "zone-2", "zone-3"},
				}
			})

			It("should require zone flag when no zone provided", func() {
				zone, err := cmd.DetermineZone(worker, "")
				Expect(err).To(MatchError(ContainSubstring(`worker "test-worker" has multiple zones configured [zone-1 zone-2 zone-3], --zone flag is required`)))
				Expect(zone).To(BeEmpty())
			})

			It("should accept a valid zone when provided", func() {
				zone, err := cmd.DetermineZone(worker, "zone-2")
				Expect(err).ToNot(HaveOccurred())
				Expect(zone).To(Equal("zone-2"))
			})

			It("should reject an invalid zone when provided", func() {
				zone, err := cmd.DetermineZone(worker, "zone-4")
				Expect(err).To(MatchError(ContainSubstring(`provided zone "zone-4" does not match the configured zones [zone-1 zone-2 zone-3] for worker "test-worker"`)))
				Expect(zone).To(BeEmpty())
			})
		})
	})
})

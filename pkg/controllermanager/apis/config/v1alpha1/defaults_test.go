// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	"time"

	. "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Defaults", func() {
	Describe("#SetDefaults_ControllerManagerConfiguration", func() {
		var obj *ControllerManagerConfiguration

		BeforeEach(func() {
			obj = &ControllerManagerConfiguration{}
		})

		It("should correctly default the controller manager configuration", func() {
			SetDefaults_ControllerManagerConfiguration(obj)

			Expect(obj.Server.HTTP.BindAddress).To(Equal("0.0.0.0"))
			Expect(obj.Server.HTTP.Port).To(Equal(2718))
			Expect(obj.Server.HTTPS.BindAddress).To(Equal("0.0.0.0"))
			Expect(obj.Server.HTTPS.Port).To(Equal(2719))

			Expect(obj.Controllers.CloudProfile).NotTo(BeNil())
			Expect(obj.Controllers.CloudProfile.ConcurrentSyncs).To(Equal(5))

			Expect(obj.Controllers.ControllerRegistration).NotTo(BeNil())
			Expect(obj.Controllers.ControllerRegistration.ConcurrentSyncs).To(Equal(5))

			Expect(obj.Controllers.Plant).NotTo(BeNil())
			Expect(obj.Controllers.Plant.ConcurrentSyncs).To(Equal(5))
			Expect(obj.Controllers.Plant.SyncPeriod).To(Equal(metav1.Duration{Duration: 30 * time.Second}))

			Expect(obj.Controllers.Project).NotTo(BeNil())
			Expect(obj.Controllers.Project.ConcurrentSyncs).To(Equal(5))
			Expect(obj.Controllers.Project.MinimumLifetimeDays).To(PointTo(Equal(30)))
			Expect(obj.Controllers.Project.StaleGracePeriodDays).To(PointTo(Equal(14)))
			Expect(obj.Controllers.Project.StaleExpirationTimeDays).To(PointTo(Equal(90)))
			Expect(obj.Controllers.Project.StaleSyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: 12 * time.Hour})))

			Expect(obj.Controllers.Quota).NotTo(BeNil())
			Expect(obj.Controllers.Quota.ConcurrentSyncs).To(Equal(5))

			Expect(obj.Controllers.SecretBinding).NotTo(BeNil())
			Expect(obj.Controllers.SecretBinding.ConcurrentSyncs).To(Equal(5))

			Expect(obj.Controllers.Seed).NotTo(BeNil())
			Expect(obj.Controllers.Seed.ConcurrentSyncs).To(Equal(5))
			Expect(obj.Controllers.Seed.SyncPeriod).To(Equal(metav1.Duration{Duration: 30 * time.Second}))
			Expect(obj.Controllers.Seed.MonitorPeriod).To(PointTo(Equal(metav1.Duration{Duration: 40 * time.Second})))
			Expect(obj.Controllers.Seed.ShootMonitorPeriod).To(PointTo(Equal(metav1.Duration{Duration: 200 * time.Second})))
		})
	})
	Describe("#SetDefaults_EventControllerConfiguration", func() {
		It("should correctly default the Event Controller configuration", func() {
			obj := &EventControllerConfiguration{}

			SetDefaults_EventControllerConfiguration(obj)
			Expect(obj.TTLNonShootEvents).To(PointTo(Equal(metav1.Duration{Duration: time.Hour})))
		})
	})
})

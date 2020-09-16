// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	"k8s.io/klog"

	. "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Defaults", func() {
	Describe("#SetDefaults_GardenletConfiguration", func() {
		var obj *GardenletConfiguration

		BeforeEach(func() {
			obj = &GardenletConfiguration{}
		})

		It("should default the gardenlet configuration", func() {
			SetDefaults_GardenletConfiguration(obj)

			Expect(obj.GardenClientConnection).NotTo(BeNil())
			Expect(obj.SeedClientConnection).NotTo(BeNil())
			Expect(obj.ShootClientConnection).NotTo(BeNil())
			Expect(obj.Controllers.BackupBucket).NotTo(BeNil())
			Expect(obj.Controllers.BackupEntry).NotTo(BeNil())
			Expect(obj.Controllers.ControllerInstallation).NotTo(BeNil())
			Expect(obj.Controllers.ControllerInstallationCare).NotTo(BeNil())
			Expect(obj.Controllers.ControllerInstallationRequired).NotTo(BeNil())
			Expect(obj.Controllers.Seed).NotTo(BeNil())
			Expect(obj.Controllers.Shoot).NotTo(BeNil())
			Expect(obj.Controllers.ShootCare).NotTo(BeNil())
			Expect(obj.Controllers.ShootStateSync).NotTo(BeNil())
			Expect(obj.LeaderElection).NotTo(BeNil())
			Expect(obj.LogLevel).To(PointTo(Equal("info")))
			Expect(obj.KubernetesLogLevel).To(PointTo(Equal(klog.Level(0))))
			Expect(obj.Server.HTTPS.BindAddress).To(Equal("0.0.0.0"))
			Expect(obj.Server.HTTPS.Port).To(Equal(2720))
		})
	})
})

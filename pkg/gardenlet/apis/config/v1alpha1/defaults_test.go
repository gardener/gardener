// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package v1alpha1_test

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			SetObjectDefaults_GardenletConfiguration(obj)

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
			Expect(obj.Controllers.ShootedSeedRegistration).NotTo(BeNil())
			Expect(obj.LeaderElection).NotTo(BeNil())
			Expect(obj.LogLevel).To(PointTo(Equal("info")))
			Expect(obj.KubernetesLogLevel).To(PointTo(Equal(klog.Level(0))))
			Expect(obj.Server.HTTPS.BindAddress).To(Equal("0.0.0.0"))
			Expect(obj.Server.HTTPS.Port).To(Equal(2720))
			Expect(obj.SNI).ToNot(BeNil())
			Expect(obj.SNI.Ingress).ToNot(BeNil())
			Expect(obj.SNI.Ingress.Labels).To(Equal(map[string]string{"istio": "ingressgateway"}))
			Expect(obj.SNI.Ingress.Namespace).To(PointTo(Equal("istio-ingress")))
			Expect(obj.SNI.Ingress.ServiceName).To(PointTo(Equal("istio-ingressgateway")))
		})
	})

	Describe("#SetDefaults_ShootedSeedRegistrationControllerConfiguration", func() {
		var obj *ShootedSeedRegistrationControllerConfiguration

		BeforeEach(func() {
			obj = &ShootedSeedRegistrationControllerConfiguration{}
		})

		It("should default the configuration", func() {
			SetDefaults_ShootedSeedRegistrationControllerConfiguration(obj)

			Expect(obj.SyncJitterPeriod).To(PointTo(Equal(metav1.Duration{Duration: 5 * time.Minute})))
		})
	})

	Describe("#SetDefaults_ShootControllerConfiguration", func() {
		var obj *ShootControllerConfiguration

		BeforeEach(func() {
			obj = &ShootControllerConfiguration{}
		})

		It("should default the configuration", func() {
			SetDefaults_ShootControllerConfiguration(obj)

			Expect(obj.ConcurrentSyncs).To(PointTo(Equal(20)))
			Expect(obj.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Hour})))
			Expect(obj.RespectSyncPeriodOverwrite).To(PointTo(Equal(false)))
			Expect(obj.ReconcileInMaintenanceOnly).To(PointTo(Equal(false)))
			Expect(obj.RetryDuration).To(PointTo(Equal(metav1.Duration{Duration: 12 * time.Hour})))
			Expect(obj.DNSEntryTTLSeconds).To(PointTo(Equal(int64(120))))
		})
	})
})

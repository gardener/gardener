// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("ControllerRegistration defaulting", func() {
	var obj *ControllerRegistration

	BeforeEach(func() {
		obj = &ControllerRegistration{
			Spec: ControllerRegistrationSpec{
				Resources: []ControllerResource{
					{
						Kind: "Infrastructure",
						Type: "provider-foo",
					},
					{
						Kind: "Extension",
						Type: "extension-foo",
					},
				},
				Deployment: &ControllerRegistrationDeployment{
					DeploymentRefs: []DeploymentRef{{
						Name: "foo",
					}},
				},
			},
		}
	})

	Describe("ControllerResource defaulting", func() {
		It("should default the primary field", func() {
			SetObjectDefaults_ControllerRegistration(obj)

			Expect(obj.Spec.Resources[0].Primary).To(PointTo(BeTrue()))
			Expect(obj.Spec.Resources[1].Primary).To(PointTo(BeTrue()))
		})

		It("should not overwrite the primary field", func() {
			obj.Spec.Resources[0].Primary = ptr.To(false)
			obj.Spec.Resources[1].Primary = ptr.To(false)

			SetObjectDefaults_ControllerRegistration(obj)

			Expect(obj.Spec.Resources[0].Primary).To(PointTo(BeFalse()))
			Expect(obj.Spec.Resources[1].Primary).To(PointTo(BeFalse()))
		})

		It("should not default the WorkerlessSupported field", func() {
			SetObjectDefaults_ControllerRegistration(obj)

			Expect(obj.Spec.Resources[0].WorkerlessSupported).To(BeNil())
			Expect(obj.Spec.Resources[1].WorkerlessSupported).To(BeNil())
		})

		Context("kind != Extension", func() {
			It("should not default the globallyEnabled field", func() {
				SetObjectDefaults_ControllerRegistration(obj)

				Expect(obj.Spec.Resources[0].GloballyEnabled).To(BeNil())
			})

			It("should not default the autoEnable field", func() {
				SetObjectDefaults_ControllerRegistration(obj)

				Expect(obj.Spec.Resources[0].AutoEnable).To(BeEmpty())
			})

			It("should not default the reconcileTimeout field", func() {
				SetObjectDefaults_ControllerRegistration(obj)

				Expect(obj.Spec.Resources[0].ReconcileTimeout).To(BeNil())
			})

			It("should default the lifecycle field", func() {
				SetObjectDefaults_ControllerRegistration(obj)

				Expect(obj.Spec.Resources[0].Lifecycle).To(BeNil())
			})
		})

		Context("kind == Extension", func() {
			It("should not default the globallyEnabled field", func() {
				SetObjectDefaults_ControllerRegistration(obj)

				Expect(obj.Spec.Resources[1].GloballyEnabled).To(BeNil())
			})

			It("should not overwrite the globallyEnabled field", func() {
				obj.Spec.Resources[1].GloballyEnabled = ptr.To(true)

				SetObjectDefaults_ControllerRegistration(obj)

				Expect(obj.Spec.Resources[1].GloballyEnabled).To(Equal(ptr.To(true)))
			})

			It("should not default the globallyEnabled field if autoEnable is set to shoot", func() {
				obj.Spec.Resources[1].AutoEnable = []ClusterType{"shoot"}

				SetObjectDefaults_ControllerRegistration(obj)

				Expect(obj.Spec.Resources[1].GloballyEnabled).To(BeNil())
			})

			It("should not default the globallyEnabled field if autoEnable is set to seed", func() {
				obj.Spec.Resources[1].AutoEnable = []ClusterType{"seed"}

				SetObjectDefaults_ControllerRegistration(obj)

				Expect(obj.Spec.Resources[1].GloballyEnabled).To(BeNil())
			})

			It("should not change the globallyEnabled field is it was set before", func() {
				obj.Spec.Resources[1].GloballyEnabled = ptr.To(false)
				obj.Spec.Resources[1].AutoEnable = []ClusterType{"seed"}

				SetObjectDefaults_ControllerRegistration(obj)

				Expect(obj.Spec.Resources[1].GloballyEnabled).To(Equal(ptr.To(false)))
			})

			It("should default the globallyEnabled field is it was set before and autoEnable contains shoot", func() {
				obj.Spec.Resources[1].GloballyEnabled = ptr.To(false)
				obj.Spec.Resources[1].AutoEnable = []ClusterType{"shoot"}

				SetObjectDefaults_ControllerRegistration(obj)

				Expect(obj.Spec.Resources[1].GloballyEnabled).To(Equal(ptr.To(true)))
			})

			It("should default the autoEnable field to shoot if globallyEnabled is true", func() {
				obj.Spec.Resources[1].GloballyEnabled = ptr.To(true)
				SetObjectDefaults_ControllerRegistration(obj)

				Expect(obj.Spec.Resources[1].AutoEnable).To(ConsistOf(ClusterType("shoot")))
			})

			It("should not overwrite the autoEnable field", func() {
				obj.Spec.Resources[1].AutoEnable = []ClusterType{"shoot", "seed"}

				SetObjectDefaults_ControllerRegistration(obj)

				Expect(obj.Spec.Resources[1].AutoEnable).To(ConsistOf(ClusterType("shoot"), ClusterType("seed")))
			})

			It("should default the reconcileTimeout field", func() {
				SetObjectDefaults_ControllerRegistration(obj)

				Expect(obj.Spec.Resources[1].ReconcileTimeout).To(Equal(&metav1.Duration{Duration: time.Minute * 3}))
			})

			It("should not overwrite the reconcileTimeout field", func() {
				obj.Spec.Resources[1].ReconcileTimeout = &metav1.Duration{Duration: time.Second * 62}

				SetObjectDefaults_ControllerRegistration(obj)

				Expect(obj.Spec.Resources[1].ReconcileTimeout).To(Equal(&metav1.Duration{Duration: time.Second * 62}))
			})

			It("should default the lifecycle field", func() {
				SetObjectDefaults_ControllerRegistration(obj)

				Expect(obj.Spec.Resources[1].Lifecycle).NotTo(BeNil())
				Expect(obj.Spec.Resources[1].Lifecycle.Reconcile).To(PointTo(BeEquivalentTo("AfterKubeAPIServer")))
				Expect(obj.Spec.Resources[1].Lifecycle.Delete).To(PointTo(BeEquivalentTo("BeforeKubeAPIServer")))
				Expect(obj.Spec.Resources[1].Lifecycle.Migrate).To(PointTo(BeEquivalentTo("BeforeKubeAPIServer")))
			})

			It("should only default the missing lifecycle fields", func() {
				before := ControllerResourceLifecycleStrategy("BeforeKubeAPIServer")
				obj.Spec.Resources[1].Lifecycle = &ControllerResourceLifecycle{}
				obj.Spec.Resources[1].Lifecycle.Reconcile = &before

				SetObjectDefaults_ControllerRegistration(obj)

				Expect(obj.Spec.Resources[1].Lifecycle).NotTo(BeNil())
				Expect(obj.Spec.Resources[1].Lifecycle.Reconcile).To(PointTo(BeEquivalentTo("BeforeKubeAPIServer")))
				Expect(obj.Spec.Resources[1].Lifecycle.Delete).To(PointTo(BeEquivalentTo("BeforeKubeAPIServer")))
				Expect(obj.Spec.Resources[1].Lifecycle.Migrate).To(PointTo(BeEquivalentTo("BeforeKubeAPIServer")))
			})

			It("should default the cluster compatibility", func() {
				SetObjectDefaults_ControllerRegistration(obj)

				Expect(obj.Spec.Resources[1].ClusterCompatibility).To(ConsistOf(ClusterType("shoot")))
			})

			It("should not default the cluster compatibility", func() {
				obj.Spec.Resources[1].ClusterCompatibility = []ClusterType{"seed"}

				SetObjectDefaults_ControllerRegistration(obj)

				Expect(obj.Spec.Resources[1].ClusterCompatibility).To(ConsistOf(ClusterType("seed")))
			})
		})
	})

	Describe("ControllerRegistrationDeployment defaulting", func() {
		It("should default the policy field", func() {
			SetObjectDefaults_ControllerRegistration(obj)

			Expect(obj.Spec.Deployment.Policy).To(PointTo(BeEquivalentTo("OnDemand")))
		})

		It("should not overwrite the policy field", func() {
			always := ControllerDeploymentPolicy("Always")
			obj.Spec.Deployment.Policy = &always

			SetObjectDefaults_ControllerRegistration(obj)

			Expect(obj.Spec.Deployment.Policy).To(PointTo(BeEquivalentTo("Always")))
		})
	})
})

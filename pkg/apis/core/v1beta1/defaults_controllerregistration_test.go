// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
					{
						Kind: "SelfHostedShootExposure",
						Type: "provider-foo",
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
			obj.Spec.Resources[0].Primary = new(false)
			obj.Spec.Resources[1].Primary = new(false)

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

		Context("kind == SelfHostedShootExposure", func() {
			BeforeEach(func() {
				obj.Spec.Resources = append(obj.Spec.Resources, ControllerResource{
					Kind: "SelfHostedShootExposure",
					Type: "provider-foo",
				})
			})

			It("should default ContinuousEndpointUpdate to true", func() {
				SetObjectDefaults_ControllerRegistration(obj)

				Expect(obj.Spec.Resources[2].ContinuousEndpointUpdate).To(PointTo(BeTrue()))
			})

			It("should not overwrite ContinuousEndpointUpdate", func() {
				obj.Spec.Resources[2].ContinuousEndpointUpdate = new(false)

				SetObjectDefaults_ControllerRegistration(obj)

				Expect(obj.Spec.Resources[2].ContinuousEndpointUpdate).To(PointTo(BeFalse()))
			})
		})

		It("should not default ContinuousEndpointUpdate", func() {
			SetObjectDefaults_ControllerRegistration(obj)

			Expect(obj.Spec.Resources[0].ContinuousEndpointUpdate).To(BeNil())
			Expect(obj.Spec.Resources[1].ContinuousEndpointUpdate).To(BeNil())
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

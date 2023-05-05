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

package v1alpha1_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	. "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
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
			obj.Spec.Resources[0].Primary = pointer.Bool(false)
			obj.Spec.Resources[1].Primary = pointer.Bool(false)

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
			It("should default the globallyEnabled field", func() {
				SetObjectDefaults_ControllerRegistration(obj)

				Expect(obj.Spec.Resources[1].GloballyEnabled).To(Equal(pointer.Bool(false)))
			})

			It("should not overwrite the globallyEnabled field", func() {
				obj.Spec.Resources[1].GloballyEnabled = pointer.Bool(true)

				SetObjectDefaults_ControllerRegistration(obj)

				Expect(obj.Spec.Resources[1].GloballyEnabled).To(Equal(pointer.Bool(true)))
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

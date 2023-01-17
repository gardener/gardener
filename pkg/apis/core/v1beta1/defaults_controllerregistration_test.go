// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package v1beta1_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	. "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("ControllerRegistration defaulting", func() {
	Describe("#SetDefaults_ControllerResource", func() {
		It("should default the primary field", func() {
			resource := ControllerResource{}

			SetDefaults_ControllerResource(&resource)

			Expect(resource.Primary).To(PointTo(BeTrue()))
		})

		It("should not default the primary field", func() {
			resource := ControllerResource{Primary: pointer.Bool(false)}
			resourceCopy := resource.DeepCopy()

			SetDefaults_ControllerResource(&resource)

			Expect(resource.Primary).To(Equal(resourceCopy.Primary))
		})

		const kindExtension = "Extension"
		It("should default the globallyEnabled field when kind is Extension", func() {
			resource := ControllerResource{Kind: kindExtension}
			SetDefaults_ControllerResource(&resource)

			Expect(resource.GloballyEnabled).To(Equal(pointer.Bool(false)))
		})

		It("should not default the globallyEnabled field when kind is Extension and globallyEnabled is already set", func() {
			resource := ControllerResource{Kind: kindExtension, GloballyEnabled: pointer.Bool(true)}
			SetDefaults_ControllerResource(&resource)

			Expect(resource.GloballyEnabled).To(Equal(pointer.Bool(true)))
		})

		It("should not default the globallyEnabled field when kind is not Extension", func() {
			resource := ControllerResource{Kind: "not extension"}
			SetDefaults_ControllerResource(&resource)

			Expect(resource.GloballyEnabled).To(BeNil())
		})

		It("should default the reconcile timeout when kind is Extension", func() {
			resource := ControllerResource{Kind: kindExtension}
			SetDefaults_ControllerResource(&resource)

			Expect(resource.ReconcileTimeout).To(Equal(&metav1.Duration{Duration: time.Minute * 3}))
		})

		It("should not default the reconcile timeout when kind is Extension and timeout is already set", func() {
			resource := ControllerResource{Kind: kindExtension, ReconcileTimeout: &metav1.Duration{Duration: time.Second * 62}}
			SetDefaults_ControllerResource(&resource)

			Expect(resource.ReconcileTimeout).To(Equal(&metav1.Duration{Duration: time.Second * 62}))
		})

		It("should not default the reconcile timeout when kind is not Extension", func() {
			resource := ControllerResource{Kind: "not extension"}
			SetDefaults_ControllerResource(&resource)

			Expect(resource.ReconcileTimeout).To(BeNil())
		})

		It("should default the lifecycle strategy field when kind is Extension", func() {
			resource := ControllerResource{Kind: kindExtension}
			SetDefaults_ControllerResource(&resource)

			Expect(resource.Lifecycle).ToNot(BeNil())
			Expect(*resource.Lifecycle.Reconcile).To(Equal(AfterKubeAPIServer))
			Expect(*resource.Lifecycle.Delete).To(Equal(BeforeKubeAPIServer))
			Expect(*resource.Lifecycle.Migrate).To(Equal(BeforeKubeAPIServer))
		})

		It("should default the lifecycle strategy field when kind is not Extension", func() {
			resource := ControllerResource{Kind: "not an extension"}
			SetDefaults_ControllerResource(&resource)

			Expect(resource.Lifecycle).To(BeNil())
		})

		It("should default only the missing lifecycle strategy fields when kind is Extension", func() {
			before := BeforeKubeAPIServer
			resource := ControllerResource{
				Kind: kindExtension,
				Lifecycle: &ControllerResourceLifecycle{
					Reconcile: &before,
				},
			}
			SetDefaults_ControllerResource(&resource)

			Expect(resource.Lifecycle).ToNot(BeNil())
			Expect(*resource.Lifecycle.Reconcile).To(Equal(BeforeKubeAPIServer))
			Expect(*resource.Lifecycle.Delete).To(Equal(BeforeKubeAPIServer))
			Expect(*resource.Lifecycle.Migrate).To(Equal(BeforeKubeAPIServer))
		})
	})

	Describe("#SetDefaults_ControllerRegistrationDeployment", func() {
		var (
			ondemand = ControllerDeploymentPolicyOnDemand
			always   = ControllerDeploymentPolicyAlways
		)

		It("should default the policy field", func() {
			deployment := ControllerRegistrationDeployment{}

			SetDefaults_ControllerRegistrationDeployment(&deployment)

			Expect(deployment.Policy).To(PointTo(Equal(ondemand)))
		})

		It("should not default the policy field", func() {
			deployment := ControllerRegistrationDeployment{Policy: &always}
			deploymentCopy := deployment.DeepCopy()

			SetDefaults_ControllerRegistrationDeployment(&deployment)

			Expect(deployment.Policy).To(Equal(deploymentCopy.Policy))
		})
	})
})

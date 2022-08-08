// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controllerdeployment

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Controller", func() {
	const finalizerName = "core.gardener.cloud/controllerdeployment"

	var (
		ctx        = context.TODO()
		fakeClient client.Client
		reconciler reconcile.Reconciler

		controllerDeploymentName string
		controllerDeployment     *gardencorev1beta1.ControllerDeployment
		controllerRegistration   *gardencorev1beta1.ControllerRegistration
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

		controllerDeploymentName = "controllerDeployment"
		reconciler = &Reconciler{Client: fakeClient}

		controllerDeployment = &gardencorev1beta1.ControllerDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: controllerDeploymentName,
			},
		}

		controllerRegistration = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-controllerInstallation",
			},
			Spec: gardencorev1beta1.ControllerRegistrationSpec{
				Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
					DeploymentRefs: []gardencorev1beta1.DeploymentRef{
						{Name: controllerDeployment.Name},
					},
				},
			},
		}

	})

	It("should return nil because object is not found", func() {
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(controllerDeployment), &gardencorev1alpha1.ControllerDeployment{})).To(BeNotFoundError())

		result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerDeploymentName}})
		Expect(result).To(Equal(reconcile.Result{}))
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when deletion timestamp is not set", func() {
		BeforeEach(func() {
			Expect(fakeClient.Create(ctx, controllerDeployment)).To(Succeed())
		})

		It("should ensure the finalizer", func() {
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerDeploymentName}})
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(controllerDeployment), controllerDeployment))
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
			Expect(controllerDeployment.GetFinalizers()).Should(ConsistOf(finalizerName))
		})
	})

	Context("when deletion timestamp is set", func() {
		BeforeEach(func() {
			now := metav1.Now()
			controllerDeployment.DeletionTimestamp = &now
			controllerDeployment.Finalizers = []string{finalizerName}

			Expect(fakeClient.Create(ctx, controllerDeployment)).To(Succeed())
		})

		It("should do nothing because finalizer is not present", func() {
			Expect(fakeClient.Create(ctx, controllerRegistration)).To(Succeed())
			patch := client.MergeFrom(controllerDeployment.DeepCopy())
			controllerDeployment.Finalizers = []string{"test-finalizer"}
			Expect(fakeClient.Patch(ctx, controllerDeployment, patch)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerDeploymentName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error because ControllerRegistration referencing ControllerDeployment exists", func() {
			Expect(fakeClient.Create(ctx, controllerRegistration)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerDeploymentName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).To(MatchError(ContainSubstring("cannot remove finalizer of ControllerDeployment")))
		})

		It("should remove the finalizer because no ControllerRegistration is referencing the ControllerDeployment", func() {
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerDeploymentName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(controllerDeployment), controllerDeployment)).To(BeNotFoundError())
		})
	})
})

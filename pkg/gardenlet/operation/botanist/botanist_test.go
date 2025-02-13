// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("Botanist", func() {
	var (
		fakeClient                client.Client
		botanist                  *Botanist
		namespace                 *corev1.Namespace
		resourceManagerDeployment *appsv1.Deployment
	)

	BeforeEach(func(ctx context.Context) {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		botanist = &Botanist{Operation: &operation.Operation{}}
		k8sSeedClient := kubernetesfake.NewClientSetBuilder().WithClient(fakeClient).Build()

		namespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "botanist-"}}
		Expect(fakeClient.Create(ctx, namespace)).To(Succeed())
		DeferCleanup(func() {
			Expect(fakeClient.Delete(ctx, namespace)).To(Succeed())
		})

		botanist.SeedClientSet = k8sSeedClient
		botanist.Shoot = &shootpkg.Shoot{
			ControlPlaneNamespace: namespace.Name,
		}

		resourceManagerDeployment = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager", Namespace: namespace.Name}}
		Expect(fakeClient.Create(ctx, resourceManagerDeployment)).To(Succeed())
		DeferCleanup(func() {
			Expect(fakeClient.Delete(ctx, resourceManagerDeployment)).To(Succeed())
		})
	})

	Describe("#IsGardenerResourceManagerReady", func() {
		It("should return false if the gardener-resource-manager is not ready", func(ctx context.Context) {
			Expect(botanist.IsGardenerResourceManagerReady(ctx)).To(BeFalse())
		})

		It("should return true if the gardener-resource-manager is ready", func(ctx context.Context) {
			resourceManagerDeployment.Status.ReadyReplicas = 1
			Expect(fakeClient.Status().Update(ctx, resourceManagerDeployment)).To(Succeed())

			Expect(botanist.IsGardenerResourceManagerReady(ctx)).To(BeTrue())
		})
	})
})

// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
)

var _ = Describe("Node", func() {
	var (
		hostName string

		fakeClient client.Client
		b          *GardenadmBotanist

		node       *corev1.Node
		cloudTaint corev1.Taint
	)

	BeforeEach(func() {
		hostName = "test"

		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		b = &GardenadmBotanist{
			Botanist: &botanistpkg.Botanist{
				Operation: &operation.Operation{
					Logger:        logr.Discard(),
					SeedClientSet: fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build(),
				},
			},
			HostName: hostName,
		}

		node = &corev1.Node{ObjectMeta: metav1.ObjectMeta{
			Name:   "machine-" + hostName,
			Labels: map[string]string{corev1.LabelHostname: hostName},
		}}
		cloudTaint = corev1.Taint{Key: "node.cloudprovider.kubernetes.io/uninitialized", Value: "true", Effect: corev1.TaintEffectNoSchedule}
	})

	Describe("#TaintControlPlaneNodeForCloudProviderInitialization", func() {
		It("should fail if the node does not exist", func(ctx SpecContext) {
			Expect(b.TaintControlPlaneNodeForCloudProviderInitialization(ctx)).To(MatchError(ContainSubstring("not found")))
		})

		It("should add the cloud provider taint if the node is not initialized yet", func(ctx SpecContext) {
			node.Spec.Taints = []corev1.Taint{{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule}}
			Expect(fakeClient.Create(ctx, node)).To(Succeed())

			Expect(b.TaintControlPlaneNodeForCloudProviderInitialization(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
			Expect(node.Spec.Taints).To(ConsistOf(
				corev1.Taint{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
				cloudTaint,
			))
		})

		It("should not add the taint twice", func(ctx SpecContext) {
			node.Spec.Taints = []corev1.Taint{cloudTaint}
			Expect(fakeClient.Create(ctx, node)).To(Succeed())

			Expect(b.TaintControlPlaneNodeForCloudProviderInitialization(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
			Expect(node.Spec.Taints).To(ConsistOf(cloudTaint))
		})

		It("should not add the taint if the node has already been initialized", func(ctx SpecContext) {
			node.Spec.ProviderID = "machine-" + hostName
			Expect(fakeClient.Create(ctx, node)).To(Succeed())

			Expect(b.TaintControlPlaneNodeForCloudProviderInitialization(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
			Expect(node.Spec.Taints).To(BeEmpty())
		})
	})

	Describe("#WaitUntilControlPlaneNodeIsInitializedByCloudProvider", func() {
		It("should fail if the node does not exist", func(ctx SpecContext) {
			Expect(b.WaitUntilControlPlaneNodeIsInitializedByCloudProvider(ctx)).To(MatchError(ContainSubstring("not found")))
		})

		It("should fail if the provider ID is not set yet", func(ctx SpecContext) {
			node.Spec.Taints = []corev1.Taint{cloudTaint}
			Expect(fakeClient.Create(ctx, node)).To(Succeed())

			Expect(b.WaitUntilControlPlaneNodeIsInitializedByCloudProvider(ctx)).To(MatchError(ContainSubstring("not yet been initialized")))
		})

		It("should fail if the taint is still present", func(ctx SpecContext) {
			node.Spec.ProviderID = "machine-" + hostName
			node.Spec.Taints = []corev1.Taint{cloudTaint}
			Expect(fakeClient.Create(ctx, node)).To(Succeed())

			Expect(b.WaitUntilControlPlaneNodeIsInitializedByCloudProvider(ctx)).To(MatchError(ContainSubstring("not yet been initialized")))
		})

		It("should succeed if the node has been initialized", func(ctx SpecContext) {
			node.Spec.ProviderID = "machine-" + hostName
			Expect(fakeClient.Create(ctx, node)).To(Succeed())

			Expect(b.WaitUntilControlPlaneNodeIsInitializedByCloudProvider(ctx)).To(Succeed())
		})
	})
})

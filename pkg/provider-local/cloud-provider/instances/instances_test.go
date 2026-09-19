// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package instances_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cloudprovider "k8s.io/cloud-provider"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	cloudproviderv1alpha1 "github.com/gardener/gardener/pkg/provider-local/cloud-provider/api/v1alpha1"
	. "github.com/gardener/gardener/pkg/provider-local/cloud-provider/instances"
)

var _ = Describe("Provider", func() {
	const namespace = "infra-shoot--foo--bar"

	var (
		fakeClient client.Client
		provider   *Provider

		node *corev1.Node
		pod  *corev1.Pod
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		provider = &Provider{
			Config: &cloudproviderv1alpha1.CloudProviderConfig{
				RuntimeCluster: &cloudproviderv1alpha1.RuntimeCluster{Namespace: namespace},
			},
			RuntimeClient: fakeClient,
		}

		node = &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "machine-shoot--foo--bar-worker-z1-abcde-12345"}}
		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      node.Name,
				Namespace: namespace,
				Labels: map[string]string{
					"app":              "machine",
					"machine-provider": "local",
					"local.provider.extensions.gardener.cloud/zone":          "local-a",
					"local.provider.extensions.gardener.cloud/region":        "local",
					"local.provider.extensions.gardener.cloud/instance-type": "local",
				},
			},
			Status: corev1.PodStatus{
				Phase:  corev1.PodRunning,
				PodIPs: []corev1.PodIP{{IP: "10.0.1.5"}, {IP: "fd00:10:1:100::5"}},
			},
		}
	})

	Describe("#InstanceExists", func() {
		It("should return false if the machine pod does not exist", func(ctx SpecContext) {
			Expect(provider.InstanceExists(ctx, node)).To(BeFalse())
		})

		It("should return false if the pod is not a machine pod", func(ctx SpecContext) {
			pod.Labels = nil
			Expect(fakeClient.Create(ctx, pod)).To(Succeed())

			Expect(provider.InstanceExists(ctx, node)).To(BeFalse())
		})

		It("should return true if the machine pod exists", func(ctx SpecContext) {
			Expect(fakeClient.Create(ctx, pod)).To(Succeed())

			Expect(provider.InstanceExists(ctx, node)).To(BeTrue())
		})

		It("should look up the machine pod by provider ID if set", func(ctx SpecContext) {
			pod.Name = "other-machine"
			Expect(fakeClient.Create(ctx, pod)).To(Succeed())

			Expect(provider.InstanceExists(ctx, node)).To(BeFalse())

			node.Spec.ProviderID = "other-machine"
			Expect(provider.InstanceExists(ctx, node)).To(BeTrue())
		})

		It("should return an error if the runtime cluster is not configured", func(ctx SpecContext) {
			provider.Config.RuntimeCluster = nil

			_, err := provider.InstanceExists(ctx, node)
			Expect(err).To(MatchError(ContainSubstring("runtime cluster is not configured")))
		})
	})

	Describe("#InstanceShutdown", func() {
		It("should return InstanceNotFound if the machine pod does not exist", func(ctx SpecContext) {
			_, err := provider.InstanceShutdown(ctx, node)
			Expect(err).To(MatchError(cloudprovider.InstanceNotFound))
		})

		It("should return false if the machine pod is running", func(ctx SpecContext) {
			Expect(fakeClient.Create(ctx, pod)).To(Succeed())

			Expect(provider.InstanceShutdown(ctx, node)).To(BeFalse())
		})

		It("should return false if the machine pod is pending", func(ctx SpecContext) {
			pod.Status.Phase = corev1.PodPending
			Expect(fakeClient.Create(ctx, pod)).To(Succeed())

			Expect(provider.InstanceShutdown(ctx, node)).To(BeFalse())
		})

		It("should return true if the machine pod has terminated", func(ctx SpecContext) {
			pod.Status.Phase = corev1.PodFailed
			Expect(fakeClient.Create(ctx, pod)).To(Succeed())

			Expect(provider.InstanceShutdown(ctx, node)).To(BeTrue())
		})

		It("should return true if the machine pod is terminating", func(ctx SpecContext) {
			pod.Finalizers = []string{"test.gardener.cloud/keep"}
			Expect(fakeClient.Create(ctx, pod)).To(Succeed())
			Expect(fakeClient.Delete(ctx, pod)).To(Succeed())

			Expect(provider.InstanceShutdown(ctx, node)).To(BeTrue())
		})
	})

	Describe("#InstanceMetadata", func() {
		It("should return InstanceNotFound if the machine pod does not exist", func(ctx SpecContext) {
			_, err := provider.InstanceMetadata(ctx, node)
			Expect(err).To(MatchError(cloudprovider.InstanceNotFound))
		})

		It("should return the metadata of the machine pod", func(ctx SpecContext) {
			Expect(fakeClient.Create(ctx, pod)).To(Succeed())

			Expect(provider.InstanceMetadata(ctx, node)).To(Equal(&cloudprovider.InstanceMetadata{
				ProviderID:   pod.Name,
				InstanceType: "local",
				NodeAddresses: []corev1.NodeAddress{
					{Type: corev1.NodeInternalIP, Address: "10.0.1.5"},
					{Type: corev1.NodeInternalIP, Address: "fd00:10:1:100::5"},
					{Type: corev1.NodeHostName, Address: pod.Name},
				},
				Zone:   "local-a",
				Region: "local",
			}))
		})

		It("should not return labels the machine pod does not have", func(ctx SpecContext) {
			pod.Labels = map[string]string{"app": "machine"}
			Expect(fakeClient.Create(ctx, pod)).To(Succeed())

			metadata, err := provider.InstanceMetadata(ctx, node)
			Expect(err).NotTo(HaveOccurred())
			Expect(metadata.ProviderID).To(Equal(pod.Name))
			Expect(metadata.InstanceType).To(BeEmpty())
			Expect(metadata.Zone).To(BeEmpty())
			Expect(metadata.Region).To(BeEmpty())
		})

		It("should prefer IPv6 addresses if the node's primary internal IP is an IPv6 address", func(ctx SpecContext) {
			node.Status.Addresses = []corev1.NodeAddress{
				{Type: corev1.NodeHostName, Address: node.Name},
				{Type: corev1.NodeInternalIP, Address: "fd00:10:1:100::5"},
			}
			Expect(fakeClient.Create(ctx, pod)).To(Succeed())

			metadata, err := provider.InstanceMetadata(ctx, node)
			Expect(err).NotTo(HaveOccurred())
			Expect(metadata.NodeAddresses).To(Equal([]corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "fd00:10:1:100::5"},
				{Type: corev1.NodeInternalIP, Address: "10.0.1.5"},
				{Type: corev1.NodeHostName, Address: pod.Name},
			}))
		})

		It("should not return any addresses if the machine pod has no IPs yet", func(ctx SpecContext) {
			pod.Status.PodIPs = nil
			Expect(fakeClient.Create(ctx, pod)).To(Succeed())

			metadata, err := provider.InstanceMetadata(ctx, node)
			Expect(err).NotTo(HaveOccurred())
			Expect(metadata.NodeAddresses).To(BeEmpty())
		})
	})
})

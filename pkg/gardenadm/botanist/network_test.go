// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Network", func() {
	var (
		ctx           context.Context
		namespaceName = "kube-system"

		b *GardenadmBotanist
	)

	BeforeEach(func() {
		ctx = context.Background()

		b = &GardenadmBotanist{
			Botanist: &botanistpkg.Botanist{
				Operation: &operation.Operation{
					Shoot: &shoot.Shoot{
						ControlPlaneNamespace: namespaceName,
						Networks: &shoot.Networks{
							Pods:     []net.IPNet{{IP: net.ParseIP("10.1.2.3"), Mask: net.CIDRMask(8, 32)}},
							Services: []net.IPNet{{IP: net.ParseIP("10.4.5.6"), Mask: net.CIDRMask(8, 32)}},
							Nodes:    []net.IPNet{{IP: net.ParseIP("10.7.8.9"), Mask: net.CIDRMask(8, 32)}},
						},
					},
					SeedClientSet: fakekubernetes.
						NewClientSetBuilder().
						WithClient(fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()).
						Build(),
				},
			},
		}
		b.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Networking: &gardencorev1beta1.Networking{
					IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4},
				},
			},
		})
	})

	Describe("#IsPodNetworkAvailable", func() {
		var (
			hostName = "foo"

			node *corev1.Node
		)

		BeforeEach(func() {
			node = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "node-",
					Labels:       map[string]string{"kubernetes.io/hostname": hostName},
				},
			}
			b.HostName = hostName
		})

		It("should return false because the Node does not exist", func() {
			available, err := b.IsPodNetworkAvailable(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(available).To(BeFalse())
		})

		It("should return an error when it fails fetching node object by hostname", func() {
			node2 := node.DeepCopy()

			Expect(b.SeedClientSet.Client().Create(ctx, node)).To(Succeed())
			Expect(b.SeedClientSet.Client().Create(ctx, node2)).To(Succeed())

			available, err := b.IsPodNetworkAvailable(ctx)
			Expect(err).To(MatchError(ContainSubstring("failed fetching node object by hostname")))
			Expect(available).To(BeFalse())
		})

		It("should return false because the Node's NetworkUnavailable condition is true", func() {
			node.Status.Conditions = []corev1.NodeCondition{
				{
					Type:   corev1.NodeNetworkUnavailable,
					Status: corev1.ConditionTrue,
				},
			}
			Expect(b.SeedClientSet.Client().Create(ctx, node)).To(Succeed())

			available, err := b.IsPodNetworkAvailable(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(available).To(BeFalse())
		})

		It("should return true because the Node's NetworkUnavailable condition is false", func() {
			node.Status.Conditions = []corev1.NodeCondition{
				{
					Type:   corev1.NodeNetworkUnavailable,
					Status: corev1.ConditionFalse,
				},
			}
			Expect(b.SeedClientSet.Client().Create(ctx, node)).To(Succeed())

			available, err := b.IsPodNetworkAvailable(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(available).To(BeTrue())
		})

		It("should return false because the Node's does not have a NetworkUnavailable condition", func() {
			node.Status.Conditions = []corev1.NodeCondition{}
			Expect(b.SeedClientSet.Client().Create(ctx, node)).To(Succeed())

			available, err := b.IsPodNetworkAvailable(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(available).To(BeFalse())
		})
	})

	Describe("#MachineIP", func() {
		var ipv4, ipv6 net.IP

		BeforeEach(func() {
			ipv4 = net.ParseIP("1.2.3.4").To4()
			ipv6 = net.ParseIP("::1")
			b.HostName = "some-host"
		})

		It("should return an IPv4 address when IPv4 is the primary IP family", func() {
			b.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Networking: &gardencorev1beta1.Networking{
						IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4},
					},
				},
			})
			DeferCleanup(test.WithVar(&LookupIP, func(_ string) ([]net.IP, error) {
				return []net.IP{ipv6, ipv4}, nil
			}))

			ip, err := b.MachineIP()
			Expect(err).NotTo(HaveOccurred())
			Expect(ip.To4()).NotTo(BeNil(), "expected an IPv4 address")
		})

		It("should return an IPv6 address when IPv6 is the primary IP family", func() {
			b.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Networking: &gardencorev1beta1.Networking{
						IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv6},
					},
				},
			})
			DeferCleanup(test.WithVar(&LookupIP, func(_ string) ([]net.IP, error) {
				return []net.IP{ipv4, ipv6}, nil
			}))

			ip, err := b.MachineIP()
			Expect(err).NotTo(HaveOccurred())
			Expect(ip.To4()).To(BeNil(), "expected an IPv6 address")
		})

		It("should fall back to any available address when no address of the preferred family is found", func() {
			b.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Networking: &gardencorev1beta1.Networking{
						IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv6},
					},
				},
			})
			DeferCleanup(test.WithVar(&LookupIP, func(_ string) ([]net.IP, error) {
				return []net.IP{ipv4}, nil
			}))

			ip, err := b.MachineIP()
			Expect(err).NotTo(HaveOccurred())
			Expect(ip.Equal(ipv4)).To(BeTrue())
		})

		It("should return an error when no IP address is found", func() {
			DeferCleanup(test.WithVar(&LookupIP, func(_ string) ([]net.IP, error) {
				return nil, nil
			}))

			ip, err := b.MachineIP()
			Expect(err).To(MatchError("no IP address found for node"))
			Expect(ip).To(BeNil())
		})
	})

	Describe("#ApplyNetworkPolicies", func() {
		It("should apply the NetworkPolicies", func() {
			namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default", Labels: map[string]string{"gardener.cloud/role": "shoot"}}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}}
			Expect(b.SeedClientSet.Client().Create(ctx, namespace)).To(Succeed())

			endpoints := &corev1.Endpoints{ObjectMeta: metav1.ObjectMeta{Name: "kubernetes", Namespace: "default"}}
			Expect(b.SeedClientSet.Client().Create(ctx, endpoints)).To(Succeed())

			networkPolicyList := &networkingv1.NetworkPolicyList{}
			Expect(b.SeedClientSet.Client().List(ctx, networkPolicyList)).To(Succeed())
			Expect(networkPolicyList.Items).To(BeEmpty())

			Expect(b.ApplyNetworkPolicies(ctx)).To(Succeed())

			Expect(b.SeedClientSet.Client().List(ctx, networkPolicyList)).To(Succeed())
			Expect(networkPolicyList.Items).NotTo(BeEmpty())
		})
	})
})

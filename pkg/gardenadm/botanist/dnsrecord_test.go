// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	mockdnsrecord "github.com/gardener/gardener/pkg/component/extensions/dnsrecord/mock"
	"github.com/gardener/gardener/pkg/component/extensions/selfhostedshootexposure"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("DNSRecord", func() {
	const controlPlaneWorkerPoolName = "control-plane"

	var (
		b *GardenadmBotanist

		fakeClient client.Client

		externalDNSRecord       *mockdnsrecord.MockInterface
		selfHostedShootExposure *selfhostedshootexposure.SelfHostedShootExposure
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeClientSet := fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()

		ctrl := gomock.NewController(GinkgoT())
		externalDNSRecord = mockdnsrecord.NewMockInterface(ctrl)
		selfHostedShootExposure = selfhostedshootexposure.New(logr.Discard(), fakeClient, &selfhostedshootexposure.Values{})

		b = &GardenadmBotanist{
			Botanist: &botanistpkg.Botanist{
				Operation: &operation.Operation{
					SeedClientSet:  fakeClientSet,
					ShootClientSet: fakeClientSet,
					Shoot: &shoot.Shoot{
						ControlPlaneNamespace: "kube-system",
						Components: &shoot.Components{
							Extensions: &shoot.Extensions{
								ExternalDNSRecord:       externalDNSRecord,
								SelfHostedShootExposure: selfHostedShootExposure,
							},
						},
					},
				},
			},
		}

		b.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Provider: gardencorev1beta1.Provider{
					Workers: []gardencorev1beta1.Worker{{
						Name:         controlPlaneWorkerPoolName,
						ControlPlane: &gardencorev1beta1.WorkerControlPlane{},
					}},
				},
				Networking: &gardencorev1beta1.Networking{
					IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4},
				},
			},
		})
		b.Shoot.SetShootState(&gardencorev1beta1.ShootState{})
	})

	// setExtensionExposure configures the shoot info to have extension-based exposure.
	setExtensionExposure := func(ipFamilies ...gardencorev1beta1.IPFamily) {
		if len(ipFamilies) == 0 {
			ipFamilies = []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4}
		}
		exposureType := "local"
		shoot := b.Shoot.GetInfo()
		shoot.Spec.Provider.Workers[0].ControlPlane.Exposure = &gardencorev1beta1.Exposure{
			Extension: &gardencorev1beta1.ExtensionExposure{Type: &exposureType},
		}
		if shoot.Spec.Networking == nil {
			shoot.Spec.Networking = &gardencorev1beta1.Networking{}
		}
		shoot.Spec.Networking.IPFamilies = ipFamilies
		b.Shoot.SetInfo(shoot)
	}

	// healthyControlPlaneNode returns a control-plane Node that passes health.CheckNode with the given addresses.
	healthyControlPlaneNode := func(name string, addresses ...corev1.NodeAddress) *corev1.Node {
		return &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: map[string]string{"node-role.kubernetes.io/control-plane": ""},
			},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
				Addresses:  addresses,
			},
		}
	}

	Describe("#RestoreExternalDNSRecord", func() {
		It("should fail if there are no control plane nodes", func(ctx SpecContext) {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "worker-1",
					Labels: map[string]string{
						"worker.gardener.cloud/pool": "worker",
					},
				},
			}
			Expect(fakeClient.Create(ctx, node)).To(Succeed())

			Expect(b.RestoreExternalDNSRecord(ctx)).To(MatchError("no control plane nodes found"))
		})

		It("should fail if a control plane node has no addresses", func(ctx SpecContext) {
			node := healthyControlPlaneNode(controlPlaneWorkerPoolName + "-1")
			Expect(fakeClient.Create(ctx, node)).To(Succeed())

			Expect(b.RestoreExternalDNSRecord(ctx)).To(MatchError(ContainSubstring("has no addresses")))
		})

		It("should fail if no node address matches the configured IP family", func(ctx SpecContext) {
			shoot := b.Shoot.GetInfo()
			shoot.Spec.Networking.IPFamilies = []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv6}
			b.Shoot.SetInfo(shoot)

			node := healthyControlPlaneNode(controlPlaneWorkerPoolName+"-1", corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: "10.0.0.1"})
			Expect(fakeClient.Create(ctx, node)).To(Succeed())

			Expect(b.RestoreExternalDNSRecord(ctx)).To(MatchError(ContainSubstring("configured IP family")))
		})

		It("should set the preferred (internal first) addresses and restore the DNSRecord", func(ctx SpecContext) {
			// The internal address is preferred so the record stays resolvable within the cluster network even when
			// the nodes have no externally routable addresses.
			node1 := healthyControlPlaneNode(controlPlaneWorkerPoolName+"-1",
				corev1.NodeAddress{Type: corev1.NodeExternalIP, Address: "1.2.3.4"},
				corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: "10.0.0.1"},
			)
			node2 := healthyControlPlaneNode(controlPlaneWorkerPoolName+"-2", corev1.NodeAddress{Type: corev1.NodeExternalIP, Address: "1.2.3.5"})

			Expect(fakeClient.Create(ctx, node1)).To(Succeed())
			Expect(fakeClient.Create(ctx, node2)).To(Succeed())

			externalDNSRecord.EXPECT().SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
			externalDNSRecord.EXPECT().SetValues([]string{"10.0.0.1", "1.2.3.5"})
			externalDNSRecord.EXPECT().Restore(gomock.Any(), gomock.Any()).Return(nil)
			externalDNSRecord.EXPECT().Wait(gomock.Any()).Return(nil)

			Expect(b.RestoreExternalDNSRecord(ctx)).To(Succeed())
		})

		It("should include unhealthy control plane nodes (restore is a one-shot at bootstrap)", func(ctx SpecContext) {
			node := healthyControlPlaneNode(controlPlaneWorkerPoolName+"-1", corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: "10.0.0.1"})
			node.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionFalse}}
			Expect(fakeClient.Create(ctx, node)).To(Succeed())

			externalDNSRecord.EXPECT().SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
			externalDNSRecord.EXPECT().SetValues([]string{"10.0.0.1"})
			externalDNSRecord.EXPECT().Restore(gomock.Any(), gomock.Any()).Return(nil)
			externalDNSRecord.EXPECT().Wait(gomock.Any()).Return(nil)

			Expect(b.RestoreExternalDNSRecord(ctx)).To(Succeed())
		})

		It("should filter dual-stack addresses to the first IP family", func(ctx SpecContext) {
			shoot := b.Shoot.GetInfo()
			shoot.Spec.Networking.IPFamilies = []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4, gardencorev1beta1.IPFamilyIPv6}
			b.Shoot.SetInfo(shoot)

			node1 := healthyControlPlaneNode(controlPlaneWorkerPoolName+"-1", corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: "10.0.0.1"})
			node2 := healthyControlPlaneNode(controlPlaneWorkerPoolName+"-2", corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: "fd12::1"})
			Expect(fakeClient.Create(ctx, node1)).To(Succeed())
			Expect(fakeClient.Create(ctx, node2)).To(Succeed())

			externalDNSRecord.EXPECT().SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
			externalDNSRecord.EXPECT().SetValues([]string{"10.0.0.1"})
			externalDNSRecord.EXPECT().Restore(gomock.Any(), gomock.Any()).Return(nil)
			externalDNSRecord.EXPECT().Wait(gomock.Any()).Return(nil)

			Expect(b.RestoreExternalDNSRecord(ctx)).To(Succeed())
		})

		Context("with extension-based exposure", func() {
			BeforeEach(func() {
				setExtensionExposure()
			})

			It("should fail if SelfHostedShootExposure has no ingress yet", func(ctx SpecContext) {
				Expect(b.RestoreExternalDNSRecord(ctx)).To(MatchError("exposure has no ingress yet"))
			})

			It("should fail if all ingress entries have neither IP nor hostname", func(ctx SpecContext) {
				selfHostedShootExposure.Ingress = []corev1.LoadBalancerIngress{{IP: "", Hostname: ""}}

				Expect(b.RestoreExternalDNSRecord(ctx)).To(MatchError(ContainSubstring("neither IPs nor hostnames")))
			})

			It("should use IPs when available", func(ctx SpecContext) {
				selfHostedShootExposure.Ingress = []corev1.LoadBalancerIngress{
					{IP: "10.0.0.1"},
					{IP: "10.0.0.2"},
				}

				externalDNSRecord.EXPECT().SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
				externalDNSRecord.EXPECT().SetValues([]string{"10.0.0.1", "10.0.0.2"})
				externalDNSRecord.EXPECT().Restore(gomock.Any(), gomock.Any()).Return(nil)
				externalDNSRecord.EXPECT().Wait(gomock.Any()).Return(nil)

				Expect(b.RestoreExternalDNSRecord(ctx)).To(Succeed())
			})

			It("should fall back to hostnames when no IPs are present", func(ctx SpecContext) {
				selfHostedShootExposure.Ingress = []corev1.LoadBalancerIngress{
					{Hostname: "lb.example.com"},
				}

				externalDNSRecord.EXPECT().SetRecordType(extensionsv1alpha1.DNSRecordTypeCNAME)
				externalDNSRecord.EXPECT().SetValues([]string{"lb.example.com"})
				externalDNSRecord.EXPECT().Restore(gomock.Any(), gomock.Any()).Return(nil)
				externalDNSRecord.EXPECT().Wait(gomock.Any()).Return(nil)

				Expect(b.RestoreExternalDNSRecord(ctx)).To(Succeed())
			})

			It("should prefer IPs over hostnames when both are present across entries", func(ctx SpecContext) {
				selfHostedShootExposure.Ingress = []corev1.LoadBalancerIngress{
					{IP: "10.0.0.1"},
					{Hostname: "lb.example.com"},
				}

				externalDNSRecord.EXPECT().SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
				externalDNSRecord.EXPECT().SetValues([]string{"10.0.0.1"})
				externalDNSRecord.EXPECT().Restore(gomock.Any(), gomock.Any()).Return(nil)
				externalDNSRecord.EXPECT().Wait(gomock.Any()).Return(nil)

				Expect(b.RestoreExternalDNSRecord(ctx)).To(Succeed())
			})

			It("should filter dual-stack IPs to the first IP family", func(ctx SpecContext) {
				setExtensionExposure(gardencorev1beta1.IPFamilyIPv4, gardencorev1beta1.IPFamilyIPv6)
				selfHostedShootExposure.Ingress = []corev1.LoadBalancerIngress{
					{IP: "10.0.0.1"},
					{IP: "fd12::1"},
				}

				externalDNSRecord.EXPECT().SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
				externalDNSRecord.EXPECT().SetValues([]string{"10.0.0.1"})
				externalDNSRecord.EXPECT().Restore(gomock.Any(), gomock.Any()).Return(nil)
				externalDNSRecord.EXPECT().Wait(gomock.Any()).Return(nil)

				Expect(b.RestoreExternalDNSRecord(ctx)).To(Succeed())
			})

			It("should fall back to the second IP family when no IPs match the first", func(ctx SpecContext) {
				setExtensionExposure(gardencorev1beta1.IPFamilyIPv6, gardencorev1beta1.IPFamilyIPv4)
				selfHostedShootExposure.Ingress = []corev1.LoadBalancerIngress{
					{IP: "10.0.0.1"},
				}

				externalDNSRecord.EXPECT().SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
				externalDNSRecord.EXPECT().SetValues([]string{"10.0.0.1"})
				externalDNSRecord.EXPECT().Restore(gomock.Any(), gomock.Any()).Return(nil)
				externalDNSRecord.EXPECT().Wait(gomock.Any()).Return(nil)

				Expect(b.RestoreExternalDNSRecord(ctx)).To(Succeed())
			})

			It("should fail if ingress IPs do not match any configured IP family", func(ctx SpecContext) {
				setExtensionExposure(gardencorev1beta1.IPFamilyIPv6)
				selfHostedShootExposure.Ingress = []corev1.LoadBalancerIngress{
					{IP: "10.0.0.1"},
				}

				Expect(b.RestoreExternalDNSRecord(ctx)).To(MatchError(ContainSubstring("configured IP family")))
			})
		})
	})
})

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
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
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("DNSRecord", func() {
	const controlPlaneWorkerPoolName = "control-plane"

	var (
		b *GardenadmBotanist

		fakeClient client.Client

		externalDNSRecord *mockdnsrecord.MockInterface
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeClientSet := fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()

		ctrl := gomock.NewController(GinkgoT())
		externalDNSRecord = mockdnsrecord.NewMockInterface(ctrl)

		b = &GardenadmBotanist{
			Botanist: &botanistpkg.Botanist{Operation: &operation.Operation{
				SeedClientSet:  fakeClientSet,
				ShootClientSet: fakeClientSet,
				Shoot: &shoot.Shoot{
					ControlPlaneNamespace: "kube-system",
					Components: &shoot.Components{
						Extensions: &shoot.Extensions{
							ExternalDNSRecord: externalDNSRecord,
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
			},
		})
		b.Shoot.SetShootState(&gardencorev1beta1.ShootState{})
	})

	Describe("#RestoreExternalDNSRecord", func() {
		It("should fail if there is no control plane worker pool", func(ctx SpecContext) {
			shoot := b.Shoot.GetInfo()
			shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{{
				Name: "worker",
			}}
			b.Shoot.SetInfo(shoot)

			Expect(b.RestoreExternalDNSRecord(ctx)).To(MatchError("failed fetching the control plane worker pool for the shoot"))
		})

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

			Expect(b.RestoreExternalDNSRecord(ctx)).To(MatchError("no control plane nodes founds"))
		})

		It("should fail if the control plane node doesn't have any addresses", func(ctx SpecContext) {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: controlPlaneWorkerPoolName + "-1",
					Labels: map[string]string{
						"worker.gardener.cloud/pool": controlPlaneWorkerPoolName,
					},
				},
			}
			Expect(fakeClient.Create(ctx, node)).To(Succeed())

			Expect(b.RestoreExternalDNSRecord(ctx)).To(MatchError(ContainSubstring("no addresses found in status")))
		})

		It("should fail if the control plane nodes have different address types", func(ctx SpecContext) {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: controlPlaneWorkerPoolName + "-1",
					Labels: map[string]string{
						"worker.gardener.cloud/pool": controlPlaneWorkerPoolName,
					},
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{{
						Type:    corev1.NodeInternalIP,
						Address: "10.0.0.1",
					}},
				},
			}

			node2 := node.DeepCopy()
			node2.Name = controlPlaneWorkerPoolName + "-2"
			node2.Status.Addresses = []corev1.NodeAddress{{
				Type:    corev1.NodeHostName,
				Address: node2.Name,
			}}

			Expect(fakeClient.Create(ctx, node)).To(Succeed())
			Expect(fakeClient.Create(ctx, node2)).To(Succeed())

			Expect(b.RestoreExternalDNSRecord(ctx)).To(MatchError(ContainSubstring("inconsistent address types")))
		})

		It("should set the correct values and restore the DNSRecord", func(ctx SpecContext) {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: controlPlaneWorkerPoolName + "-1",
					Labels: map[string]string{
						"worker.gardener.cloud/pool": controlPlaneWorkerPoolName,
					},
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{{
						Type:    corev1.NodeInternalIP,
						Address: "10.0.0.1",
					}},
				},
			}

			node2 := node.DeepCopy()
			node2.Name = controlPlaneWorkerPoolName + "-2"
			node2.Status.Addresses[0].Address = "10.0.0.2"

			Expect(fakeClient.Create(ctx, node)).To(Succeed())
			Expect(fakeClient.Create(ctx, node2)).To(Succeed())

			externalDNSRecord.EXPECT().SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
			externalDNSRecord.EXPECT().SetValues([]string{node.Status.Addresses[0].Address, node2.Status.Addresses[0].Address})
			externalDNSRecord.EXPECT().Restore(gomock.Any(), gomock.Any()).Return(nil)
			externalDNSRecord.EXPECT().Wait(gomock.Any()).Return(nil)

			Expect(b.RestoreExternalDNSRecord(ctx)).To(Succeed())
		})
	})
})

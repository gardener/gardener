// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"
	"net"
	"strconv"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	mocknodelocaldns "github.com/gardener/gardener/pkg/component/networking/nodelocaldns/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("NodeLocalDNS", func() {
	var (
		ctrl     *gomock.Controller
		botanist *Botanist
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		botanist = &Botanist{Operation: &operation.Operation{}}
		botanist.Shoot = &shootpkg.Shoot{}
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				SystemComponents: &gardencorev1beta1.SystemComponents{
					NodeLocalDNS: &gardencorev1beta1.NodeLocalDNS{
						Enabled: true,
					},
				},
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.35.0",
				},
				Networking: &gardencorev1beta1.Networking{IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4}},
				Provider: gardencorev1beta1.Provider{
					Workers: []gardencorev1beta1.Worker{
						{
							Kubernetes: &gardencorev1beta1.WorkerKubernetes{
								Version: new("1.35.0"),
							},
							Name: "worker-aaaa",
							Machine: gardencorev1beta1.Machine{
								Type: "machine-type",
							},
							Minimum: 1,
							Maximum: 3,
						},
					},
				},
			},
		})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultNodeLocalDNS", func() {
		BeforeEach(func() {
			fakeClient := fakeclient.NewClientBuilder().Build()
			botanist.SeedClientSet = fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()
		})

		It("should successfully create a node-local-dns interface", func() {
			nodeLocalDNS, err := botanist.DefaultNodeLocalDNS()
			Expect(nodeLocalDNS).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#ReconcileNodeLocalDNS", func() {
		var (
			nodelocaldns *mocknodelocaldns.MockInterface
			seedClient   client.Client
			shootClient  client.Client

			ctx     = context.TODO()
			fakeErr = errors.New("fake err")
		)

		BeforeEach(func() {
			nodelocaldns = mocknodelocaldns.NewMockInterface(ctrl)
			seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			shootClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()

			nodelocaldns.EXPECT().SetShootClientSet(gomock.Any())
			botanist.ShootClientSet = fakekubernetes.NewClientSetBuilder().WithClient(shootClient).Build()
			botanist.Shoot.Components = &shootpkg.Components{
				SystemComponents: &shootpkg.SystemComponents{
					NodeLocalDNS: nodelocaldns,
				},
			}
			botanist.Shoot.Networks = &shootpkg.Networks{
				CoreDNS: []net.IP{net.ParseIP("18.19.20.21"), net.ParseIP("2001:db8::10")},
			}
			botanist.Shoot.NodeLocalDNSEnabled = true

			nodelocaldns.EXPECT().SetClusterDNS([]string{"__PILLAR__CLUSTER__DNS__"})
			nodelocaldns.EXPECT().SetDNSServers([]string{botanist.Shoot.Networks.CoreDNS[0].String(), botanist.Shoot.Networks.CoreDNS[1].String()})
		})

		It("should fail when the deploy function fails", func() {
			nodelocaldns.EXPECT().SetIPFamilies([]gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4})
			nodelocaldns.EXPECT().SetWorkerPoolNames([]string{"worker-aaaa"})
			nodelocaldns.EXPECT().Deploy(ctx).Return(fakeErr)

			Expect(botanist.ReconcileNodeLocalDNS(ctx)).To(MatchError(fakeErr))
		})

		It("should successfully deploy when enabled", func() {
			nodelocaldns.EXPECT().SetIPFamilies([]gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4})
			nodelocaldns.EXPECT().SetWorkerPoolNames([]string{"worker-aaaa"})
			nodelocaldns.EXPECT().Deploy(ctx)

			Expect(botanist.ReconcileNodeLocalDNS(ctx)).To(Succeed())
		})

		It("should include worker pool names of stale nodes (e.g. after a pool rename)", func() {
			oldNode := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "old-node",
					Labels: map[string]string{v1beta1constants.LabelWorkerPool: "worker-old"},
				},
			}
			newNode := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "new-node",
					Labels: map[string]string{v1beta1constants.LabelWorkerPool: "worker-aaaa"},
				},
			}
			Expect(shootClient.Create(ctx, oldNode)).To(Succeed())
			Expect(shootClient.Create(ctx, newNode)).To(Succeed())

			nodelocaldns.EXPECT().SetIPFamilies([]gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4})
			nodelocaldns.EXPECT().SetWorkerPoolNames([]string{"worker-aaaa", "worker-old"})
			nodelocaldns.EXPECT().Deploy(ctx)

			Expect(botanist.ReconcileNodeLocalDNS(ctx)).To(Succeed())
		})

		It("should successfully deploy when enabled with ipfamily IPv6", func() {
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					SystemComponents: &gardencorev1beta1.SystemComponents{
						NodeLocalDNS: &gardencorev1beta1.NodeLocalDNS{
							Enabled: true,
						},
					},
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.28.1",
					},
					Networking: &gardencorev1beta1.Networking{IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv6}},
				},
			})
			nodelocaldns.EXPECT().SetIPFamilies([]gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv6})
			nodelocaldns.EXPECT().SetWorkerPoolNames([]string{})
			nodelocaldns.EXPECT().Deploy(ctx)

			Expect(botanist.ReconcileNodeLocalDNS(ctx)).To(Succeed())
		})

		Context("node-local-dns disabled", func() {
			BeforeEach(func() {
				botanist.Shoot.NodeLocalDNSEnabled = false
				botanist.Shoot.KubernetesVersion, _ = semver.NewVersion("1.30.1")
				nodelocaldns.EXPECT().SetIPFamilies([]gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4})
			})

			Context("but still node with label existing", func() {
				It("label enabled", func() {
					node := corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "node",
							Labels: map[string]string{v1beta1constants.LabelNodeLocalDNS: strconv.FormatBool(true)},
						},
					}
					Expect(seedClient.Create(ctx, &node)).To(Succeed())
					nodelocaldns.EXPECT().Destroy(ctx).Return(nil)

					Expect(botanist.ReconcileNodeLocalDNS(ctx)).To(Succeed())
				})

				It("label disabled", func() {
					node := corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "node",
							Labels: map[string]string{v1beta1constants.LabelNodeLocalDNS: strconv.FormatBool(false)},
						},
					}
					Expect(seedClient.Create(ctx, &node)).To(Succeed())
					nodelocaldns.EXPECT().Destroy(ctx)

					Expect(botanist.ReconcileNodeLocalDNS(ctx)).To(Succeed())
				})
			})

			It("should fail when the destroy function fails", func() {
				nodelocaldns.EXPECT().Destroy(ctx).Return(fakeErr)

				Expect(botanist.ReconcileNodeLocalDNS(ctx)).To(MatchError(fakeErr))
			})

			It("should successfully destroy", func() {
				nodelocaldns.EXPECT().Destroy(ctx)

				Expect(botanist.ReconcileNodeLocalDNS(ctx)).To(Succeed())
			})
		})
	})
})

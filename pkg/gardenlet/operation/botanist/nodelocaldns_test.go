// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"
	"net"
	"strconv"

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
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
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
					Version: "1.30.1",
				},
				Networking: &gardencorev1beta1.Networking{IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4}},
			},
		})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultNodeLocalDNS", func() {
		var kubernetesClient *kubernetesmock.MockInterface

		BeforeEach(func() {
			kubernetesClient = kubernetesmock.NewMockInterface(ctrl)

			botanist.SeedClientSet = kubernetesClient
		})

		It("should successfully create a node-local-dns interface", func() {
			kubernetesClient.EXPECT().Client()

			nodeLocalDNS, err := botanist.DefaultNodeLocalDNS()
			Expect(nodeLocalDNS).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#ReconcileNodeLocalDNS", func() {
		var (
			nodelocaldns     *mocknodelocaldns.MockInterface
			kubernetesClient *kubernetesmock.MockInterface
			c                client.Client

			ctx     = context.TODO()
			fakeErr = errors.New("fake err")
		)

		BeforeEach(func() {
			nodelocaldns = mocknodelocaldns.NewMockInterface(ctrl)
			kubernetesClient = kubernetesmock.NewMockInterface(ctrl)
			c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

			botanist.ShootClientSet = kubernetesClient
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
			nodelocaldns.EXPECT().Deploy(ctx).Return(fakeErr)

			Expect(botanist.ReconcileNodeLocalDNS(ctx)).To(MatchError(fakeErr))
		})

		It("should successfully deploy when enabled", func() {
			nodelocaldns.EXPECT().SetIPFamilies([]gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4})
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
			nodelocaldns.EXPECT().Deploy(ctx)

			Expect(botanist.ReconcileNodeLocalDNS(ctx)).To(Succeed())
		})

		Context("node-local-dns disabled", func() {
			BeforeEach(func() {
				botanist.Shoot.NodeLocalDNSEnabled = false
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
					Expect(c.Create(ctx, &node)).To(Succeed())

					kubernetesClient.EXPECT().Client().Return(c).Times(2)

					Expect(botanist.ReconcileNodeLocalDNS(ctx)).To(Succeed())
				})

				It("label disabled", func() {
					node := corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "node",
							Labels: map[string]string{v1beta1constants.LabelNodeLocalDNS: strconv.FormatBool(false)},
						},
					}
					Expect(c.Create(ctx, &node)).To(Succeed())

					kubernetesClient.EXPECT().Client().Return(c).Times(2)

					nodelocaldns.EXPECT().Destroy(ctx)

					Expect(botanist.ReconcileNodeLocalDNS(ctx)).To(Succeed())
				})
			})

			It("should fail when the destroy function fails", func() {
				kubernetesClient.EXPECT().Client().Return(c).Times(2)

				nodelocaldns.EXPECT().Destroy(ctx).Return(fakeErr)

				Expect(botanist.ReconcileNodeLocalDNS(ctx)).To(MatchError(fakeErr))
			})

			It("should successfully destroy", func() {
				kubernetesClient.EXPECT().Client().Return(c).Times(2)

				nodelocaldns.EXPECT().Destroy(ctx)

				Expect(botanist.ReconcileNodeLocalDNS(ctx)).To(Succeed())
			})
		})
	})
})

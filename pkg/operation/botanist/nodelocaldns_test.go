// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist_test

import (
	"context"
	"fmt"
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
	mocknodelocaldns "github.com/gardener/gardener/pkg/component/nodelocaldns/mock"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
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
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					v1beta1constants.AnnotationNodeLocalDNS: "true",
				},
			},
			Spec: gardencorev1beta1.ShootSpec{
				SystemComponents: &gardencorev1beta1.SystemComponents{
					NodeLocalDNS: &gardencorev1beta1.NodeLocalDNS{
						Enabled: true,
					},
				},
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.22.1",
				},
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
			botanist.Shoot.Networks = &shootpkg.Networks{
				CoreDNS: net.ParseIP("18.19.20.21"),
			}
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
			fakeErr = fmt.Errorf("fake err")
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
			botanist.Shoot.NodeLocalDNSEnabled = true
		})

		It("should fail when the deploy function fails", func() {
			nodelocaldns.EXPECT().Deploy(ctx).Return(fakeErr)

			Expect(botanist.ReconcileNodeLocalDNS(ctx)).To(MatchError(fakeErr))
		})

		It("should successfully deploy when enabled", func() {
			nodelocaldns.EXPECT().Deploy(ctx)

			Expect(botanist.ReconcileNodeLocalDNS(ctx)).To(Succeed())
		})

		Context("node-local-dns disabled", func() {
			BeforeEach(func() {
				botanist.Shoot.NodeLocalDNSEnabled = false
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

					kubernetesClient.EXPECT().Client().Return(c)

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

					kubernetesClient.EXPECT().Client().Return(c)

					nodelocaldns.EXPECT().Destroy(ctx)

					Expect(botanist.ReconcileNodeLocalDNS(ctx)).To(Succeed())
				})
			})

			It("should fail when the destroy function fails", func() {
				kubernetesClient.EXPECT().Client().Return(c)

				nodelocaldns.EXPECT().Destroy(ctx).Return(fakeErr)

				Expect(botanist.ReconcileNodeLocalDNS(ctx)).To(MatchError(fakeErr))
			})

			It("should successfully destroy", func() {
				kubernetesClient.EXPECT().Client().Return(c)

				nodelocaldns.EXPECT().Destroy(ctx)

				Expect(botanist.ReconcileNodeLocalDNS(ctx)).To(Succeed())
			})
		})
	})
})

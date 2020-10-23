// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"time"

	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mock "github.com/gardener/gardener/pkg/mock/gardener/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/botanist"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("waiter", func() {

	var (
		ctrl                  *gomock.Controller
		k8sShootClient        *mock.MockInterface
		k8sShootRuntimeClient *mockclient.MockClient
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		k8sShootClient = mock.NewMockInterface(ctrl)
		k8sShootRuntimeClient = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("#WaitUntilShootLoadBalancerIsReady", func() {
		var (
			key = client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: "load-balancer"}
			b   botanist.Botanist
		)

		BeforeEach(func() {
			op := &operation.Operation{
				K8sShootClient: k8sShootClient,
				Logger:         logrus.NewEntry(logger.NewNopLogger()),
			}
			b = botanist.Botanist{Operation: op}
		})

		It("should return nil when the Service has .status.loadBalancer.ingress[]", func() {
			var (
				ctx = context.TODO()
				svc = &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "load-balancer",
						Namespace: metav1.NamespaceSystem,
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{
									Hostname: "cluster.local",
								},
							},
						},
					},
				}
			)

			gomock.InOrder(
				k8sShootClient.EXPECT().Client().Return(k8sShootRuntimeClient),
				k8sShootRuntimeClient.EXPECT().Get(gomock.Any(), key, gomock.AssignableToTypeOf(&corev1.Service{})).DoAndReturn(
					func(_ context.Context, _ client.ObjectKey, obj *corev1.Service) error {
						*obj = *svc
						return nil
					}),
			)

			actual, err := b.WaitUntilShootLoadBalancerIsReady(ctx, metav1.NamespaceSystem, "load-balancer", 1*time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(Equal("cluster.local"))
		})

		It("should return err when the Service has no .status.loadBalancer.ingress[]", func() {
			var (
				ctx = context.TODO()
				svc = &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "load-balancer",
						Namespace: metav1.NamespaceSystem,
					},
					Status: corev1.ServiceStatus{},
				}
				event = corev1.Event{
					Source:         corev1.EventSource{Component: "service-controller"},
					Message:        "Error syncing load balancer: an error occurred",
					FirstTimestamp: metav1.NewTime(time.Date(2020, time.January, 15, 0, 0, 0, 0, time.UTC)),
					LastTimestamp:  metav1.NewTime(time.Date(2020, time.January, 15, 0, 0, 0, 0, time.UTC)),
					Count:          1,
					Type:           corev1.EventTypeWarning,
				}
			)

			gomock.InOrder(
				k8sShootClient.EXPECT().Client().Return(k8sShootRuntimeClient),
				k8sShootRuntimeClient.EXPECT().Get(gomock.Any(), key, gomock.AssignableToTypeOf(&corev1.Service{})).DoAndReturn(
					func(_ context.Context, _ client.ObjectKey, obj *corev1.Service) error {
						*obj = *svc
						return nil
					}),
				k8sShootClient.EXPECT().DirectClient().Return(k8sShootRuntimeClient),
				k8sShootRuntimeClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.EventList{}), gomock.Any()).DoAndReturn(
					func(_ context.Context, list *corev1.EventList, _ ...client.ListOption) error {
						list.Items = append(list.Items, event)
						return nil
					}),
			)

			actual, err := b.WaitUntilShootLoadBalancerIsReady(ctx, metav1.NamespaceSystem, "load-balancer", 1*time.Second)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("-> Events:\n* service-controller reported"))
			Expect(err.Error()).To(ContainSubstring("Error syncing load balancer: an error occurred"))
			Expect(actual).To(BeEmpty())
		})
	})
})

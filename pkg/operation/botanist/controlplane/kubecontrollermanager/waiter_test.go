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

package kubecontrollermanager

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("WaiterTest", func() {
	var (
		ctx                   = context.TODO()
		testLogger            = logrus.NewEntry(logger.NewNopLogger())
		errorMsg              = "fake error"
		fakeErr               = fmt.Errorf(errorMsg)
		kubeControllerManager KubeControllerManager
		namespace             = "shoot--foo--bar"
		version               = semver.MustParse("v1.16.8")

		// mock
		ctrl        *gomock.Controller
		seedClient  *mockclient.MockClient
		shootClient *mockclient.MockClient
		waiter      *retryfake.Ops
		cleanupFunc func()

		listOptions = []client.ListOption{
			client.InNamespace(namespace),
			client.MatchingLabels(map[string]string{
				"app":  "kubernetes",
				"role": "controller-manager",
			}),
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		seedClient = mockclient.NewMockClient(ctrl)
		shootClient = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
		cleanupFunc()
	})

	Describe("#WaitForControllerToBeActive", func() {
		BeforeEach(func() {
			kubeControllerManager = New(
				testLogger,
				seedClient,
				namespace,
				version,
				"",
				nil,
				nil,
				nil,
			)

			kubeControllerManager.SetShootClient(shootClient)

			waiter = &retryfake.Ops{MaxAttempts: 1}
			cleanupFunc = test.WithVars(
				&retry.Until, waiter.Until,
				&retry.UntilTimeout, waiter.UntilTimeout,
			)
		})

		It("should fail if the seed client cannot talk to the Seed API Server", func() {
			gomock.InOrder(
				seedClient.EXPECT().Get(ctx, kutil.Key(namespace, "kube-controller-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).Return(fakeErr),
			)

			Expect(kubeControllerManager.WaitForControllerToBeActive(ctx)).To(MatchError(fakeErr))
		})

		It("should fail if the kube controller manager deployment does not exist", func() {
			notFoundError := apierrors.NewNotFound(schema.GroupResource{}, "kube-controller-manager")
			gomock.InOrder(
				seedClient.EXPECT().Get(ctx, kutil.Key(namespace, "kube-controller-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).Return(notFoundError),
			)

			Expect(kubeControllerManager.WaitForControllerToBeActive(ctx)).To(MatchError("kube controller manager deployment not found:  \"kube-controller-manager\" not found"))
		})

		It("should fail if it fails to list pods in the shoot namespace in the Seed", func() {
			gomock.InOrder(
				seedClient.EXPECT().Get(ctx, kutil.Key(namespace, "kube-controller-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
				seedClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.PodList{}), listOptions).Return(fakeErr),
			)

			Expect(kubeControllerManager.WaitForControllerToBeActive(ctx)).To(MatchError(fmt.Sprintf("could not check whether controller kube-controller-manager is active: %s", errorMsg)))
		})

		It("should fail if there is more than one kube controller manager pod", func() {
			gomock.InOrder(
				seedClient.EXPECT().Get(ctx, kutil.Key(namespace, "kube-controller-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
				seedClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.PodList{}), listOptions).DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
					*list = corev1.PodList{Items: []corev1.Pod{
						{ObjectMeta: metav1.ObjectMeta{Name: "pod1"}},
						{ObjectMeta: metav1.ObjectMeta{Name: "pod2"}},
					}}
					return nil
				}),
			)

			Expect(kubeControllerManager.WaitForControllerToBeActive(ctx)).To(MatchError(Equal("retry failed with max attempts reached, last error: controller kube-controller-manager is not active")))
		})

		It("should fail if no kube controller manager pod can be found", func() {
			gomock.InOrder(
				seedClient.EXPECT().Get(ctx, kutil.Key(namespace, "kube-controller-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
				seedClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.PodList{}), listOptions).DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
					*list = corev1.PodList{Items: []corev1.Pod{}}
					return nil
				}),
			)

			Expect(kubeControllerManager.WaitForControllerToBeActive(ctx)).To(MatchError(Equal("retry failed with max attempts reached, last error: controller kube-controller-manager is not active")))
		})

		It("should fail if the existing kube controller manager pod has a deletion timestamp", func() {
			gomock.InOrder(
				seedClient.EXPECT().Get(ctx, kutil.Key(namespace, "kube-controller-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
				seedClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.PodList{}), listOptions).DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
					now := metav1.Now()
					*list = corev1.PodList{Items: []corev1.Pod{
						{ObjectMeta: metav1.ObjectMeta{Name: "pod1", DeletionTimestamp: &now}},
					}}
					return nil
				}),
			)

			Expect(kubeControllerManager.WaitForControllerToBeActive(ctx)).To(MatchError(Equal("retry failed with max attempts reached, last error: controller kube-controller-manager is not active")))
		})

		It("should fail if the existing kube controller manager misses leader election annotation control-plane.alpha.kubernetes.io/leader on the endpoints resource", func() {
			gomock.InOrder(
				seedClient.EXPECT().Get(ctx, kutil.Key(namespace, "kube-controller-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
				seedClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.PodList{}), listOptions).DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
					*list = corev1.PodList{Items: []corev1.Pod{
						{ObjectMeta: metav1.ObjectMeta{Name: "pod1"}},
					}}
					return nil
				}),
				shootClient.EXPECT().Get(ctx, kutil.Key(metav1.NamespaceSystem, "kube-controller-manager"), gomock.AssignableToTypeOf(&corev1.Endpoints{})),
			)

			Expect(kubeControllerManager.WaitForControllerToBeActive(ctx)).To(MatchError(Equal("could not check whether controller kube-controller-manager is active: could not find key \"control-plane.alpha.kubernetes.io/leader\" in annotations")))
		})

		It("should fail if the existing kube controller manager fails to acquire leader election", func() {
			gomock.InOrder(
				seedClient.EXPECT().Get(ctx, kutil.Key(namespace, "kube-controller-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
				seedClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.PodList{}), listOptions).DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
					*list = corev1.PodList{Items: []corev1.Pod{
						{ObjectMeta: metav1.ObjectMeta{Name: "pod1"}},
					}}
					return nil
				}),
				shootClient.EXPECT().Get(ctx, kutil.Key(metav1.NamespaceSystem, "kube-controller-manager"), gomock.AssignableToTypeOf(&corev1.Endpoints{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, actual *corev1.Endpoints) error {
					election := resourcelock.LeaderElectionRecord{
						RenewTime: metav1.Time{Time: time.Now().UTC().Add(-10 * time.Second)},
					}
					byteAnnotation, err := json.Marshal(election)
					Expect(err).ToNot(HaveOccurred())

					*actual = corev1.Endpoints{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								resourcelock.LeaderElectionRecordAnnotationKey: string(byteAnnotation),
							},
						},
					}
					return nil
				}),
			)

			Expect(kubeControllerManager.WaitForControllerToBeActive(ctx)).To(MatchError(Equal("retry failed with max attempts reached, last error: controller kube-controller-manager is not active")))
		})

		It("should succeed (k8s < 1.20)", func() {
			gomock.InOrder(
				seedClient.EXPECT().Get(ctx, kutil.Key(namespace, "kube-controller-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
				seedClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.PodList{}), listOptions).DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
					*list = corev1.PodList{Items: []corev1.Pod{
						{ObjectMeta: metav1.ObjectMeta{Name: "pod1"}},
					}}
					return nil
				}),
				shootClient.EXPECT().Get(ctx, kutil.Key(metav1.NamespaceSystem, "kube-controller-manager"), gomock.AssignableToTypeOf(&corev1.Endpoints{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, actual *corev1.Endpoints) error {
					election := resourcelock.LeaderElectionRecord{
						RenewTime: metav1.Time{Time: time.Now().UTC()},
					}
					byteAnnotation, err := json.Marshal(election)
					Expect(err).ToNot(HaveOccurred())

					*actual = corev1.Endpoints{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								resourcelock.LeaderElectionRecordAnnotationKey: string(byteAnnotation),
							},
						},
					}
					return nil
				}),
			)

			Expect(kubeControllerManager.WaitForControllerToBeActive(ctx)).To(Succeed())
		})

		It("should succeed (k8s >= 1.20)", func() {
			kubeControllerManager = New(
				testLogger,
				seedClient,
				namespace,
				semver.MustParse("v1.20.1"),
				"",
				nil,
				nil,
				nil,
			)
			kubeControllerManager.SetShootClient(shootClient)

			gomock.InOrder(
				seedClient.EXPECT().Get(ctx, kutil.Key(namespace, "kube-controller-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
				seedClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.PodList{}), listOptions).DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
					*list = corev1.PodList{Items: []corev1.Pod{
						{ObjectMeta: metav1.ObjectMeta{Name: "pod1"}},
					}}
					return nil
				}),
				shootClient.EXPECT().Get(ctx, kutil.Key(metav1.NamespaceSystem, "kube-controller-manager"), gomock.AssignableToTypeOf(&coordinationv1.Lease{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, actual *coordinationv1.Lease) error {
					*actual = coordinationv1.Lease{
						Spec: coordinationv1.LeaseSpec{
							RenewTime: &metav1.MicroTime{Time: time.Now().UTC()},
						},
					}
					return nil
				}),
			)

			Expect(kubeControllerManager.WaitForControllerToBeActive(ctx)).To(Succeed())
		})
	})
})

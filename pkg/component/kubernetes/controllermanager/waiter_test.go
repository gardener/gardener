// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllermanager_test

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/component/kubernetes/controllermanager"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("WaiterTest", func() {
	var (
		ctx                   = context.TODO()
		testLogger            = logr.Discard()
		errorMsg              = "fake error"
		fakeErr               = fmt.Errorf("fake error")
		kubeControllerManager Interface
		namespace             = "shoot--foo--bar"
		version               = semver.MustParse("v1.31.1")
		isWorkerless          = false

		// mock
		ctrl              *gomock.Controller
		fakeSeedInterface kubernetes.Interface
		seedClient        *mockclient.MockClient
		shootClient       *mockclient.MockClient
		waiter            *retryfake.Ops
		cleanupFunc       func()

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
		fakeSeedInterface = fakekubernetes.NewClientSetBuilder().WithAPIReader(seedClient).WithClient(seedClient).Build()
		shootClient = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
		cleanupFunc()
	})

	Describe("#WaitForControllerToBeActive", func() {
		BeforeEach(func() {
			kubeControllerManager = New(testLogger, fakeSeedInterface, namespace, nil, Values{
				RuntimeVersion: semver.MustParse("1.31.1"),
				TargetVersion:  version,
				IsWorkerless:   isWorkerless,
			})
			kubeControllerManager.SetShootClient(shootClient)

			waiter = &retryfake.Ops{MaxAttempts: 1}
			cleanupFunc = test.WithVars(
				&retry.Until, waiter.Until,
				&retry.UntilTimeout, waiter.UntilTimeout,
			)
		})

		It("should fail if the seed client cannot talk to the Seed API Server", func() {
			gomock.InOrder(
				seedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "kube-controller-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})).Return(fakeErr),
			)

			Expect(kubeControllerManager.WaitForControllerToBeActive(ctx)).To(MatchError(fakeErr))
		})

		It("should fail if the kube controller manager deployment does not exist", func() {
			notFoundError := apierrors.NewNotFound(schema.GroupResource{}, "kube-controller-manager")
			gomock.InOrder(
				seedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "kube-controller-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})).Return(notFoundError),
			)

			Expect(kubeControllerManager.WaitForControllerToBeActive(ctx)).To(MatchError("kube controller manager deployment not found:  \"kube-controller-manager\" not found"))
		})

		It("should fail if it fails to list pods in the shoot namespace in the Seed", func() {
			gomock.InOrder(
				seedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "kube-controller-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})),
				seedClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.PodList{}), listOptions).Return(fakeErr),
			)

			Expect(kubeControllerManager.WaitForControllerToBeActive(ctx)).To(MatchError(fmt.Sprintf("could not check whether controller kube-controller-manager is active: %s", errorMsg)))
		})

		It("should fail if no kube controller manager pod can be found", func() {
			gomock.InOrder(
				seedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "kube-controller-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})),
				seedClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.PodList{}), listOptions).DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
					*list = corev1.PodList{Items: []corev1.Pod{}}
					return nil
				}),
			)

			Expect(kubeControllerManager.WaitForControllerToBeActive(ctx)).To(MatchError(Equal("retry failed with max attempts reached, last error: controller kube-controller-manager is not active")))
		})

		It("should fail if one of the existing kube controller manager pods has a deletion timestamp", func() {
			gomock.InOrder(
				seedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "kube-controller-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})),
				seedClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.PodList{}), listOptions).DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
					now := metav1.Now()
					*list = corev1.PodList{Items: []corev1.Pod{
						{ObjectMeta: metav1.ObjectMeta{Name: "pod1"}},
						{ObjectMeta: metav1.ObjectMeta{Name: "pod2", DeletionTimestamp: &now}},
					}}
					return nil
				}),
			)

			Expect(kubeControllerManager.WaitForControllerToBeActive(ctx)).To(MatchError(Equal("retry failed with max attempts reached, last error: controller kube-controller-manager is not active")))
		})

		It("should fail if the existing kube controller manager fails to acquire leader election", func() {
			gomock.InOrder(
				seedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "kube-controller-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})),
				seedClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.PodList{}), listOptions).DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
					*list = corev1.PodList{Items: []corev1.Pod{
						{ObjectMeta: metav1.ObjectMeta{Name: "pod1"}},
					}}
					return nil
				}),
				shootClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: "kube-controller-manager"}, gomock.AssignableToTypeOf(&coordinationv1.Lease{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, actual *coordinationv1.Lease, _ ...client.GetOption) error {
					*actual = coordinationv1.Lease{
						Spec: coordinationv1.LeaseSpec{
							RenewTime: &metav1.MicroTime{Time: time.Now().UTC().Add(-10 * time.Second)},
						},
					}
					return nil
				}),
			)

			Expect(kubeControllerManager.WaitForControllerToBeActive(ctx)).To(MatchError(Equal("retry failed with max attempts reached, last error: controller kube-controller-manager is not active")))
		})

		It("should succeed", func() {
			gomock.InOrder(
				seedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "kube-controller-manager"}, gomock.AssignableToTypeOf(&appsv1.Deployment{})),
				seedClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.PodList{}), listOptions).DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
					*list = corev1.PodList{Items: []corev1.Pod{
						{ObjectMeta: metav1.ObjectMeta{Name: "pod1"}},
					}}
					return nil
				}),
				shootClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: "kube-controller-manager"}, gomock.AssignableToTypeOf(&coordinationv1.Lease{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, actual *coordinationv1.Lease, _ ...client.GetOption) error {
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

	Describe("#WaitCleanup", func() {
		var (
			fakeClient              client.Client
			fakeKubernetesInterface kubernetes.Interface
			managedResource         *resourcesv1alpha1.ManagedResource
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			fakeKubernetesInterface = fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()
			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot-core-kube-controller-manager",
					Namespace: namespace,
				},
			}

			kubeControllerManager = New(testLogger, fakeKubernetesInterface, namespace, nil, Values{})

			fakeOps := &retryfake.Ops{MaxAttempts: 2}
			DeferCleanup(test.WithVars(
				&retry.Until, fakeOps.Until,
				&retry.UntilTimeout, fakeOps.UntilTimeout,
			))
		})

		It("should fail when the wait for the runtime managed resource deletion times out", func() {
			Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())

			Expect(kubeControllerManager.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
		})

		It("should not return an error when they are already removed", func() {
			Expect(kubeControllerManager.WaitCleanup(ctx)).To(Succeed())
		})
	})
})

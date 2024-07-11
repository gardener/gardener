// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package virtualcluster_test

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/operator/apis/config"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	. "github.com/gardener/gardener/pkg/operator/controller/extension/virtualcluster"
	"github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/gardener/gardener/third_party/mock/client-go/tools/record"
)

const (
	gardenName    = "garden"
	extensionName = "extension"
)

var _ = Describe("Virtual Cluster Reconciler", func() {
	var (
		ctrl            *gomock.Controller
		runtimeClient   client.Client
		virtualClient   client.Client
		ctx             context.Context
		operatorConfig  config.OperatorConfiguration
		gardenClientMap *fakeclientmap.ClientMap
		reconciler      *Reconciler
		garden          *operatorv1alpha1.Garden
		extension       *operatorv1alpha1.Extension
		fakeClock       *testclock.FakeClock
		mockRecorder    *record.MockEventRecorder
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.Background()
		logf.IntoContext(ctx, logr.Discard())

		operatorConfig = config.OperatorConfiguration{
			Controllers: config.ControllerConfiguration{
				ExtensionVirtualCluster: config.ExtensionVirtualClusterControllerConfiguration{
					ConcurrentSyncs: ptr.To(1),
				},
			},
		}
		garden = &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name: gardenName,
			},
		}
		extension = &operatorv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name: extensionName,
			},
			Spec: operatorv1alpha1.ExtensionSpec{
				Resources: []gardencorev1beta1.ControllerResource{
					gardencorev1beta1.ControllerResource{
						Kind: "Worker",
					},
				},
				Deployment: &operatorv1alpha1.Deployment{
					ExtensionDeployment: &operatorv1alpha1.ExtensionDeploymentSpec{
						DeploymentSpec: operatorv1alpha1.DeploymentSpec{
							Helm: &operatorv1alpha1.ExtensionHelm{
								OCIRepository: &gardencorev1.OCIRepository{
									Ref: ptr.To("removeFinalizers"),
								},
							},
						},
					},
				},
			},
		}

		virtualClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.VirtualScheme).Build()
		runtimeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).
			WithStatusSubresource(&operatorv1alpha1.Extension{}, &operatorv1alpha1.Extension{}).Build()
		gardenClientMap = fakeclientmap.NewClientMapBuilder().WithRuntimeClientForKey(keys.ForGarden(garden), virtualClient).Build()

		fakeClock = testclock.NewFakeClock(time.Now())

		mockRecorder = record.NewMockEventRecorder(ctrl)
		mockRecorder.EXPECT().Event(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	})

	Describe("#ExtensionVirtualCluster", func() {
		var req reconcile.Request

		BeforeEach(func() {
			req = reconcile.Request{NamespacedName: client.ObjectKey{Name: extensionName}}
		})

		JustBeforeEach(func() {
			Expect(runtimeClient.Create(ctx, extension)).To(Succeed())
			reconciler = &Reconciler{
				Config:          operatorConfig,
				GardenClientMap: gardenClientMap,
				RuntimeClient:   runtimeClient,
				Clock:           fakeClock,
				Recorder:        mockRecorder,
			}
		})

		Context("when extension no longer exists", func() {
			It("should stop reconciling and not requeue", func() {
				req = reconcile.Request{NamespacedName: client.ObjectKey{Name: "some-random-extension"}}
				Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{}))
			})
		})

		Context("when garden is deleted", func() {
			It("remove finalizers when garden is deleted", func() {
				extension.Finalizers = append(extension.Finalizers, operatorv1alpha1.FinalizerName)
				Expect(runtimeClient.Update(ctx, extension)).To(Succeed())

				req = reconcile.Request{NamespacedName: client.ObjectKey{Name: extensionName}}
				Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{}))

				sutExt := &operatorv1alpha1.Extension{}
				Expect(runtimeClient.Get(ctx, client.ObjectKey{Name: extensionName}, sutExt)).To(Succeed())
				Expect(sutExt.Finalizers).To(BeEmpty())
			})
		})

		Context("when garden is ready", func() {
			BeforeEach(func() {
				garden.Status = operatorv1alpha1.GardenStatus{
					Conditions: []gardencorev1beta1.Condition{
						{
							Type:               operatorv1alpha1.VirtualGardenAPIServerAvailable,
							Status:             gardencorev1beta1.ConditionTrue,
							LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
						},
					},
				}
				Expect(runtimeClient.Create(ctx, garden)).To(Succeed())
			})

			It("should create the ctrl-{registration,deployment}", func() {
				req = reconcile.Request{NamespacedName: client.ObjectKey{Name: extensionName}}
				Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{}))

				sutExt := &operatorv1alpha1.Extension{}
				Expect(runtimeClient.Get(ctx, client.ObjectKey{Name: extensionName}, sutExt)).To(Succeed())
				Expect(sutExt.Status.Conditions).To(HaveLen(1))
				Expect(sutExt.Status.Conditions[0]).To(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":   Equal(operatorv1alpha1.VirtualClusterExtensionReconciled),
					"Status": Equal(gardencorev1beta1.ConditionTrue),
					"Reason": Equal(ConditionReconcileSuccess),
				}))

				var ctrlRegList gardencorev1beta1.ControllerRegistrationList
				var ctrlDepList gardencorev1.ControllerDeploymentList
				Expect(virtualClient.List(ctx, &ctrlRegList)).To(Succeed())
				Expect(virtualClient.List(ctx, &ctrlDepList)).To(Succeed())
				Expect(ctrlDepList.Items).To(HaveLen(1))
				Expect(ctrlRegList.Items).To(HaveLen(1))

				Expect(runtimeClient.Delete(ctx, extension)).To(Succeed())
				req = reconcile.Request{NamespacedName: client.ObjectKey{Name: extensionName}}
				Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{}))

				Expect(runtimeClient.Get(ctx, client.ObjectKey{Name: extensionName}, sutExt)).To(matchers.BeNotFoundError())
				Expect(virtualClient.List(ctx, &ctrlRegList)).To(Succeed())
				Expect(virtualClient.List(ctx, &ctrlDepList)).To(Succeed())
				Expect(ctrlDepList.Items).To(BeEmpty())
				Expect(ctrlRegList.Items).To(BeEmpty())
			})
		})
	})
})

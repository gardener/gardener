// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension_test

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/operator/apis/config"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	. "github.com/gardener/gardener/pkg/operator/controller/extension"
	ocifake "github.com/gardener/gardener/pkg/utils/oci/fake"
	"github.com/gardener/gardener/pkg/utils/test/matchers"
)

const (
	gardenName    = "garden"
	extensionName = "extension"
)

var _ = Describe("Reconciler", func() {
	var (
		runtimeClientSet kubernetes.Interface
		virtualClient    client.Client
		ctx              context.Context
		operatorConfig   config.OperatorConfiguration
		gardenClientMap  *fakeclientmap.ClientMap
		helmRegistry     *ocifake.Registry
		reconciler       *Reconciler
		garden           *operatorv1alpha1.Garden
		extension        *operatorv1alpha1.Extension
		fakeClock        *testclock.FakeClock
		fakeRecorder     *record.FakeRecorder
	)

	BeforeEach(func() {
		ctx = context.Background()
		logf.IntoContext(ctx, logr.Discard())

		operatorConfig = config.OperatorConfiguration{
			Controllers: config.ControllerConfiguration{
				Extension: config.ExtensionControllerConfiguration{
					ConcurrentSyncs: ptr.To(1),
				},
			},
		}
		garden = &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name:        gardenName,
				Annotations: map[string]string{v1beta1constants.AnnotationKeyGenericTokenKubeconfigSecretName: "foo-kubeconfig"},
			},
		}
		extension = &operatorv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name: extensionName,
			},
			Spec: operatorv1alpha1.ExtensionSpec{
				Resources: []gardencorev1beta1.ControllerResource{
					{Kind: "Worker"},
				},
				Deployment: &operatorv1alpha1.Deployment{
					ExtensionDeployment: &operatorv1alpha1.ExtensionDeploymentSpec{
						DeploymentSpec: operatorv1alpha1.DeploymentSpec{
							Helm: &operatorv1alpha1.ExtensionHelm{
								OCIRepository: &gardencorev1.OCIRepository{
									Ref: ptr.To("removeFinalizer"),
								},
							},
						},
					},
				},
			},
		}

		virtualClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.VirtualScheme).Build()
		runtimeClient := fakeclient.NewClientBuilder().
			WithScheme(operatorclient.RuntimeScheme).
			WithStatusSubresource(&operatorv1alpha1.Extension{}, &operatorv1alpha1.Extension{}).Build()
		runtimeClientSet = kubernetesfake.NewClientSetBuilder().WithClient(runtimeClient).Build()
		gardenClientMap = fakeclientmap.NewClientMapBuilder().WithRuntimeClientForKey(keys.ForGarden(garden), virtualClient, nil).Build()
		helmRegistry = ocifake.NewRegistry()

		fakeClock = testclock.NewFakeClock(time.Now())
		fakeRecorder = &record.FakeRecorder{}
	})

	Describe("#Extension", func() {
		var req reconcile.Request

		BeforeEach(func() {
			req = reconcile.Request{NamespacedName: client.ObjectKey{Name: extensionName}}
		})

		JustBeforeEach(func() {
			Expect(runtimeClientSet.Client().Create(ctx, extension)).To(Succeed())
			reconciler = &Reconciler{
				Config:           operatorConfig,
				GardenClientMap:  gardenClientMap,
				HelmRegistry:     helmRegistry,
				RuntimeClientSet: runtimeClientSet,
				Clock:            fakeClock,
				Recorder:         fakeRecorder,
			}
		})

		Context("when extension no longer exists", func() {
			It("should stop reconciling and not requeue", func() {
				req = reconcile.Request{NamespacedName: client.ObjectKey{Name: "some-random-extension"}}
				Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{}))
			})
		})

		Context("reconcile based on garden state", func() {
			It("remove finalizers when garden is deleted", func() {
				extension.Finalizers = append(extension.Finalizers, operatorv1alpha1.FinalizerName)
				Expect(runtimeClientSet.Client().Update(ctx, extension)).To(Succeed())

				req = reconcile.Request{NamespacedName: client.ObjectKey{Name: extensionName}}
				Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{}))

				extension := &operatorv1alpha1.Extension{}
				Expect(runtimeClientSet.Client().Get(ctx, client.ObjectKey{Name: extensionName}, extension)).To(Succeed())
				Expect(extension.Finalizers).To(BeEmpty())
			})

			It("should requeue if gardener is not ready", func() {
				garden.Status = operatorv1alpha1.GardenStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateProcessing,
						Type:  gardencorev1beta1.LastOperationTypeReconcile,
					},
				}
				Expect(runtimeClientSet.Client().Create(ctx, garden)).To(Succeed())

				res, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKey{Name: extensionName}})
				Expect(res).To(Equal(reconcile.Result{RequeueAfter: 10 * time.Second}))
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create the controller-{registration,deployment} if garden is ready", func() {
				garden.Status = operatorv1alpha1.GardenStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateSucceeded,
						Type:  gardencorev1beta1.LastOperationTypeReconcile,
					},
				}
				Expect(runtimeClientSet.Client().Create(ctx, garden)).To(Succeed())

				By("reconcile extension")
				req = reconcile.Request{NamespacedName: client.ObjectKey{Name: extensionName}}
				Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{}))

				extension := &operatorv1alpha1.Extension{}
				Expect(runtimeClientSet.Client().Get(ctx, client.ObjectKey{Name: extensionName}, extension)).To(Succeed())
				Expect(extension.Status.Conditions).To(HaveLen(2))
				Expect(extension.Status.Conditions[0]).To(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":   Equal(operatorv1alpha1.RuntimeClusterExtensionReconciled),
					"Status": Equal(gardencorev1beta1.ConditionTrue),
					"Reason": Equal(ConditionReconcileSuccess),
				}))
				Expect(extension.Status.Conditions[1]).To(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":   Equal(operatorv1alpha1.VirtualClusterExtensionReconciled),
					"Status": Equal(gardencorev1beta1.ConditionTrue),
					"Reason": Equal(ConditionReconcileSuccess),
				}))

				var (
					controllerDeploymentList   gardencorev1.ControllerDeploymentList
					controllerRegistrationList gardencorev1beta1.ControllerRegistrationList
				)
				Expect(virtualClient.List(ctx, &controllerRegistrationList)).To(Succeed())
				Expect(virtualClient.List(ctx, &controllerDeploymentList)).To(Succeed())
				Expect(controllerDeploymentList.Items).To(HaveLen(1))
				Expect(controllerRegistrationList.Items).To(HaveLen(1))

				By("reconcile extension after disabling admission by annotation")
				toggleAdmission(ctx, runtimeClientSet.Client(), false)
				Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{}))
				Expect(runtimeClientSet.Client().Get(ctx, client.ObjectKey{Name: extensionName}, extension)).To(Succeed())
				Expect(extension.Status.Conditions).To(HaveLen(2))
				Expect(extension.Status.Conditions[0]).To(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":   Equal(operatorv1alpha1.RuntimeClusterExtensionReconciled),
					"Status": Equal(gardencorev1beta1.ConditionFalse),
					"Reason": Equal(ConditionDeleteSuccessful),
				}))
				Expect(extension.Status.Conditions[1]).To(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":   Equal(operatorv1alpha1.VirtualClusterExtensionReconciled),
					"Status": Equal(gardencorev1beta1.ConditionFalse),
					"Reason": Equal(ConditionDeleteSuccessful),
				}))

				By("reoncile extension after re-enabling admission by annotation")
				toggleAdmission(ctx, runtimeClientSet.Client(), true)
				Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{}))
				Expect(runtimeClientSet.Client().Get(ctx, client.ObjectKey{Name: extensionName}, extension)).To(Succeed())
				Expect(extension.Status.Conditions).To(HaveLen(2))
				Expect(extension.Status.Conditions[0]).To(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":   Equal(operatorv1alpha1.RuntimeClusterExtensionReconciled),
					"Status": Equal(gardencorev1beta1.ConditionTrue),
					"Reason": Equal(ConditionReconcileSuccess),
				}))
				Expect(extension.Status.Conditions[1]).To(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":   Equal(operatorv1alpha1.VirtualClusterExtensionReconciled),
					"Status": Equal(gardencorev1beta1.ConditionTrue),
					"Reason": Equal(ConditionReconcileSuccess),
				}))

				By("delete extension")
				Expect(runtimeClientSet.Client().Delete(ctx, extension)).To(Succeed())
				req = reconcile.Request{NamespacedName: client.ObjectKey{Name: extensionName}}
				Expect(reconciler.Reconcile(ctx, req)).To(Equal(reconcile.Result{}))

				Expect(runtimeClientSet.Client().Get(ctx, client.ObjectKey{Name: extensionName}, extension)).To(matchers.BeNotFoundError())
				Expect(virtualClient.List(ctx, &controllerRegistrationList)).To(Succeed())
				Expect(virtualClient.List(ctx, &controllerDeploymentList)).To(Succeed())
				Expect(controllerDeploymentList.Items).To(BeEmpty())
				Expect(controllerRegistrationList.Items).To(BeEmpty())
			})
		})
	})
})

func toggleAdmission(ctx context.Context, runtimeClient client.Client, enable bool) {
	obj := &operatorv1alpha1.Extension{}
	Expect(runtimeClient.Get(ctx, client.ObjectKey{Name: extensionName}, obj)).To(Succeed())

	patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
	annotations := obj.GetAnnotations()
	if enable {
		delete(annotations, AnnotationKeyDisableAdmission)
	} else {
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[AnnotationKeyDisableAdmission] = "true"
	}
	obj.SetAnnotations(annotations)
	Expect(runtimeClient.Patch(ctx, obj, patch)).To(Succeed())
}

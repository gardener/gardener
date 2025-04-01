// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane_test

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/controlplane"
	"github.com/gardener/gardener/pkg/extensions"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mocktime "github.com/gardener/gardener/third_party/mock/go/time"
)

var _ = Describe("ControlPlane", func() {
	var (
		ctrl *gomock.Controller
		c    client.Client

		mockNow *mocktime.MockNow
		now     time.Time

		ctx = context.TODO()
		log = logr.Discard()

		name                         = "test"
		namespace                    = "testnamespace"
		extensionType                = "some-type"
		purpose                      = extensionsv1alpha1.Purpose("foo")
		region                       = "local"
		providerConfig               = &runtime.RawExtension{Raw: []byte(`{"bar":"baz"}`)}
		infrastructureProviderStatus = &runtime.RawExtension{Raw: []byte(`{"baz":"foo"}`)}

		cp, empty *extensionsv1alpha1.ControlPlane
		cpSpec    extensionsv1alpha1.ControlPlaneSpec

		defaultDepWaiter controlplane.Interface
		values           *controlplane.Values
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockNow = mocktime.NewMockNow(ctrl)
		now = time.Now()

		s := runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(s)).NotTo(HaveOccurred())
		c = fake.NewClientBuilder().WithScheme(s).Build()

		empty = &extensionsv1alpha1.ControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}
		cp = empty.DeepCopy()
		cp.SetAnnotations(map[string]string{
			v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
			v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
		})

		cpSpec = extensionsv1alpha1.ControlPlaneSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type:           extensionType,
				ProviderConfig: providerConfig,
			},
			Region:  region,
			Purpose: &purpose,
			SecretRef: corev1.SecretReference{
				Name:      "cloudprovider",
				Namespace: namespace,
			},
			InfrastructureProviderStatus: infrastructureProviderStatus,
		}

		values = &controlplane.Values{
			Name:                         name,
			Namespace:                    namespace,
			Type:                         extensionType,
			ProviderConfig:               providerConfig,
			Purpose:                      purpose,
			Region:                       region,
			InfrastructureProviderStatus: infrastructureProviderStatus,
		}
		defaultDepWaiter = controlplane.New(log, c, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		It("should successfully deploy the ControlPlane resource (purpose != exposure)", func() {
			defer test.WithVars(&controlplane.TimeNow, mockNow.Do)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			obj := &extensionsv1alpha1.ControlPlane{}
			err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, obj)
			Expect(err).NotTo(HaveOccurred())

			Expect(obj).To(DeepEqual(&extensionsv1alpha1.ControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Annotations: map[string]string{
						"gardener.cloud/operation": "reconcile",
						"gardener.cloud/timestamp": now.UTC().Format(time.RFC3339Nano),
					},
					ResourceVersion: "1",
				},
				Spec: cpSpec,
			}))
		})

		It("should successfully deploy the ControlPlane resource (purpose == exposure)", func() {
			defer test.WithVars(&controlplane.TimeNow, mockNow.Do)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			values.Purpose = extensionsv1alpha1.Exposure
			cpSpec.Purpose = &values.Purpose
			defaultDepWaiter = controlplane.New(log, c, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)

			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			obj := &extensionsv1alpha1.ControlPlane{}
			err := c.Get(ctx, client.ObjectKey{Name: name + "-exposure", Namespace: namespace}, obj)
			Expect(err).NotTo(HaveOccurred())

			Expect(obj).To(DeepEqual(&extensionsv1alpha1.ControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name + "-exposure",
					Namespace: namespace,
					Annotations: map[string]string{
						"gardener.cloud/operation": "reconcile",
						"gardener.cloud/timestamp": now.UTC().Format(time.RFC3339Nano),
					},
					ResourceVersion: "1",
				},
				Spec: cpSpec,
			}))
		})
	})

	Describe("#Wait", func() {
		It("should return error when no resources are found", func() {
			Expect(defaultDepWaiter.Wait(ctx)).To(HaveOccurred())
		})

		It("should return error when resource is not ready", func() {
			obj := cp.DeepCopy()
			obj.Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}
			Expect(c.Create(ctx, obj)).To(Succeed(), "creating controlplane succeeds")

			Expect(defaultDepWaiter.Wait(ctx)).To(HaveOccurred(), "controlplane indicates error")
		})

		It("should return error if we haven't observed the latest timestamp annotation (purpose != exposure)", func() {
			defer test.WithVars(&controlplane.TimeNow, mockNow.Do)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			By("Deploy")
			// Deploy should fill internal state with the added timestamp annotation
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			By("Patch object")
			patch := client.MergeFrom(cp.DeepCopy())
			// remove operation annotation, add old timestamp annotation
			cp.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.Add(-time.Millisecond).UTC().Format(time.RFC3339Nano),
			}
			cp.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}
			Expect(c.Patch(ctx, cp, patch)).To(Succeed(), "patching controlplane succeeds")

			By("Wait")
			Expect(defaultDepWaiter.Wait(ctx)).NotTo(Succeed(), "controlplane indicates error")
		})

		It("should return no error when it's ready (purpose != exposure)", func() {
			defer test.WithVars(&controlplane.TimeNow, mockNow.Do)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			By("Deploy")
			// Deploy should fill internal state with the added timestamp annotation
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			By("Patch object")
			patch := client.MergeFrom(cp.DeepCopy())
			// remove operation annotation, add up-to-date timestamp annotation
			cp.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
			}
			cp.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: metav1.Time{Time: now.UTC().Add(time.Second)},
			}
			Expect(c.Patch(ctx, cp, patch)).To(Succeed(), "patching controlplane succeeds")

			By("Wait")
			Expect(defaultDepWaiter.Wait(ctx)).To(Succeed(), "controlplane is ready")
		})

		It("should return error if we haven't observed the latest timestamp annotation (purpose == exposure)", func() {
			defer test.WithVars(&controlplane.TimeNow, mockNow.Do)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			values.Purpose = extensionsv1alpha1.Exposure
			defaultDepWaiter = controlplane.New(log, c, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
			cp.Name += "-exposure"

			By("Deploy")
			// Deploy should fill internal state with the added timestamp annotation
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			By("Patch object")
			patch := client.MergeFrom(cp.DeepCopy())
			// remove operation annotation, add old timestamp annotation
			cp.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.Add(-time.Millisecond).UTC().Format(time.RFC3339Nano),
			}
			cp.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}
			Expect(c.Patch(ctx, cp, patch)).To(Succeed(), "patching controlplane succeeds")

			By("Wait")
			Expect(defaultDepWaiter.Wait(ctx)).NotTo(Succeed(), "controlplane indicates error")
		})

		It("should return no error when it's ready (purpose == exposure)", func() {
			defer test.WithVars(&controlplane.TimeNow, mockNow.Do)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			values.Purpose = extensionsv1alpha1.Exposure
			defaultDepWaiter = controlplane.New(log, c, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
			cp.Name += "-exposure"

			By("Deploy")
			// Deploy should fill internal state with the added timestamp annotation
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			By("Patch object")
			patch := client.MergeFrom(cp.DeepCopy())
			// remove operation annotation, add up-to-date timestamp annotation
			cp.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
			}
			cp.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: metav1.Time{Time: now.UTC().Add(time.Second)},
			}
			Expect(c.Patch(ctx, cp, patch)).To(Succeed(), "patching controlplane succeeds")

			By("Wait")
			Expect(defaultDepWaiter.Wait(ctx)).To(Succeed(), "controlplane is ready")
		})
	})

	Describe("#Destroy", func() {
		It("should not return error when not found", func() {
			Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())
		})

		It("should not return error when deleted successfully", func() {
			Expect(c.Create(ctx, cp.DeepCopy())).To(Succeed(), "adding pre-existing controlplane succeeds")
			Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())
		})

		It("should return error if not deleted successfully (purpose != exposure)", func() {
			defer test.WithVars(
				&extensions.TimeNow, mockNow.Do,
				&gardenerutils.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			fakeErr := fmt.Errorf("some random error")
			obj := cp.DeepCopy()
			obj.Annotations = map[string]string{
				"confirmation.gardener.cloud/deletion": "true",
				"gardener.cloud/timestamp":             now.UTC().Format(time.RFC3339Nano),
			}

			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.ControlPlane{}), gomock.Any())
			mc.EXPECT().Delete(ctx, obj).Return(fakeErr)

			err := controlplane.New(log, mc, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond).Destroy(ctx)
			Expect(err).To(MatchError(fakeErr))
		})

		It("should return error if not deleted successfully (purpose == exposure)", func() {
			defer test.WithVars(
				&extensions.TimeNow, mockNow.Do,
				&gardenerutils.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			values.Purpose = extensionsv1alpha1.Exposure
			fakeErr := fmt.Errorf("some random error")
			obj := cp.DeepCopy()
			obj.Name += "-exposure"
			obj.Annotations = map[string]string{
				"confirmation.gardener.cloud/deletion": "true",
				"gardener.cloud/timestamp":             now.UTC().Format(time.RFC3339Nano),
			}

			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.ControlPlane{}), gomock.Any())
			mc.EXPECT().Delete(ctx, obj).Return(fakeErr)

			err := controlplane.New(log, mc, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond).Destroy(ctx)
			Expect(err).To(MatchError(fakeErr))
		})
	})

	Describe("#WaitCleanup", func() {
		It("should not return error when resources are removed", func() {
			Expect(defaultDepWaiter.WaitCleanup(ctx)).To(Succeed())
		})

		It("should return error if resources with deletionTimestamp still exist (purpose != exposure)", func() {
			timeNow := metav1.Now()
			obj := cp.DeepCopy()
			obj.DeletionTimestamp = &timeNow
			Expect(c.Create(ctx, obj)).To(Succeed())

			Expect(defaultDepWaiter.WaitCleanup(ctx)).To(HaveOccurred())
		})

		It("should return error if resources with deletionTimestamp still exist (purpose == exposure)", func() {
			timeNow := metav1.Now()
			obj := cp.DeepCopy()
			obj.Name += "-exposure"
			obj.DeletionTimestamp = &timeNow
			Expect(c.Create(ctx, obj)).To(Succeed())

			values.Purpose = extensionsv1alpha1.Exposure
			defaultDepWaiter = controlplane.New(log, c, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
			Expect(defaultDepWaiter.WaitCleanup(ctx)).To(HaveOccurred())
		})
	})

	Describe("#Restore", func() {
		var (
			state      = &runtime.RawExtension{Raw: []byte(`{"dummy":"state"}`)}
			shootState *gardencorev1beta1.ShootState
		)

		BeforeEach(func() {
			shootState = &gardencorev1beta1.ShootState{
				Spec: gardencorev1beta1.ShootStateSpec{
					Extensions: []gardencorev1beta1.ExtensionResourceState{
						{
							Name:    &name,
							Kind:    extensionsv1alpha1.ControlPlaneResource,
							Purpose: ptr.To(string(purpose)),
							State:   state,
						},
					},
				},
			}
		})

		It("should properly restore the controlplane state if it exists (purpose != exposure)", func() {
			defer test.WithVars(
				&controlplane.TimeNow, mockNow.Do,
				&extensions.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			mc := mockclient.NewMockClient(ctrl)
			mockStatusWriter := mockclient.NewMockStatusWriter(ctrl)

			mc.EXPECT().Status().Return(mockStatusWriter)

			mc.EXPECT().Get(ctx, client.ObjectKeyFromObject(empty), gomock.AssignableToTypeOf(empty)).
				Return(apierrors.NewNotFound(extensionsv1alpha1.Resource("controlplanes"), name))

			// deploy with wait-for-state annotation
			obj := cp.DeepCopy()
			obj.Spec = cpSpec
			metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "gardener.cloud/operation", "wait-for-state")
			metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "gardener.cloud/timestamp", now.UTC().Format(time.RFC3339Nano))
			mc.EXPECT().Create(ctx, test.HasObjectKeyOf(obj)).
				DoAndReturn(func(_ context.Context, actual client.Object, _ ...client.CreateOption) error {
					Expect(actual).To(DeepEqual(obj))
					return nil
				})

			// restore state
			expectedWithState := obj.DeepCopy()
			expectedWithState.Status.State = state
			test.EXPECTStatusPatch(ctx, mockStatusWriter, expectedWithState, obj, types.MergePatchType)

			// annotate with restore annotation
			expectedWithRestore := expectedWithState.DeepCopy()
			metav1.SetMetaDataAnnotation(&expectedWithRestore.ObjectMeta, "gardener.cloud/operation", "restore")
			test.EXPECTPatch(ctx, mc, expectedWithRestore, expectedWithState, types.MergePatchType)

			Expect(controlplane.New(log, mc, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond).Restore(ctx, shootState)).To(Succeed())
		})

		It("should properly restore the controlplane state if it exists (purpose == exposure)", func() {
			defer test.WithVars(
				&controlplane.TimeNow, mockNow.Do,
				&extensions.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			mc := mockclient.NewMockClient(ctrl)
			mockStatusWriter := mockclient.NewMockStatusWriter(ctrl)

			mc.EXPECT().Status().Return(mockStatusWriter)

			empty.Name += "-exposure"
			mc.EXPECT().Get(ctx, client.ObjectKeyFromObject(empty), gomock.AssignableToTypeOf(empty)).
				Return(apierrors.NewNotFound(extensionsv1alpha1.Resource("controlplanes"), name))

			// deploy with wait-for-state annotation
			values.Purpose = extensionsv1alpha1.Exposure
			cp.Name += "-exposure"
			obj := cp.DeepCopy()
			obj.Spec = cpSpec
			obj.Spec.Purpose = &values.Purpose
			metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "gardener.cloud/operation", "wait-for-state")
			metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "gardener.cloud/timestamp", now.UTC().Format(time.RFC3339Nano))
			mc.EXPECT().Create(ctx, test.HasObjectKeyOf(obj)).
				DoAndReturn(func(_ context.Context, actual client.Object, _ ...client.CreateOption) error {
					Expect(actual).To(DeepEqual(obj))
					return nil
				})

			// restore state
			shootState.Spec.Extensions[0].Name = &obj.Name
			shootState.Spec.Extensions[0].Purpose = ptr.To(string(values.Purpose))
			expectedWithState := obj.DeepCopy()
			expectedWithState.Status.State = state
			test.EXPECTStatusPatch(ctx, mockStatusWriter, expectedWithState, obj, types.MergePatchType)

			// annotate with restore annotation
			expectedWithRestore := expectedWithState.DeepCopy()
			expectedWithRestore.Annotations["gardener.cloud/operation"] = "restore"
			test.EXPECTPatch(ctx, mc, expectedWithRestore, expectedWithState, types.MergePatchType)

			Expect(controlplane.New(log, mc, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond).Restore(ctx, shootState)).To(Succeed())
		})
	})

	Describe("#Migrate", func() {
		It("should migrate the resources (purpose != exposure)", func() {
			Expect(c.Create(ctx, cp.DeepCopy())).To(Succeed(), "creating controlplane succeeds")

			Expect(defaultDepWaiter.Migrate(ctx)).To(Succeed())

			result := &extensionsv1alpha1.ControlPlane{}
			Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, result)).To(Succeed())
			Expect(result.Annotations).To(HaveKeyWithValue("gardener.cloud/operation", "migrate"))
		})

		It("should migrate the resources (purpose == exposure)", func() {
			values.Purpose = extensionsv1alpha1.Exposure
			defaultDepWaiter = controlplane.New(log, c, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)

			Expect(c.Create(ctx, cp.DeepCopy())).To(Succeed(), "creating controlplane succeeds")

			Expect(defaultDepWaiter.Migrate(ctx)).To(Succeed())

			result := &extensionsv1alpha1.ControlPlane{}
			Expect(c.Get(ctx, client.ObjectKey{Name: values.Name, Namespace: values.Namespace}, result)).To(Succeed())
			Expect(result.Annotations).To(HaveKeyWithValue("gardener.cloud/operation", "migrate"))
		})

		It("should not return error if resource does not exist", func() {
			Expect(defaultDepWaiter.Migrate(ctx)).To(Succeed())
		})
	})

	Describe("#WaitMigrate", func() {
		It("should not return error when resource is missing", func() {
			Expect(defaultDepWaiter.WaitMigrate(ctx)).To(Succeed())
		})

		It("should return error if resource is not yet migrated successfully", func() {
			obj := cp.DeepCopy()
			obj.Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}
			obj.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateError,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(c.Create(ctx, obj)).To(Succeed(), "creating controlplane succeeds")
			Expect(defaultDepWaiter.WaitMigrate(ctx)).To(HaveOccurred())
		})

		It("should not return error if resource gets migrated successfully (purpose != exposure)", func() {
			obj := cp.DeepCopy()
			obj.Status.LastError = nil
			obj.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(c.Create(ctx, obj)).To(Succeed(), "creating controlplane succeeds")
			Expect(defaultDepWaiter.WaitMigrate(ctx)).To(Succeed(), "controlplane is ready, should not return an error")
		})

		It("should not return error if resource gets migrated successfully (purpose == exposure)", func() {
			values.Purpose = extensionsv1alpha1.Exposure
			defaultDepWaiter = controlplane.New(log, c, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)

			obj := cp.DeepCopy()
			obj.Name += "-exposure"
			obj.Status.LastError = nil
			obj.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(c.Create(ctx, obj)).To(Succeed(), "creating controlplane succeeds")
			Expect(defaultDepWaiter.WaitMigrate(ctx)).To(Succeed(), "controlplane is ready, should not return an error")
		})
	})
})

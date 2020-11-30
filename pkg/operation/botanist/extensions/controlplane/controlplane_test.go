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

package controlplane_test

import (
	"context"
	"fmt"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mocktime "github.com/gardener/gardener/pkg/mock/go/time"
	"github.com/gardener/gardener/pkg/operation/botanist/extensions/controlplane"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/shoot"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ControlPlane", func() {
	var (
		ctrl *gomock.Controller
		c    client.Client

		mockNow *mocktime.MockNow
		now     time.Time

		ctx = context.TODO()
		log = logger.NewNopLogger()

		name                         = "test"
		namespace                    = "testnamespace"
		extensionType                = "some-type"
		purpose                      = extensionsv1alpha1.Purpose("foo")
		region                       = "local"
		providerConfig               = &runtime.RawExtension{Raw: []byte(`{"bar":"baz"}`)}
		infrastructureProviderStatus = &runtime.RawExtension{Raw: []byte(`{"baz":"foo"}`)}

		cp = &extensionsv1alpha1.ControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}
		cpSpec extensionsv1alpha1.ControlPlaneSpec

		defaultDepWaiter shoot.ExtensionControlPlane
		values           *controlplane.Values
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockNow = mocktime.NewMockNow(ctrl)

		s := runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(s)).NotTo(HaveOccurred())
		c = fake.NewFakeClientWithScheme(s)

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
				TypeMeta: metav1.TypeMeta{
					APIVersion: extensionsv1alpha1.SchemeGroupVersion.String(),
					Kind:       extensionsv1alpha1.ControlPlaneResource,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Annotations: map[string]string{
						"gardener.cloud/operation": "reconcile",
						"gardener.cloud/timestamp": now.UTC().String(),
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
				TypeMeta: metav1.TypeMeta{
					APIVersion: extensionsv1alpha1.SchemeGroupVersion.String(),
					Kind:       extensionsv1alpha1.ControlPlaneResource,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name + "-exposure",
					Namespace: namespace,
					Annotations: map[string]string{
						"gardener.cloud/operation": "reconcile",
						"gardener.cloud/timestamp": now.UTC().String(),
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

		It("should return no error when it's ready (purpose != exposure)", func() {
			obj := cp.DeepCopy()
			obj.Annotations = nil
			obj.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}
			Expect(c.Create(ctx, obj)).To(Succeed(), "creating controlplane succeeds")

			Expect(defaultDepWaiter.Wait(ctx)).To(Succeed(), "controlplane is ready, should not return an error")
		})

		It("should return no error when it's ready (purpose == exposure)", func() {
			values.Purpose = extensionsv1alpha1.Exposure
			defaultDepWaiter = controlplane.New(log, c, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)

			obj := cp.DeepCopy()
			obj.Name += "-exposure"
			obj.Annotations = nil
			obj.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}
			Expect(c.Create(ctx, obj)).To(Succeed(), "creating controlplane succeeds")

			Expect(defaultDepWaiter.Wait(ctx)).To(Succeed(), "controlplane is ready, should not return an error")
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
			defer test.WithVars(&common.TimeNow, mockNow.Do)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			values.Purpose = extensionsv1alpha1.Exposure
			fakeErr := fmt.Errorf("some random error")
			obj := cp.DeepCopy()
			obj.Name += "-exposure"
			obj.Annotations = map[string]string{
				"confirmation.gardener.cloud/deletion": "true",
				"gardener.cloud/timestamp":             now.UTC().String(),
			}

			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Get(ctx, kutil.Key(obj.Namespace, obj.Name), gomock.AssignableToTypeOf(&extensionsv1alpha1.ControlPlane{}))
			mc.EXPECT().Update(ctx, obj)
			mc.EXPECT().Delete(ctx, obj).Return(fakeErr)

			err := controlplane.New(log, mc, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond).Destroy(ctx)
			Expect(err).To(MatchError(fakeErr))
		})

		It("should return error if not deleted successfully (purpose == exposure)", func() {
			defer test.WithVars(&common.TimeNow, mockNow.Do)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			fakeErr := fmt.Errorf("some random error")
			obj := cp.DeepCopy()
			obj.Annotations = map[string]string{
				"confirmation.gardener.cloud/deletion": "true",
				"gardener.cloud/timestamp":             now.UTC().String(),
			}

			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&extensionsv1alpha1.ControlPlane{}))
			mc.EXPECT().Update(ctx, obj)
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
			state      = &runtime.RawExtension{Raw: []byte("dummy state")}
			shootState *gardencorev1alpha1.ShootState
		)

		BeforeEach(func() {
			shootState = &gardencorev1alpha1.ShootState{
				Spec: gardencorev1alpha1.ShootStateSpec{
					Extensions: []gardencorev1alpha1.ExtensionResourceState{
						{
							Name:    &name,
							Kind:    extensionsv1alpha1.ControlPlaneResource,
							Purpose: pointer.StringPtr(string(purpose)),
							State:   state,
						},
					},
				},
			}
		})

		It("should properly restore the controlplane state if it exists (purpose != exposure)", func() {
			defer test.WithVars(
				&controlplane.TimeNow, mockNow.Do,
				&common.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			obj := cp.DeepCopy()
			obj.Spec = cpSpec
			metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "gardener.cloud/operation", "wait-for-state")
			metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "gardener.cloud/timestamp", now.UTC().String())
			expectedWithState := obj.DeepCopy()
			expectedWithState.Status.State = state
			expectedWithRestore := expectedWithState.DeepCopy()
			expectedWithRestore.Annotations["gardener.cloud/operation"] = "restore"

			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&extensionsv1alpha1.ControlPlane{})).Return(apierrors.NewNotFound(extensionsv1alpha1.Resource("ControlPlane"), name))
			mc.EXPECT().Create(ctx, obj)
			mc.EXPECT().Status().Return(mc)
			mc.EXPECT().Update(ctx, expectedWithState)
			mc.EXPECT().Patch(ctx, expectedWithRestore, client.MergeFrom(expectedWithState))

			Expect(controlplane.New(log, mc, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond).Restore(ctx, shootState)).To(Succeed())
		})

		It("should properly restore the controlplane state if it exists (purpose == exposure)", func() {
			defer test.WithVars(
				&controlplane.TimeNow, mockNow.Do,
				&common.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			values.Purpose = extensionsv1alpha1.Exposure
			obj := cp.DeepCopy()
			obj.Name += "-exposure"
			obj.Spec = cpSpec
			obj.Spec.Purpose = &values.Purpose
			shootState.Spec.Extensions[0].Name = &obj.Name
			shootState.Spec.Extensions[0].Purpose = pointer.StringPtr(string(values.Purpose))
			metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "gardener.cloud/operation", "wait-for-state")
			metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "gardener.cloud/timestamp", now.UTC().String())
			expectedWithState := obj.DeepCopy()
			expectedWithState.Status.State = state
			expectedWithRestore := expectedWithState.DeepCopy()
			expectedWithRestore.Annotations["gardener.cloud/operation"] = "restore"

			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Get(ctx, kutil.Key(obj.Namespace, obj.Name), gomock.AssignableToTypeOf(&extensionsv1alpha1.ControlPlane{})).Return(apierrors.NewNotFound(extensionsv1alpha1.Resource("ControlPlane"), obj.Name))
			mc.EXPECT().Create(ctx, obj)
			mc.EXPECT().Status().Return(mc)
			mc.EXPECT().Update(ctx, expectedWithState)
			mc.EXPECT().Patch(ctx, expectedWithRestore, client.MergeFrom(expectedWithState))

			defaultDepWaiter = controlplane.New(log, mc, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
			Expect(defaultDepWaiter.Restore(ctx, shootState)).To(Succeed())
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

		It("should not return error if resource gets migrated successfully (purpose == exposure)", func() {
			obj := cp.DeepCopy()
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

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

package extension_test

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mocktime "github.com/gardener/gardener/pkg/mock/go/time"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/extension"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Extension", func() {
	const namespace = "test-namespace"

	var (
		defaultDepWaiter extension.Interface
		extensionTypes   = []string{"type1", "type2"}
		extensionsMap    = map[string]extension.Extension{
			"type1": {
				Extension: extensionsv1alpha1.Extension{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name1",
						Namespace: namespace,
					},
				},
				Timeout: time.Millisecond,
			},
		}

		ctrl *gomock.Controller

		ctx      context.Context
		c        client.Client
		empty    *extensionsv1alpha1.Extension
		expected []*extensionsv1alpha1.Extension
		values   *extension.Values
		log      logrus.FieldLogger
		fakeErr  = fmt.Errorf("some random error")

		mockNow *mocktime.MockNow
		now     time.Time
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockNow = mocktime.NewMockNow(ctrl)
		now = time.Now()

		ctx = context.TODO()
		log = logger.NewNopLogger()

		s := runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(s)).To(Succeed())
		c = fake.NewClientBuilder().WithScheme(s).Build()

		empty = &extensionsv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
			},
		}

		expected = make([]*extensionsv1alpha1.Extension, 0, len(extensionsMap))
		for _, ext := range extensionsMap {
			expected = append(expected, &extensionsv1alpha1.Extension{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ext.Name,
					Namespace: ext.Namespace,
					Annotations: map[string]string{
						v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
						v1beta1constants.GardenerTimestamp: now.UTC().String(),
					},
				},
				Spec: extensionsv1alpha1.ExtensionSpec{
					DefaultSpec: extensionsv1alpha1.DefaultSpec{
						Type:           ext.Spec.Type,
						ProviderConfig: ext.Spec.ProviderConfig,
					},
				},
			})
		}

		values = &extension.Values{
			Namespace:  namespace,
			Extensions: extensionsMap,
		}
		defaultDepWaiter = extension.New(log, c, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		It("should successfully deploy all extensions resources", func() {
			defer test.WithVars(&extension.TimeNow, mockNow.Do)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			for _, e := range expected {
				actual := &extensionsv1alpha1.Extension{}
				Expect(c.Get(ctx, client.ObjectKey{Name: e.Name, Namespace: e.Namespace}, actual)).To(Succeed())
				Expect(actual).To(DeepDerivativeEqual(e))
			}
		})
	})

	Describe("#Wait", func() {
		It("should return error when no resources are found", func() {
			Expect(defaultDepWaiter.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
		})

		It("should return error when resource is not ready", func() {
			errDescription := "Some error"

			for i := range expected {
				expected[i].Status.LastError = &gardencorev1beta1.LastError{
					Description: errDescription,
				}
				Expect(c.Create(ctx, expected[i])).To(Succeed(), "creating extensions succeeds")
			}

			Expect(defaultDepWaiter.Wait(ctx)).To(MatchError(ContainSubstring("error during reconciliation: "+errDescription)), "extensions indicates error")
		})

		It("should return error if we haven't observed the latest timestamp annotation", func() {
			defer test.WithVars(
				&extension.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			By("deploy")
			// Deploy should fill internal state with the added timestamp annotation
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			By("patch object")
			for i := range expected {
				patch := client.MergeFrom(expected[i].DeepCopy())
				// remove operation annotation, add old timestamp annotation
				expected[i].ObjectMeta.Annotations = map[string]string{
					v1beta1constants.GardenerTimestamp: now.Add(-time.Millisecond).UTC().String(),
				}
				// set last operation
				expected[i].Status.LastOperation = &gardencorev1beta1.LastOperation{
					State: gardencorev1beta1.LastOperationStateSucceeded,
				}
				Expect(c.Patch(ctx, expected[i], patch)).ToNot(HaveOccurred(), "patching extension succeeds")
			}

			By("wait")
			Expect(defaultDepWaiter.Wait(ctx)).NotTo(Succeed(), "extension indicates error")
		})

		It("should return no error when it's ready", func() {
			defer test.WithVars(
				&extension.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			By("deploy")
			// Deploy should fill internal state with the added timestamp annotation
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			By("patch object")
			for i := range expected {
				patch := client.MergeFrom(expected[i].DeepCopy())
				// remove operation annotation, add up-to-date timestamp annotation
				expected[i].ObjectMeta.Annotations = map[string]string{
					v1beta1constants.GardenerTimestamp: now.UTC().String(),
				}
				// set last operation
				expected[i].Status.LastOperation = &gardencorev1beta1.LastOperation{
					State: gardencorev1beta1.LastOperationStateSucceeded,
				}
				Expect(c.Patch(ctx, expected[i], patch)).ToNot(HaveOccurred(), "patching extension succeeds")
			}

			By("wait")
			Expect(defaultDepWaiter.Wait(ctx)).To(Succeed(), "extension is ready")
		})
	})

	Describe("#Destroy", func() {
		It("should not return error when not found", func() {
			Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())
		})

		It("should not return error when deleted successfully", func() {
			Expect(c.Create(ctx, expected[0])).To(Succeed(), "adding pre-existing extensions succeeds")
			Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())
		})

		It("should return error if not deleted successfully", func() {
			defer test.WithVars(
				&extensions.TimeNow, mockNow.Do,
				&gutil.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			expectedExtension := extensionsv1alpha1.Extension{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ext1",
					Namespace: namespace,
					Annotations: map[string]string{
						gutil.ConfirmationDeletion:         "true",
						v1beta1constants.GardenerTimestamp: now.UTC().String(),
					},
				},
			}

			mc := mockclient.NewMockClient(ctrl)
			// check if the extensions exist
			mc.EXPECT().List(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.ExtensionList{}), client.InNamespace(namespace)).SetArg(1, extensionsv1alpha1.ExtensionList{Items: []extensionsv1alpha1.Extension{expectedExtension}})
			// add deletion confirmation and Timestamp annotation
			mc.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.Extension{}), gomock.Any())
			mc.EXPECT().Delete(ctx, &expectedExtension).Times(1).Return(fakeErr)

			defaultDepWaiter = extension.New(log, mc, &extension.Values{
				Namespace:  namespace,
				Extensions: extensionsMap,
			}, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)

			Expect(defaultDepWaiter.Destroy(ctx)).To(MatchError(multierror.Append(fakeErr)))
		})
	})

	Describe("#WaitCleanup", func() {
		It("should not return error if all resources are gone", func() {
			Expect(defaultDepWaiter.WaitCleanup(ctx)).To(Succeed())
		})

		It("should return error if resources still exist", func() {
			Expect(c.Create(ctx, expected[0])).To(Succeed())
			Expect(defaultDepWaiter.WaitCleanup(ctx)).To(MatchError(ContainSubstring("Extension test-namespace/name1 is still present")))
		})
	})

	Describe("#Restore", func() {
		var (
			state      = []byte(`{"dummy":"state"}`)
			shootState *gardencorev1alpha1.ShootState
		)

		BeforeEach(func() {
			extensions := make([]gardencorev1alpha1.ExtensionResourceState, 0, len(extensionTypes))
			for _, ext := range extensionsMap {
				extensions = append(extensions, gardencorev1alpha1.ExtensionResourceState{
					Name:  pointer.String(ext.Name),
					Kind:  extensionsv1alpha1.ExtensionResource,
					State: &runtime.RawExtension{Raw: state},
				})
			}
			shootState = &gardencorev1alpha1.ShootState{
				Spec: gardencorev1alpha1.ShootStateSpec{
					Extensions: extensions,
				},
			}
		})

		It("should properly restore the extensions state if it exists", func() {
			defer test.WithVars(
				&extension.TimeNow, mockNow.Do,
				&extensions.TimeNow, mockNow.Do,
			)()

			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()
			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Status().Return(mc)

			empty.SetName(expected[0].GetName())
			mc.EXPECT().Get(ctx, client.ObjectKeyFromObject(empty), gomock.AssignableToTypeOf(empty)).
				Return(apierrors.NewNotFound(extensionsv1alpha1.Resource("extensions"), empty.GetName()))

			// deploy with wait-for-state annotation
			expected[0].Annotations[v1beta1constants.GardenerOperation] = v1beta1constants.GardenerOperationWaitForState
			expected[0].Annotations[v1beta1constants.GardenerTimestamp] = now.UTC().String()
			mc.EXPECT().Create(ctx, test.HasObjectKeyOf(expected[0])).
				DoAndReturn(func(ctx context.Context, actual client.Object, opts ...client.CreateOption) error {
					Expect(actual).To(DeepEqual(expected[0]))
					return nil
				})

			// restore state
			expectedWithState := expected[0].DeepCopy()
			expectedWithState.Status = extensionsv1alpha1.ExtensionStatus{
				DefaultStatus: extensionsv1alpha1.DefaultStatus{State: &runtime.RawExtension{Raw: state}},
			}
			test.EXPECTPatch(ctx, mc, expectedWithState, expected[0], types.MergePatchType)

			// annotate with restore annotation
			expectedWithRestore := expectedWithState.DeepCopy()
			expectedWithRestore.Annotations[v1beta1constants.GardenerOperation] = v1beta1constants.GardenerOperationRestore
			test.EXPECTPatch(ctx, mc, expectedWithRestore, expectedWithState, types.MergePatchType)

			defaultDepWaiter = extension.New(
				log,
				mc,
				&extension.Values{
					Namespace:  namespace,
					Extensions: extensionsMap,
				}, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond,
			)

			Expect(defaultDepWaiter.Restore(ctx, shootState)).To(Succeed())
		})
	})

	Describe("#Migrate", func() {
		It("should migrate the resources", func() {
			Expect(c.Create(ctx, expected[0])).To(Succeed(), "creating extensions succeeds")

			Expect(defaultDepWaiter.Migrate(ctx)).To(Succeed())

			annotatedResource := &extensionsv1alpha1.Extension{}
			Expect(c.Get(ctx, client.ObjectKey{Name: expected[0].Name, Namespace: expected[0].Namespace}, annotatedResource)).To(Succeed())
			Expect(annotatedResource.Annotations[v1beta1constants.GardenerOperation]).To(Equal(v1beta1constants.GardenerOperationMigrate))
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
			expected[0].Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}
			expected[0].Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateError,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(c.Create(ctx, expected[0])).To(Succeed(), "creating extensions succeeds")
			Expect(defaultDepWaiter.WaitMigrate(ctx)).To(MatchError(ContainSubstring("is not Migrate=Succeeded")))
		})

		It("should not return error if resource gets migrated successfully", func() {
			expected[0].Status.LastError = nil
			expected[0].Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(c.Create(ctx, expected[0])).To(Succeed(), "creating extensions succeeds")
			Expect(defaultDepWaiter.WaitMigrate(ctx)).To(Succeed(), "extensions is ready, should not return an error")
		})

		It("should return error if one resources is not migrated successfully and others are", func() {
			for i := range expected[1:] {
				expected[i].Status.LastError = nil
				expected[i].Status.LastOperation = &gardencorev1beta1.LastOperation{
					State: gardencorev1beta1.LastOperationStateSucceeded,
					Type:  gardencorev1beta1.LastOperationTypeMigrate,
				}
			}
			expected[0].Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}
			expected[0].Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateError,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			for _, e := range expected {
				Expect(c.Create(ctx, e)).To(Succeed(), "creating extensions succeeds")
			}
			Expect(defaultDepWaiter.WaitMigrate(ctx)).To(MatchError(ContainSubstring("is not Migrate=Succeeded")))
		})
	})

	Describe("#DeleteStaleResources", func() {
		It("should delete stale extensions resources", func() {
			newType := "new-type"

			staleExtension := expected[0].DeepCopy()
			staleExtension.Name = "new-name"
			staleExtension.Spec.Type = newType
			Expect(c.Create(ctx, staleExtension)).To(Succeed(), "creating stale extensions succeeds")

			for _, e := range expected {
				Expect(c.Create(ctx, e)).To(Succeed(), "creating extensions succeeds")
			}

			Expect(defaultDepWaiter.DeleteStaleResources(ctx)).To(Succeed())

			extensionList := &extensionsv1alpha1.ExtensionList{}
			Expect(c.List(ctx, extensionList)).To(Succeed())

			Expect(len(extensionList.Items)).To(Equal(1))
			for _, item := range extensionList.Items {
				Expect(item.Spec.Type).ToNot(Equal(newType))
			}
		})
	})

	Describe("#WaitCleanupStaleResources", func() {
		It("should not return error if all resources are gone", func() {
			Expect(defaultDepWaiter.WaitCleanupStaleResources(ctx)).To(Succeed())
		})

		It("should not return error if wanted resources exist", func() {
			Expect(c.Create(ctx, expected[0])).To(Succeed())
			Expect(defaultDepWaiter.WaitCleanupStaleResources(ctx)).To(Succeed())
		})

		It("should return error if stale resources still exist", func() {
			staleExtension := expected[0].DeepCopy()
			staleExtension.Name = "new-name"
			staleExtension.Spec.Type = "new-type"
			Expect(c.Create(ctx, staleExtension)).To(Succeed(), "creating stale extensions succeeds")

			Expect(defaultDepWaiter.WaitCleanupStaleResources(ctx)).To(MatchError(ContainSubstring("Extension test-namespace/new-name is still present")))
		})
	})
})

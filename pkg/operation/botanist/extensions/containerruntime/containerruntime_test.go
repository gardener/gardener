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

package containerruntime_test

import (
	"context"
	"fmt"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mocktime "github.com/gardener/gardener/pkg/mock/go/time"
	"github.com/gardener/gardener/pkg/operation/botanist/extensions/containerruntime"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/shoot"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/test/gomega"

	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("#ContainerRuntimee", func() {
	const (
		namespace = "test-namespace"
	)

	var (
		workerNames           = []string{"worker1", "worker2"}
		containerRuntimeTypes = []string{"type1", "type2"}

		ctrl *gomock.Controller

		ctx      context.Context
		c        client.Client
		expected []*extensionsv1alpha1.ContainerRuntime
		values   *containerruntime.Values
		log      *logrus.Entry

		defaultDepWaiter shoot.ContainerRuntime
		workers          []gardencorev1beta1.Worker

		mockNow *mocktime.MockNow
		now     time.Time
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockNow = mocktime.NewMockNow(ctrl)

		ctx = context.TODO()

		log = logrus.NewEntry(logger.NewNopLogger())

		s := runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(s)).NotTo(HaveOccurred())

		c = fake.NewFakeClientWithScheme(s)

		workers = make([]gardencorev1beta1.Worker, 0, len(workerNames))

		for _, name := range workerNames {
			containerRuntimes := make([]gardencorev1beta1.ContainerRuntime, 0, len(containerRuntimeTypes))
			for _, runtimeType := range containerRuntimeTypes {
				containerRuntimes = append(containerRuntimes, gardencorev1beta1.ContainerRuntime{
					Type:           runtimeType,
					ProviderConfig: nil,
				})
			}
			workers = append(workers, gardencorev1beta1.Worker{
				Name: name,
				CRI: &gardencorev1beta1.CRI{
					Name:              gardencorev1beta1.CRIName(name),
					ContainerRuntimes: containerRuntimes,
				},
			})
		}

		expected = []*extensionsv1alpha1.ContainerRuntime{}
		for _, name := range workerNames {
			for _, runtimeType := range containerRuntimeTypes {
				expected = append(expected, &extensionsv1alpha1.ContainerRuntime{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-%s", runtimeType, name),
						Namespace: namespace,
						Annotations: map[string]string{
							v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
							v1beta1constants.GardenerTimestamp: now.UTC().String(),
						},
					},
					Spec: extensionsv1alpha1.ContainerRuntimeSpec{
						BinaryPath: extensionsv1alpha1.ContainerDRuntimeContainersBinFolder,
						DefaultSpec: extensionsv1alpha1.DefaultSpec{
							Type:           runtimeType,
							ProviderConfig: nil,
						},
						WorkerPool: extensionsv1alpha1.ContainerRuntimeWorkerPool{
							Name: name,
							Selector: metav1.LabelSelector{
								MatchLabels: map[string]string{v1beta1constants.LabelWorkerPool: name, v1beta1constants.LabelWorkerPoolDeprecated: name},
							},
						},
					},
				})
			}
		}

		values = &containerruntime.Values{
			Namespace: namespace,
			Workers:   workers,
		}
		defaultDepWaiter = containerruntime.New(log, c, values, time.Second, 2*time.Second, 3*time.Second)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		It("should successfully deploy all containerruntime resources", func() {
			defer test.WithVars(
				&containerruntime.TimeNow, mockNow.Do,
			)()

			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			for _, e := range expected {
				actual := &extensionsv1alpha1.ContainerRuntime{}
				err := c.Get(ctx, client.ObjectKey{Name: e.Name, Namespace: e.Namespace}, actual)
				Expect(err).NotTo(HaveOccurred())
				Expect(actual).To(DeepDerivativeEqual(e))
			}
		})
	})

	Describe("#Wait", func() {
		It("should return error when no resources are found", func() {
			Expect(defaultDepWaiter.Wait(ctx)).To(HaveOccurred())
		})

		It("should return error when resource is not ready", func() {
			for i := range expected {
				expected[i].Status.LastError = &gardencorev1beta1.LastError{
					Description: "Some error",
				}
				Expect(c.Create(ctx, expected[i])).ToNot(HaveOccurred(), "creating containerruntime succeeds")
			}

			Expect(defaultDepWaiter.Wait(ctx)).To(HaveOccurred(), "containerruntime indicates error")
		})

		It("should return no error when it's ready", func() {
			for i := range expected {
				// remove operation annotation
				expected[i].ObjectMeta.Annotations = map[string]string{}
				// set last operation
				expected[i].Status.LastOperation = &gardencorev1beta1.LastOperation{
					State: gardencorev1beta1.LastOperationStateSucceeded,
				}
				Expect(c.Create(ctx, expected[i])).ToNot(HaveOccurred(), "creating containerruntime succeeds")
			}

			Expect(defaultDepWaiter.Wait(ctx)).ToNot(HaveOccurred(), "containerruntime is ready, should not return an error")
		})
	})

	Describe("#Destroy", func() {
		It("should not return erorr when not found", func() {
			Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())
		})

		It("should not return error when deleted successfully", func() {
			Expect(c.Create(ctx, expected[0])).ToNot(HaveOccurred(), "adding pre-existing containerruntime succeeds")
			Expect(defaultDepWaiter.Destroy(ctx)).ToNot(HaveOccurred())
		})

		It("should return error if not deleted successfully", func() {
			defer test.WithVars(
				&common.TimeNow, mockNow.Do,
			)()

			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			worker := gardencorev1beta1.Worker{
				Name: workerNames[0],
				CRI: &gardencorev1beta1.CRI{
					Name: gardencorev1beta1.CRIName(workerNames[0]),
					ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{
						{
							Type:           containerRuntimeTypes[0],
							ProviderConfig: nil,
						},
					},
				},
			}

			containerRuntimeName := fmt.Sprintf("%s-%s", containerRuntimeTypes[0], workerNames[0])

			expectedContainerRuntime := extensionsv1alpha1.ContainerRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      containerRuntimeName,
					Namespace: namespace,
					Annotations: map[string]string{
						common.ConfirmationDeletion:        "true",
						v1beta1constants.GardenerTimestamp: now.UTC().String(),
					},
				},
			}

			mc := mockclient.NewMockClient(ctrl)
			// check if the containerruntime exist
			mc.EXPECT().List(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.ContainerRuntimeList{}), client.InNamespace(namespace)).SetArg(1, extensionsv1alpha1.ContainerRuntimeList{Items: []extensionsv1alpha1.ContainerRuntime{expectedContainerRuntime}})
			mc.EXPECT().Get(ctx, kutil.Key(namespace, containerRuntimeName), gomock.AssignableToTypeOf(&extensionsv1alpha1.ContainerRuntime{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, n *extensionsv1alpha1.ContainerRuntime) error {
				return nil
			})
			// add deletion confirmation and Timestamp annotation
			mc.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.ContainerRuntime{})).Return(nil)
			mc.EXPECT().Delete(ctx, &expectedContainerRuntime).Times(1).Return(fmt.Errorf("some random error"))

			defaultDepWaiter = containerruntime.New(log, mc, &containerruntime.Values{
				Namespace: namespace,
				Workers:   []gardencorev1beta1.Worker{worker},
			}, time.Second, 2*time.Second, 3*time.Second)

			err := defaultDepWaiter.Destroy(ctx)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should not return error when resources are removed", func() {
			Expect(defaultDepWaiter.WaitCleanup(ctx)).To(Succeed())
		})

		It("should not return error if resources exist but they don't have deletionTimestamp", func() {
			Expect(c.Create(ctx, expected[0])).To(Succeed())
			Expect(defaultDepWaiter.WaitCleanup(ctx)).To(Succeed())
		})

		It("should return error if resources with deletionTimestamp still exist", func() {
			timeNow := metav1.Now()
			expected[0].DeletionTimestamp = &timeNow
			Expect(c.Create(ctx, expected[0])).To(Succeed())
			Expect(defaultDepWaiter.WaitCleanup(ctx)).To(HaveOccurred())
		})
	})

	Describe("#Restore", func() {
		var (
			shootState *gardencorev1alpha1.ShootState
		)

		BeforeEach(func() {
			extensions := make([]gardencorev1alpha1.ExtensionResourceState, 0, len(workerNames)+len(containerRuntimeTypes))
			for _, name := range workerNames {
				for _, criType := range containerRuntimeTypes {
					extensionName := fmt.Sprintf("%s-%s", criType, name)
					extensions = append(extensions, gardencorev1alpha1.ExtensionResourceState{
						Name:  &extensionName,
						Kind:  extensionsv1alpha1.ContainerRuntimeResource,
						State: &runtime.RawExtension{Raw: []byte("dummy state")},
					})
				}
			}
			shootState = &gardencorev1alpha1.ShootState{
				Spec: gardencorev1alpha1.ShootStateSpec{
					Extensions: extensions,
				},
			}
		})

		It("should properly restore the containerruntime state if it exists", func() {
			defer test.WithVars(
				&containerruntime.TimeNow, mockNow.Do,
				&common.TimeNow, mockNow.Do,
			)()

			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			worker := gardencorev1beta1.Worker{
				Name: workerNames[0],
				CRI: &gardencorev1beta1.CRI{
					Name: gardencorev1beta1.CRIName(workerNames[0]),
					ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{
						{
							Type:           containerRuntimeTypes[0],
							ProviderConfig: nil,
						},
					},
				},
			}

			expected[0].Annotations[v1beta1constants.GardenerOperation] = v1beta1constants.GardenerOperationWaitForState
			expectedWithState := expected[0].DeepCopy()
			expectedWithState.Status = extensionsv1alpha1.ContainerRuntimeStatus{
				DefaultStatus: extensionsv1alpha1.DefaultStatus{State: &runtime.RawExtension{Raw: []byte("dummy state")}},
			}
			expectedWithRestore := expectedWithState.DeepCopy()
			expectedWithRestore.Annotations[v1beta1constants.GardenerOperation] = v1beta1constants.GardenerOperationRestore

			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Get(ctx, kutil.Key(namespace, expected[0].Name), gomock.AssignableToTypeOf(&extensionsv1alpha1.ContainerRuntime{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, n *extensionsv1alpha1.ContainerRuntime) error {
				return apierrors.NewNotFound(extensionsv1alpha1.Resource("ContainerRuntime"), expected[0].Name)
			})
			mc.EXPECT().Create(ctx, expected[0]).Return(nil).Times(1)
			mc.EXPECT().Status().DoAndReturn(func() *mockclient.MockClient {
				return mc
			})
			mc.EXPECT().Update(ctx, expectedWithState).Return(nil)
			mc.EXPECT().Patch(ctx, expectedWithRestore, client.MergeFrom(expectedWithState))

			defaultDepWaiter = containerruntime.New(
				log,
				mc,
				&containerruntime.Values{
					namespace,
					[]gardencorev1beta1.Worker{worker},
				}, time.Second, 2*time.Second, 3*time.Second)

			Expect(defaultDepWaiter.Restore(ctx, shootState)).To(Succeed())
		})
	})

	Describe("#Migrate", func() {
		It("should migrate the resources", func() {
			Expect(c.Create(ctx, expected[0])).To(Succeed(), "creating containerruntime succeeds")

			Expect(defaultDepWaiter.Migrate(ctx)).To(Succeed())

			annotatedResource := &extensionsv1alpha1.ContainerRuntime{}
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

			Expect(c.Create(ctx, expected[0])).To(Succeed(), "creating containerruntime succeeds")
			Expect(defaultDepWaiter.WaitMigrate(ctx)).To(HaveOccurred())
		})

		It("should not return error if resource gets migrated successfully", func() {
			expected[0].Status.LastError = nil
			expected[0].Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(c.Create(ctx, expected[0])).ToNot(HaveOccurred(), "creating containerruntime succeeds")
			Expect(defaultDepWaiter.WaitMigrate(ctx)).ToNot(HaveOccurred(), "containerruntime is ready, should not return an error")
		})

		It("should return error if one resources is not migrated succesfully and others are", func() {
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
				Expect(c.Create(ctx, e)).To(Succeed(), "creating containerruntime succeeds")
			}
			Expect(defaultDepWaiter.WaitMigrate(ctx)).To(HaveOccurred())
		})
	})

	Describe("#DeleteStaleResources", func() {
		It("should delete stale containerruntime resources", func() {
			staleContainerRuntime := expected[0].DeepCopy()
			staleContainerRuntime.Name = fmt.Sprintf("%s-%s", "new-type", workerNames[0])
			staleContainerRuntime.Spec.Type = "new-type"
			Expect(c.Create(ctx, staleContainerRuntime)).To(Succeed(), "creating containerruntime succeeds")

			for _, e := range expected {
				Expect(c.Create(ctx, e)).To(Succeed(), "creating containerruntime succeeds")
			}

			Expect(defaultDepWaiter.DeleteStaleResources(ctx)).To(Succeed())

			containerRuntimeList := &extensionsv1alpha1.ContainerRuntimeList{}
			Expect(c.List(ctx, containerRuntimeList)).To(Succeed())

			Expect(len(containerRuntimeList.Items)).To(Equal(4))
			for _, item := range containerRuntimeList.Items {
				Expect(item.Spec.Type).ToNot(Equal("new-type"))
			}
		})
	})
})

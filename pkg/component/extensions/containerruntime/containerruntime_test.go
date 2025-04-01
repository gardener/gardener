// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package containerruntime_test

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/containerruntime"
	"github.com/gardener/gardener/pkg/extensions"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mocktime "github.com/gardener/gardener/third_party/mock/go/time"
)

var _ = Describe("#ContainerRuntime", func() {
	const namespace = "test-namespace"

	var (
		workerNames           = []string{"worker1", "worker2"}
		containerRuntimeTypes = []string{"type1", "type2"}

		ctrl *gomock.Controller

		ctx      context.Context
		c        client.Client
		empty    *extensionsv1alpha1.ContainerRuntime
		expected []*extensionsv1alpha1.ContainerRuntime
		values   *containerruntime.Values
		log      logr.Logger

		defaultDepWaiter containerruntime.Interface
		workers          []gardencorev1beta1.Worker

		mockNow *mocktime.MockNow
		now     time.Time
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockNow = mocktime.NewMockNow(ctrl)
		now = time.Now()

		ctx = context.TODO()
		log = logr.Discard()

		s := runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(s)).NotTo(HaveOccurred())
		c = fake.NewClientBuilder().WithScheme(s).Build()

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

		empty = &extensionsv1alpha1.ContainerRuntime{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
			},
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
							v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
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
		defaultDepWaiter = containerruntime.New(log, c, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
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

				// ignore changes to TypeMeta and resourceVersion
				actual.SetGroupVersionKind(schema.GroupVersionKind{})
				actual.SetResourceVersion("")

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

		It("should return error if we haven't observed the latest timestamp annotation", func() {
			defer test.WithVars(
				&containerruntime.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			By("Deploy")
			// Deploy should fill internal state with the added timestamp annotation
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			By("Patch object")
			for i := range expected {
				patch := client.MergeFrom(expected[i].DeepCopy())
				// remove operation annotation, add old timestamp annotation
				expected[i].Annotations = map[string]string{
					v1beta1constants.GardenerTimestamp: now.Add(-time.Millisecond).UTC().Format(time.RFC3339Nano),
				}
				// set last operation
				expected[i].Status.LastOperation = &gardencorev1beta1.LastOperation{
					State: gardencorev1beta1.LastOperationStateSucceeded,
				}
				Expect(c.Patch(ctx, expected[i], patch)).ToNot(HaveOccurred(), "patching containerruntime succeeds")
			}

			By("Wait")
			Expect(defaultDepWaiter.Wait(ctx)).NotTo(Succeed(), "containerruntime indicates error")
		})

		It("should return no error when it's ready", func() {
			defer test.WithVars(
				&containerruntime.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			By("Deploy")
			// Deploy should fill internal state with the added timestamp annotation
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			By("Patch object")
			for i := range expected {
				patch := client.MergeFrom(expected[i].DeepCopy())
				// remove operation annotation, add up-to-date timestamp annotation
				expected[i].Annotations = map[string]string{
					v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
				}
				// set last operation
				expected[i].Status.LastOperation = &gardencorev1beta1.LastOperation{
					State:          gardencorev1beta1.LastOperationStateSucceeded,
					LastUpdateTime: metav1.Time{Time: now.UTC().Add(time.Second)},
				}
				Expect(c.Patch(ctx, expected[i], patch)).ToNot(HaveOccurred(), "patching containerruntime succeeds")
			}

			By("Wait")
			Expect(defaultDepWaiter.Wait(ctx)).To(Succeed(), "containerruntime is ready, should not return an error")
		})
	})

	Describe("#Destroy", func() {
		It("should not return error when not found", func() {
			Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())
		})

		It("should not return error when deleted successfully", func() {
			Expect(c.Create(ctx, expected[0])).ToNot(HaveOccurred(), "adding pre-existing containerruntime succeeds")
			Expect(defaultDepWaiter.Destroy(ctx)).ToNot(HaveOccurred())
		})

		It("should return error if not deleted successfully", func() {
			defer test.WithVars(
				&extensions.TimeNow, mockNow.Do,
				&gardenerutils.TimeNow, mockNow.Do,
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
						v1beta1constants.ConfirmationDeletion: "true",
						v1beta1constants.GardenerTimestamp:    now.UTC().Format(time.RFC3339Nano),
					},
				},
			}

			mc := mockclient.NewMockClient(ctrl)
			// check if the containerruntime exist
			mc.EXPECT().List(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.ContainerRuntimeList{}), client.InNamespace(namespace)).SetArg(1, extensionsv1alpha1.ContainerRuntimeList{Items: []extensionsv1alpha1.ContainerRuntime{expectedContainerRuntime}})
			// add deletion confirmation and Timestamp annotation
			mc.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.ContainerRuntime{}), gomock.Any())
			mc.EXPECT().Delete(ctx, &expectedContainerRuntime).Times(1).Return(fmt.Errorf("some random error"))

			defaultDepWaiter = containerruntime.New(log, mc, &containerruntime.Values{
				Namespace: namespace,
				Workers:   []gardencorev1beta1.Worker{worker},
			}, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)

			err := defaultDepWaiter.Destroy(ctx)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should not return error if all resources are gone", func() {
			Expect(defaultDepWaiter.WaitCleanup(ctx)).To(Succeed())
		})

		It("should return error if resources still exist", func() {
			Expect(c.Create(ctx, expected[0])).To(Succeed())
			Expect(defaultDepWaiter.WaitCleanup(ctx)).To(MatchError(ContainSubstring("ContainerRuntime test-namespace/type1-worker1 is still present")))
		})
	})

	Describe("#Restore", func() {
		var (
			shootState *gardencorev1beta1.ShootState
		)

		BeforeEach(func() {
			extensions := make([]gardencorev1beta1.ExtensionResourceState, 0, len(workerNames)+len(containerRuntimeTypes))
			for _, name := range workerNames {
				for _, criType := range containerRuntimeTypes {
					extensionName := fmt.Sprintf("%s-%s", criType, name)
					extensions = append(extensions, gardencorev1beta1.ExtensionResourceState{
						Name:  &extensionName,
						Kind:  extensionsv1alpha1.ContainerRuntimeResource,
						State: &runtime.RawExtension{Raw: []byte(`{"dummy":"state"}`)},
					})
				}
			}
			shootState = &gardencorev1beta1.ShootState{
				Spec: gardencorev1beta1.ShootStateSpec{
					Extensions: extensions,
				},
			}
		})

		It("should properly restore the containerruntime state if it exists", func() {
			defer test.WithVars(
				&containerruntime.TimeNow, mockNow.Do,
				&extensions.TimeNow, mockNow.Do,
			)()

			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()
			mc := mockclient.NewMockClient(ctrl)
			mockStatusWriter := mockclient.NewMockStatusWriter(ctrl)

			mc.EXPECT().Status().Return(mockStatusWriter)

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

			empty.SetName(expected[0].GetName())
			mc.EXPECT().Get(ctx, client.ObjectKeyFromObject(empty), gomock.AssignableToTypeOf(empty)).
				Return(apierrors.NewNotFound(extensionsv1alpha1.Resource("containerruntimes"), expected[0].GetName()))

			// deploy with wait-for-state annotation
			expected[0].Annotations[v1beta1constants.GardenerOperation] = v1beta1constants.GardenerOperationWaitForState
			expected[0].Annotations[v1beta1constants.GardenerTimestamp] = now.UTC().Format(time.RFC3339Nano)
			mc.EXPECT().Create(ctx, test.HasObjectKeyOf(expected[0])).
				DoAndReturn(func(_ context.Context, actual client.Object, _ ...client.CreateOption) error {
					Expect(actual).To(DeepEqual(expected[0]))
					return nil
				})

			// restore state
			expectedWithState := expected[0].DeepCopy()
			expectedWithState.Status = extensionsv1alpha1.ContainerRuntimeStatus{
				DefaultStatus: extensionsv1alpha1.DefaultStatus{State: &runtime.RawExtension{Raw: []byte(`{"dummy":"state"}`)}},
			}
			test.EXPECTStatusPatch(ctx, mockStatusWriter, expectedWithState, expected[0], types.MergePatchType)

			// annotate with restore annotation
			expectedWithRestore := expectedWithState.DeepCopy()
			expectedWithRestore.Annotations[v1beta1constants.GardenerOperation] = v1beta1constants.GardenerOperationRestore
			test.EXPECTPatch(ctx, mc, expectedWithRestore, expectedWithState, types.MergePatchType)

			defaultDepWaiter = containerruntime.New(
				log,
				mc,
				&containerruntime.Values{
					Namespace: namespace,
					Workers:   []gardencorev1beta1.Worker{worker},
				}, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)

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
			Expect(c.Create(ctx, staleContainerRuntime)).To(Succeed(), "creating stale containerruntime succeeds")

			for _, e := range expected {
				Expect(c.Create(ctx, e)).To(Succeed(), "creating containerruntime succeeds")
			}

			Expect(defaultDepWaiter.DeleteStaleResources(ctx)).To(Succeed())

			containerRuntimeList := &extensionsv1alpha1.ContainerRuntimeList{}
			Expect(c.List(ctx, containerRuntimeList)).To(Succeed())

			Expect(containerRuntimeList.Items).To(HaveLen(4))
			for _, item := range containerRuntimeList.Items {
				Expect(item.Spec.Type).ToNot(Equal("new-type"))
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
			staleContainerRuntime := expected[0].DeepCopy()
			staleContainerRuntime.Name = fmt.Sprintf("%s-%s", "new-type", workerNames[0])
			staleContainerRuntime.Spec.Type = "new-type"
			Expect(c.Create(ctx, staleContainerRuntime)).To(Succeed(), "creating stale containerruntime succeeds")

			Expect(defaultDepWaiter.WaitCleanupStaleResources(ctx)).To(MatchError(ContainSubstring("ContainerRuntime test-namespace/new-type-worker1 is still present")))
		})
	})
})

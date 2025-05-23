// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure_test

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/infrastructure"
	"github.com/gardener/gardener/pkg/extensions"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mocktime "github.com/gardener/gardener/third_party/mock/go/time"
)

var _ = Describe("#Interface", func() {
	const (
		namespace    = "test-namespace"
		name         = "test-deploy"
		providerType = "foo"
	)

	var (
		ctx     context.Context
		log     logr.Logger
		fakeErr = errors.New("fake")

		ctrl    *gomock.Controller
		c       client.Client
		mockNow *mocktime.MockNow
		now     time.Time

		region         string
		sshPublicKey   []byte
		providerConfig *runtime.RawExtension
		providerStatus *runtime.RawExtension
		nodesCIDRs     []string
		servicesCIDRs  []string
		podsCIDRs      []string
		egressCIDRs    []string

		empty, expected *extensionsv1alpha1.Infrastructure
		values          *infrastructure.Values
		deployWaiter    infrastructure.Interface
		waiter          *retryfake.Ops

		cleanupFunc func()
	)

	BeforeEach(func() {
		ctx = context.TODO()
		log = logr.Discard()

		ctrl = gomock.NewController(GinkgoT())
		mockNow = mocktime.NewMockNow(ctrl)
		now = time.Now()

		s := runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(s)).To(Succeed())
		c = fake.NewClientBuilder().WithScheme(s).Build()

		region = "europe"
		sshPublicKey = []byte("secure")
		providerConfig = &runtime.RawExtension{Raw: []byte(`{"very":"provider-specific"}`)}
		providerStatus = &runtime.RawExtension{Raw: []byte(`{"very":"provider-specific-status"}`)}
		nodesCIDRs = []string{"1.2.3.4/5", "2.3.4.5/6"}
		servicesCIDRs = []string{"5.6.7.8/9", "6.7.8.9/10"}
		podsCIDRs = []string{"10.11.12.13/14", "11.12.13.14/15"}
		egressCIDRs = []string{"1.2.3.4/5", "5.6.7.8/9"}

		values = &infrastructure.Values{
			Namespace:      namespace,
			Name:           name,
			Type:           providerType,
			ProviderConfig: providerConfig,
			Region:         region,
		}

		empty = &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}

		expected = &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Annotations: map[string]string{
					v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
					v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
				},
			},
			Spec: extensionsv1alpha1.InfrastructureSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type:           providerType,
					ProviderConfig: providerConfig,
				},
				Region:       region,
				SSHPublicKey: sshPublicKey,
				SecretRef: corev1.SecretReference{
					Name:      v1beta1constants.SecretNameCloudProvider,
					Namespace: namespace,
				},
			},
		}

		waiter = &retryfake.Ops{MaxAttempts: 1}
		cleanupFunc = test.WithVars(
			&retry.Until, waiter.Until,
			&retry.UntilTimeout, waiter.UntilTimeout,
		)

		deployWaiter = infrastructure.New(log, c, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
	})

	AfterEach(func() {
		ctrl.Finish()
		cleanupFunc()
	})

	Describe("#Deploy", func() {
		BeforeEach(func() {
			expected.ResourceVersion = "1"
		})

		It("correct Infrastructure is created (AnnotateOperation=false)", func() {
			defer test.WithVars(
				&infrastructure.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			deployWaiter.SetSSHPublicKey([]byte(""))
			Expect(deployWaiter.Deploy(ctx)).To(Succeed())

			actual := &extensionsv1alpha1.Infrastructure{}
			expected.Spec.SSHPublicKey = []byte("")
			expected.Annotations = map[string]string{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(expected), actual)).To(Succeed())
			Expect(actual).To(DeepEqual(expected))
		})

		It("correct Infrastructure is created (AnnotateOperation=true)", func() {
			defer test.WithVars(
				&infrastructure.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			values.AnnotateOperation = true
			deployWaiter.SetSSHPublicKey(sshPublicKey)
			Expect(deployWaiter.Deploy(ctx)).To(Succeed())

			actual := &extensionsv1alpha1.Infrastructure{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(expected), actual)).To(Succeed())
			Expect(actual).To(DeepEqual(expected))
		})

		It("should deploy the Infrastructure with operation annotation if it is in error state", func() {
			defer test.WithVars(
				&infrastructure.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()
			existingInfra := expected.DeepCopy()
			existingInfra.ResourceVersion = ""
			delete(existingInfra.Annotations, v1beta1constants.GardenerOperation)
			metav1.SetMetaDataAnnotation(&existingInfra.ObjectMeta, v1beta1constants.GardenerTimestamp, now.UTC().Add(-time.Second).Format(time.RFC3339Nano))
			existingInfra.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateError,
			}

			expected.ResourceVersion = "2"
			expected.Status = extensionsv1alpha1.InfrastructureStatus{
				DefaultStatus: extensionsv1alpha1.DefaultStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateError,
					},
				},
			}

			Expect(c.Create(ctx, existingInfra)).To(Succeed())
			values.AnnotateOperation = false
			deployWaiter = infrastructure.New(log, c, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
			deployWaiter.SetSSHPublicKey(sshPublicKey)
			Expect(deployWaiter.Deploy(ctx)).To(Succeed())

			deployedInfra := &extensionsv1alpha1.Infrastructure{ObjectMeta: metav1.ObjectMeta{
				Name:      existingInfra.Name,
				Namespace: existingInfra.Namespace,
			}}
			err := c.Get(ctx, client.ObjectKeyFromObject(deployedInfra), deployedInfra)
			Expect(err).NotTo(HaveOccurred())
			Expect(deployedInfra).To(DeepEqual(expected))
		})

		It("should deploy the Infrastructure with operation annotation if gardener timestamp is after status.lastOperation.lastUpdateTime", func() {
			defer test.WithVars(
				&infrastructure.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()
			existingInfra := expected.DeepCopy()
			existingInfra.ResourceVersion = ""
			delete(existingInfra.Annotations, v1beta1constants.GardenerOperation)
			metav1.SetMetaDataAnnotation(&existingInfra.ObjectMeta, v1beta1constants.GardenerTimestamp, now.UTC().Add(time.Second*10).Format(time.RFC3339Nano))
			existingInfra.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: metav1.NewTime(now.UTC()),
			}

			expected.ResourceVersion = "2"
			expected.Status = extensionsv1alpha1.InfrastructureStatus{
				DefaultStatus: extensionsv1alpha1.DefaultStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State:          gardencorev1beta1.LastOperationStateSucceeded,
						LastUpdateTime: metav1.NewTime(now.Truncate(time.Second)), // this is also truncated when read from the client later on in the test
					},
				},
			}

			Expect(c.Create(ctx, existingInfra)).To(Succeed())
			values.AnnotateOperation = false
			deployWaiter = infrastructure.New(log, c, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
			deployWaiter.SetSSHPublicKey(sshPublicKey)
			Expect(deployWaiter.Deploy(ctx)).To(Succeed())

			deployedInfra := &extensionsv1alpha1.Infrastructure{ObjectMeta: metav1.ObjectMeta{
				Name:      existingInfra.Name,
				Namespace: existingInfra.Namespace,
			}}
			err := c.Get(ctx, client.ObjectKeyFromObject(deployedInfra), deployedInfra)
			Expect(err).NotTo(HaveOccurred())
			Expect(deployedInfra).To(DeepEqual(expected))
		})
	})

	Describe("#Wait", func() {
		It("should return error when it's not found", func() {
			Expect(deployWaiter.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
		})

		It("should return error when it's not ready", func() {
			expected.Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}

			Expect(c.Create(ctx, expected)).To(Succeed(), "creating infrastructure succeeds")
			Expect(deployWaiter.Wait(ctx)).To(MatchError(ContainSubstring("error during reconciliation: Some error")))
		})

		It("should return error if we haven't observed the latest timestamp annotation", func() {
			defer test.WithVars(
				&infrastructure.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			By("Deploy")
			// Deploy should fill internal state with the added timestamp annotation
			values.AnnotateOperation = true
			deployWaiter.SetSSHPublicKey(sshPublicKey)
			Expect(deployWaiter.Deploy(ctx)).To(Succeed())

			By("Patch object")
			patch := client.MergeFrom(expected.DeepCopy())
			expected.Status.LastError = nil
			// remove operation annotation, add old timestamp annotation
			expected.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.Add(-time.Millisecond).UTC().Format(time.RFC3339Nano),
			}
			expected.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}
			Expect(c.Patch(ctx, expected, patch)).To(Succeed(), "patching infrastructure succeeds")

			By("Wait")
			Expect(deployWaiter.Wait(ctx)).NotTo(Succeed(), "infrastructure indicates error")
		})

		It("should return no error when is ready", func() {
			defer test.WithVars(
				&infrastructure.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			By("Deploy")
			// Deploy should fill internal state with the added timestamp annotation
			values.AnnotateOperation = true
			deployWaiter.SetSSHPublicKey(sshPublicKey)
			Expect(deployWaiter.Deploy(ctx)).To(Succeed())

			By("Patch object")
			patch := client.MergeFrom(expected.DeepCopy())
			expected.Status.LastError = nil
			// remove operation annotation, add up-to-date timestamp annotation
			expected.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
			}
			expected.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: metav1.Time{Time: now.UTC().Add(time.Second)},
			}
			expected.Status.NodesCIDR = &nodesCIDRs[0]
			expected.Status.Networking = &extensionsv1alpha1.InfrastructureStatusNetworking{
				Nodes:    nodesCIDRs,
				Services: servicesCIDRs,
				Pods:     podsCIDRs,
			}
			expected.Status.ProviderStatus = providerStatus
			expected.Status.EgressCIDRs = egressCIDRs
			Expect(c.Patch(ctx, expected, patch)).To(Succeed(), "patching infrastructure succeeds")

			By("Wait")
			Expect(deployWaiter.Wait(ctx)).To(Succeed(), "infrastructure is ready")

			By("Verify status")
			Expect(deployWaiter.ProviderStatus()).To(Equal(providerStatus))
			Expect(deployWaiter.NodesCIDRs()).To(Equal(nodesCIDRs))
			Expect(deployWaiter.ServicesCIDRs()).To(Equal(servicesCIDRs))
			Expect(deployWaiter.PodsCIDRs()).To(Equal(podsCIDRs))
			Expect(deployWaiter.EgressCIDRs()).To(Equal(egressCIDRs))
		})

		It("should return no error when is ready (AnnotateOperation == false)", func() {
			expected.Status.LastError = nil
			expected.Annotations = map[string]string{}
			expected.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}
			oldNodes := "9.8.7.6/5"
			expected.Status.NodesCIDR = ptr.To(oldNodes)
			expected.Status.Networking = &extensionsv1alpha1.InfrastructureStatusNetworking{
				Nodes:    nodesCIDRs,
				Services: servicesCIDRs,
				Pods:     podsCIDRs,
			}
			expected.Status.ProviderStatus = providerStatus
			expected.Status.EgressCIDRs = egressCIDRs

			Expect(c.Create(ctx, expected)).To(Succeed(), "creating infrastructure succeeds")
			Expect(deployWaiter.Wait(ctx)).To(Succeed())
			Expect(deployWaiter.ProviderStatus()).To(Equal(providerStatus))
			Expect(deployWaiter.NodesCIDRs()).To(Equal(append([]string{oldNodes}, nodesCIDRs...)))
			Expect(deployWaiter.ServicesCIDRs()).To(Equal(servicesCIDRs))
			Expect(deployWaiter.PodsCIDRs()).To(Equal(podsCIDRs))
			Expect(deployWaiter.EgressCIDRs()).To(Equal(egressCIDRs))
		})
	})

	Describe("#Destroy", func() {
		It("should not return error when it's not found", func() {
			Expect(deployWaiter.Destroy(ctx)).To(Succeed())
		})

		It("should not return error when it's deleted successfully", func() {
			Expect(c.Create(ctx, expected)).To(Succeed(), "creating infrastructure succeeds")
			Expect(deployWaiter.Destroy(ctx)).To(Succeed())
		})

		It("should return error when it's not deleted successfully", func() {
			defer test.WithVars(
				&extensions.TimeNow, mockNow.Do,
				&gardenerutils.TimeNow, mockNow.Do,
			)()

			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()
			mc := mockclient.NewMockClient(ctrl)

			expected = empty.DeepCopy()
			expected.SetAnnotations(map[string]string{
				v1beta1constants.ConfirmationDeletion: "true",
				v1beta1constants.GardenerTimestamp:    now.UTC().Format(time.RFC3339Nano),
			})

			// add deletion confirmation and timestamp annotation
			mc.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.Infrastructure{}), gomock.Any()).Return(nil)
			mc.EXPECT().Delete(ctx, expected).Times(1).Return(fakeErr)

			deployWaiter = infrastructure.New(log, mc, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
			Expect(deployWaiter.Destroy(ctx)).To(MatchError(fakeErr))
		})
	})

	Describe("#WaitCleanup", func() {
		It("should not return error when it's already removed", func() {
			Expect(deployWaiter.WaitCleanup(ctx)).To(Succeed())
		})

		It("should return error when it's not deleted successfully", func() {
			expected.Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}

			Expect(c.Create(ctx, expected)).To(Succeed(), "creating infrastructure succeeds")
			Expect(deployWaiter.WaitCleanup(ctx)).To(MatchError(ContainSubstring("Some error")))
		})
	})

	Describe("#Restore", func() {
		var (
			state      = &runtime.RawExtension{Raw: []byte(`{"dummy":"state"}`)}
			shootState = &gardencorev1beta1.ShootState{
				Spec: gardencorev1beta1.ShootStateSpec{
					Extensions: []gardencorev1beta1.ExtensionResourceState{
						{
							Name:  ptr.To(name),
							Kind:  extensionsv1alpha1.InfrastructureResource,
							State: state,
						},
					},
				},
			}
		)

		It("should properly restore the infrastructure state if it exists", func() {
			defer test.WithVars(
				&infrastructure.TimeNow, mockNow.Do,
				&extensions.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			mc := mockclient.NewMockClient(ctrl)
			mockStatusWriter := mockclient.NewMockStatusWriter(ctrl)

			mc.EXPECT().Status().Return(mockStatusWriter)

			values.SSHPublicKey = sshPublicKey
			values.AnnotateOperation = true

			mc.EXPECT().Get(ctx, client.ObjectKeyFromObject(empty), gomock.AssignableToTypeOf(empty)).
				Return(apierrors.NewNotFound(extensionsv1alpha1.Resource("infrastructures"), name))

			// deploy with wait-for-state annotation
			obj := expected.DeepCopy()
			metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "gardener.cloud/operation", "wait-for-state")
			metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "gardener.cloud/timestamp", now.UTC().Format(time.RFC3339Nano))
			obj.TypeMeta = metav1.TypeMeta{}
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

			Expect(infrastructure.New(log, mc, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond).Restore(ctx, shootState)).To(Succeed())
		})
	})

	Describe("#Migrate", func() {
		It("should migrate the resources", func() {
			defer test.WithVars(
				&infrastructure.TimeNow, mockNow.Do,
				&extensions.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			Expect(c.Create(ctx, expected)).To(Succeed(), "creating infrastructure succeeds")

			deployWaiter.SetSSHPublicKey(sshPublicKey)
			Expect(deployWaiter.Migrate(ctx)).To(Succeed())

			actual := &extensionsv1alpha1.Infrastructure{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(expected), actual)).To(Succeed())
			expected.SetResourceVersion("2")
			metav1.SetMetaDataAnnotation(&expected.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationMigrate)
			metav1.SetMetaDataAnnotation(&expected.ObjectMeta, v1beta1constants.GardenerTimestamp, now.UTC().Format(time.RFC3339Nano))
			Expect(actual).To(DeepEqual(expected))
		})

		It("should not return error if resource does not exist", func() {
			Expect(deployWaiter.Migrate(ctx)).To(Succeed())
		})
	})

	Describe("#WaitMigrate", func() {
		It("should not return error when resource is missing", func() {
			Expect(deployWaiter.WaitMigrate(ctx)).To(Succeed())
		})

		It("should return error if resource is not yet migrated successfully", func() {
			expected.Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}

			expected.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateError,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(c.Create(ctx, expected)).To(Succeed(), "creating infrastructure succeeds")
			Expect(deployWaiter.WaitMigrate(ctx)).To(MatchError(ContainSubstring("to be successfully migrated")))
		})

		It("should not return error if resource gets migrated successfully", func() {
			expected.Status.LastError = nil
			expected.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(c.Create(ctx, expected)).To(Succeed(), "creating infrastructure succeeds")
			Expect(deployWaiter.WaitMigrate(ctx)).To(Succeed(), "infrastructure is ready, should not return an error")
		})
	})

	Describe("#Get", func() {
		It("should return an error when the retrieval fails", func() {
			res, err := deployWaiter.Get(ctx)
			Expect(res).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})

		It("should retrieve the object and extract the status", func() {
			Expect(deployWaiter.ProviderStatus()).To(BeNil())
			Expect(deployWaiter.NodesCIDRs()).To(BeEmpty())
			Expect(deployWaiter.ServicesCIDRs()).To(BeEmpty())
			Expect(deployWaiter.PodsCIDRs()).To(BeEmpty())

			var (
				providerStatus = &runtime.RawExtension{Raw: []byte(`{"some":"status"}`)}
				nodesCIDR      = ptr.To("1.2.3.4")
			)

			infra := empty.DeepCopy()
			infra.Status.ProviderStatus = providerStatus
			infra.Status.NodesCIDR = nodesCIDR
			infra.Status.Networking = &extensionsv1alpha1.InfrastructureStatusNetworking{
				Nodes:    nodesCIDRs,
				Services: servicesCIDRs,
				Pods:     podsCIDRs,
			}
			infra.Status.EgressCIDRs = egressCIDRs
			Expect(c.Create(ctx, infra)).To(Succeed())

			expected = infra.DeepCopy()
			actual, err := deployWaiter.Get(ctx)
			Expect(err).NotTo(HaveOccurred())
			actual.SetGroupVersionKind(schema.GroupVersionKind{})
			Expect(actual).To(DeepEqual(expected))

			Expect(deployWaiter.ProviderStatus()).To(Equal(providerStatus))
			Expect(deployWaiter.NodesCIDRs()).To(Equal(append([]string{*nodesCIDR}, nodesCIDRs...)))
			Expect(deployWaiter.ServicesCIDRs()).To(Equal(servicesCIDRs))
			Expect(deployWaiter.PodsCIDRs()).To(Equal(podsCIDRs))
			Expect(deployWaiter.EgressCIDRs()).To(Equal(egressCIDRs))
		})
	})
})

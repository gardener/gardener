// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/controlplane"
	"github.com/gardener/gardener/pkg/extensions"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ControlPlane", func() {
	var (
		ctrl       *gomock.Controller
		fakeClient client.Client

		fakeClock *testclock.FakeClock
		now       time.Time
		scheme    *runtime.Scheme

		ctx = context.TODO()
		log = logr.Discard()

		name                         = "test"
		namespace                    = "testnamespace"
		extensionType                = "some-type"
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
		now = time.Unix(60, 0)
		fakeClock = testclock.NewFakeClock(now)

		scheme = runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
		fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme).Build()

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
			Region: region,
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
			Region:                       region,
			InfrastructureProviderStatus: infrastructureProviderStatus,
		}
		defaultDepWaiter = controlplane.New(log, fakeClient, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		It("should successfully deploy the ControlPlane resource", func() {
			defer test.WithVars(&controlplane.TimeNow, fakeClock.Now)()

			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			obj := &extensionsv1alpha1.ControlPlane{}
			err := fakeClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, obj)
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
			Expect(fakeClient.Create(ctx, obj)).To(Succeed(), "creating controlplane succeeds")

			Expect(defaultDepWaiter.Wait(ctx)).To(HaveOccurred(), "controlplane indicates error")
		})

		It("should return error if we haven't observed the latest timestamp annotation", func() {
			defer test.WithVars(&controlplane.TimeNow, fakeClock.Now)()

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
			Expect(fakeClient.Patch(ctx, cp, patch)).To(Succeed(), "patching controlplane succeeds")

			By("Wait")
			Expect(defaultDepWaiter.Wait(ctx)).NotTo(Succeed(), "controlplane indicates error")
		})

		It("should return no error when it's ready", func() {
			defer test.WithVars(&controlplane.TimeNow, fakeClock.Now)()

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
			Expect(fakeClient.Patch(ctx, cp, patch)).To(Succeed(), "patching controlplane succeeds")

			By("Wait")
			Expect(defaultDepWaiter.Wait(ctx)).To(Succeed(), "controlplane is ready")
		})
	})

	Describe("#Destroy", func() {
		It("should not return error when not found", func() {
			Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())
		})

		It("should not return error when deleted successfully", func() {
			Expect(fakeClient.Create(ctx, cp.DeepCopy())).To(Succeed(), "adding pre-existing controlplane succeeds")
			Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())
		})

		It("should return error if not deleted successfully", func() {
			defer test.WithVars(
				&extensions.TimeNow, fakeClock.Now,
				&gardenerutils.TimeNow, fakeClock.Now,
			)()

			fakeErr := fmt.Errorf("some random error")

			fakeClient := fakeclient.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
				Delete: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
					if _, ok := obj.(*extensionsv1alpha1.ControlPlane); ok {
						return fakeErr
					}
					return client.Delete(ctx, obj, opts...)
				},
			}).Build()

			Expect(fakeClient.Create(ctx, cp.DeepCopy())).To(Succeed())

			err := controlplane.New(log, fakeClient, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond).Destroy(ctx)
			Expect(err).To(MatchError(fakeErr))
		})
	})

	Describe("#WaitCleanup", func() {
		It("should not return error when resources are removed", func() {
			Expect(defaultDepWaiter.WaitCleanup(ctx)).To(Succeed())
		})

		It("should return error if resources with deletionTimestamp still exist", func() {
			timeNow := metav1.Now()
			obj := cp.DeepCopy()
			obj.DeletionTimestamp = &timeNow
			Expect(fakeClient.Create(ctx, obj)).To(Succeed())

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
							Name:  &name,
							Kind:  extensionsv1alpha1.ControlPlaneResource,
							State: state,
						},
					},
				},
			}
		})

		It("should properly restore the controlplane state if it exists", func() {
			defer test.WithVars(
				&controlplane.TimeNow, fakeClock.Now,
				&extensions.TimeNow, fakeClock.Now,
			)()

			c := fakeclient.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&extensionsv1alpha1.ControlPlane{}).Build()

			Expect(controlplane.New(log, c, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond).Restore(ctx, shootState)).To(Succeed())

			// Verify the ControlPlane was created with restore annotation
			actual := &extensionsv1alpha1.ControlPlane{}
			Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, actual)).To(Succeed())
			Expect(actual.Status.State).To(Equal(state))
			Expect(actual.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationRestore))
			Expect(actual.Spec).To(Equal(cpSpec))
		})
	})

	Describe("#Migrate", func() {
		It("should migrate the resources", func() {
			Expect(fakeClient.Create(ctx, cp.DeepCopy())).To(Succeed(), "creating controlplane succeeds")

			Expect(defaultDepWaiter.Migrate(ctx)).To(Succeed())

			result := &extensionsv1alpha1.ControlPlane{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, result)).To(Succeed())
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

			Expect(fakeClient.Create(ctx, obj)).To(Succeed(), "creating controlplane succeeds")
			Expect(defaultDepWaiter.WaitMigrate(ctx)).To(HaveOccurred())
		})

		It("should not return error if resource gets migrated successfully", func() {
			obj := cp.DeepCopy()
			obj.Status.LastError = nil
			obj.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(fakeClient.Create(ctx, obj)).To(Succeed(), "creating controlplane succeeds")
			Expect(defaultDepWaiter.WaitMigrate(ctx)).To(Succeed(), "controlplane is ready, should not return an error")
		})
	})
})

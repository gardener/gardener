// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package network_test

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/extensions/network"
	"github.com/gardener/gardener/pkg/extensions"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("#Network", func() {
	const (
		networkNs          = "test-namespace"
		networkName        = "test-deploy"
		networkType        = "calico"
		networkPodIp       = "100.96.0.0"
		networkPodMask     = 11
		networkServiceIp   = "100.64.0.0"
		networkServiceMask = 13

		networkPodV6IP       = "2001:db8:1::"
		networkPodV6Mask     = 48
		networkServiceV6IP   = "2001:db8:3::"
		networkServiceV6Mask = 108
	)
	var (
		ctrl *gomock.Controller

		ctx              context.Context
		c                client.Client
		scheme           *runtime.Scheme
		expected, empty  *extensionsv1alpha1.Network
		values           *network.Values
		log              logr.Logger
		defaultDepWaiter component.DeployMigrateWaiter

		fakeClock *testclock.FakeClock
		now       time.Time

		networkPodCIDR     = fmt.Sprintf("%s/%d", networkPodIp, networkPodMask)
		networkServiceCIDR = fmt.Sprintf("%s/%d", networkServiceIp, networkServiceMask)

		networkPodV6CIDR     = fmt.Sprintf("%s/%d", networkPodV6IP, networkPodV6Mask)
		networkServiceV6CIDR = fmt.Sprintf("%s/%d", networkServiceV6IP, networkServiceV6Mask)
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		now = time.Unix(60, 0)
		fakeClock = testclock.NewFakeClock(now)

		ctx = context.TODO()
		log = logr.Discard()

		scheme = runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())

		c = fakeclient.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&extensionsv1alpha1.Network{}).Build()

		podCIDR := net.IPNet{
			IP:   net.ParseIP(networkPodIp),
			Mask: net.CIDRMask(networkPodMask, 32),
		}
		serviceCIDR := net.IPNet{
			IP:   net.ParseIP(networkServiceIp),
			Mask: net.CIDRMask(networkServiceMask, 32),
		}

		values = &network.Values{
			Name:           networkName,
			Namespace:      networkNs,
			Type:           networkType,
			ProviderConfig: nil,
			PodCIDRs:       []net.IPNet{podCIDR, serviceCIDR},
			ServiceCIDRs:   []net.IPNet{serviceCIDR, podCIDR},
			IPFamilies:     []extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv4},
		}

		empty = &extensionsv1alpha1.Network{
			ObjectMeta: metav1.ObjectMeta{
				Name:      networkName,
				Namespace: networkNs,
			},
		}
		expected = &extensionsv1alpha1.Network{
			ObjectMeta: metav1.ObjectMeta{
				Name:      networkName,
				Namespace: networkNs,
				Annotations: map[string]string{
					v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
					v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
				},
			},
			Spec: extensionsv1alpha1.NetworkSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type:           networkType,
					ProviderConfig: nil,
				},
				PodCIDR:     networkPodCIDR,
				ServiceCIDR: networkServiceCIDR,
				IPFamilies:  []extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv4},
			},
		}

		defaultDepWaiter = network.New(log, c, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		testFunc := func() {
			defer test.WithVars(
				&network.TimeNow, fakeClock.Now,
			)()

			Expect(defaultDepWaiter.Deploy(ctx)).ToNot(HaveOccurred())

			actual := &extensionsv1alpha1.Network{}
			err := c.Get(ctx, client.ObjectKey{Name: networkName, Namespace: networkNs}, actual)

			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(DeepDerivativeEqual(expected))
		}

		Context("IPv4", func() {
			It("should create correct Network", func() {
				testFunc()
			})
		})

		Context("IPv6", func() {
			BeforeEach(func() {
				podCIDR := net.IPNet{
					IP:   net.ParseIP(networkPodV6IP),
					Mask: net.CIDRMask(networkPodV6Mask, 128),
				}
				serviceCIDR := net.IPNet{
					IP:   net.ParseIP(networkServiceV6IP),
					Mask: net.CIDRMask(networkServiceV6Mask, 128),
				}

				values.PodCIDRs = []net.IPNet{podCIDR, serviceCIDR}
				values.ServiceCIDRs = []net.IPNet{serviceCIDR, podCIDR}
				values.IPFamilies = []extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv6}

				expected.Spec.PodCIDR = networkPodV6CIDR
				expected.Spec.ServiceCIDR = networkServiceV6CIDR
				expected.Spec.IPFamilies = []extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv6}

			})
			It("should create correct Network", func() {
				testFunc()
			})
		})
	})

	Describe("#Wait", func() {
		It("should return error when it's not found", func() {
			Expect(defaultDepWaiter.Wait(ctx)).To(HaveOccurred())
		})

		It("should return error when it's not ready", func() {
			expected.Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}

			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "creating network succeeds")
			Expect(defaultDepWaiter.Wait(ctx)).To(HaveOccurred(), "network indicates error")
		})

		It("should return error if we haven't observed the latest timestamp annotation", func() {
			defer test.WithVars(
				&network.TimeNow, fakeClock.Now,
			)()

			By("Deploy")
			// Deploy should fill internal state with the added timestamp annotation
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			By("Patch object")
			patch := client.MergeFrom(expected.DeepCopy())
			// remove operation annotation, add old timestamp annotation
			expected.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.Add(-time.Millisecond).UTC().Format(time.RFC3339Nano),
			}

			Expect(c.Patch(ctx, expected, patch)).To(Succeed(), "patching network succeeds")

			expected.Status.LastError = nil
			expected.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}
			Expect(c.Status().Patch(ctx, expected, patch)).To(Succeed(), "patching network status succeeds")

			By("Wait")
			Expect(defaultDepWaiter.Wait(ctx)).NotTo(Succeed(), "network indicates error")
		})

		It("should return no error when it's ready", func() {
			defer test.WithVars(
				&network.TimeNow, fakeClock.Now,
			)()

			By("Deploy")
			// Deploy should fill internal state with the added timestamp annotation
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			By("Patch object")
			patch := client.MergeFrom(expected.DeepCopy())
			// remove operation annotation, add up-to-date timestamp annotation
			expected.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
			}

			Expect(c.Patch(ctx, expected, patch)).To(Succeed(), "patching network succeeds")

			expected.Status.LastError = nil
			expected.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: metav1.Time{Time: now.UTC().Add(time.Second)},
			}
			Expect(c.Status().Patch(ctx, expected, patch)).To(Succeed(), "patching network status succeeds")

			By("Wait")
			Expect(defaultDepWaiter.Wait(ctx)).To(Succeed(), "network is ready")
		})
	})

	Describe("#Destroy", func() {
		It("should not return error when it's not found", func() {
			Expect(defaultDepWaiter.Destroy(ctx)).ToNot(HaveOccurred())
		})

		It("should not return error when it's deleted successfully", func() {
			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "adding pre-existing network succeeds")

			Expect(defaultDepWaiter.Destroy(ctx)).ToNot(HaveOccurred())
		})

		It("should return error when it's not deleted successfully", func() {
			defer test.WithVars(
				&extensions.TimeNow, fakeClock.Now,
				&gardenerutils.TimeNow, fakeClock.Now,
			)()

			expected := extensionsv1alpha1.Network{
				ObjectMeta: metav1.ObjectMeta{
					Name:      networkName,
					Namespace: networkNs,
					Annotations: map[string]string{
						v1beta1constants.ConfirmationDeletion: "true",
						v1beta1constants.GardenerTimestamp:    now.UTC().Format(time.RFC3339Nano),
					},
				}}

			fakeErr := fmt.Errorf("some random error")

			c := fakeclient.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
				Delete: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
					if _, ok := obj.(*extensionsv1alpha1.Network); ok {
						return fakeErr
					}
					return client.Delete(ctx, obj, opts...)
				},
			}).Build()

			Expect(c.Create(ctx, expected.DeepCopy())).To(Succeed())

			defaultDepWaiter = network.New(log, c, &network.Values{
				Namespace: networkNs,
				Name:      networkName,
			}, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)

			err := defaultDepWaiter.Destroy(ctx)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should not return error when it's already removed", func() {
			Expect(defaultDepWaiter.WaitCleanup(ctx)).ToNot(HaveOccurred())
		})
	})

	Describe("#Restore", func() {
		var (
			shootState *gardencorev1beta1.ShootState
		)

		BeforeEach(func() {
			shootState = &gardencorev1beta1.ShootState{
				Spec: gardencorev1beta1.ShootStateSpec{
					Extensions: []gardencorev1beta1.ExtensionResourceState{
						{
							Name:  &expected.Name,
							Kind:  extensionsv1alpha1.NetworkResource,
							State: &runtime.RawExtension{Raw: []byte(`{"dummy":"state"}`)},
						},
					},
				},
			}
		})

		It("should restore the network state if it exists in the shoot state", func() {
			defer test.WithVars(
				&network.TimeNow, fakeClock.Now,
				&extensions.TimeNow, fakeClock.Now,
			)()

			c := fakeclient.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&extensionsv1alpha1.Network{}).Build()

			defaultDepWaiter = network.New(log, c, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
			Expect(defaultDepWaiter.Restore(ctx, shootState)).To(Succeed())

			// Verify the Network was created with restore annotation
			actual := &extensionsv1alpha1.Network{}
			Expect(c.Get(ctx, client.ObjectKey{Name: networkName, Namespace: networkNs}, actual)).To(Succeed())
			Expect(actual.Status.State).To(Equal(&runtime.RawExtension{Raw: []byte(`{"dummy":"state"}`)}))
			Expect(actual.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationRestore))
		})
	})

	Describe("#Migrate", func() {
		It("should migrate the resource", func() {
			defer test.WithVars(
				&network.TimeNow, fakeClock.Now,
				&extensions.TimeNow, fakeClock.Now,
			)()
			Expect(c.Create(ctx, expected.DeepCopy())).To(Succeed())

			Expect(defaultDepWaiter.Migrate(ctx)).To(Succeed())

			actual := &extensionsv1alpha1.Network{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(empty), actual)).To(Succeed())
			Expect(actual.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationMigrate))
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
			expected.Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}

			expected.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateError,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(c.Create(ctx, expected)).To(Succeed(), "creating network succeeds")
			Expect(defaultDepWaiter.WaitMigrate(ctx)).To(HaveOccurred())
		})

		It("should not return error if resource gets migrated successfully", func() {
			expected.Status.LastError = nil
			expected.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "creating network succeeds")
			Expect(defaultDepWaiter.WaitMigrate(ctx)).ToNot(HaveOccurred(), "network is ready, should not return an error")
		})
	})
})

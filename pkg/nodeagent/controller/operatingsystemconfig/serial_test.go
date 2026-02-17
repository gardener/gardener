// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

var _ = Describe("#serialReconciliation", func() {
	It("should return true when annotation is 'true'", func() {
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{v1beta1constants.AnnotationNodeAgentSerialOSCReconciliation: "true"}}}
		Expect(serialReconciliation(s)).To(BeTrue())
	})

	It("should return false when annotation is absent", func() {
		Expect(serialReconciliation(&corev1.Secret{})).To(BeFalse())
	})

	It("should return false when annotation is 'false'", func() {
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{v1beta1constants.AnnotationNodeAgentSerialOSCReconciliation: "false"}}}
		Expect(serialReconciliation(s)).To(BeFalse())
	})

	It("should return false when annotation is empty string", func() {
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{v1beta1constants.AnnotationNodeAgentSerialOSCReconciliation: ""}}}
		Expect(serialReconciliation(s)).To(BeFalse())
	})
})

var _ = Describe("#newLeaderElectorForSecret", func() {
	var (
		log        = logr.Discard()
		fakeClient client.Client
		fakeClock  clock.Clock
		identity   = "test-identity"
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeClock = testclock.NewFakeClock(time.Now())
	})

	It("should return leaderElector with nil lease when annotation is not set", func() {
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "s", Namespace: "ns"}}

		le := newLeaderElectorForSecret(log, fakeClient, fakeClock, secret, identity)
		Expect(le).NotTo(BeNil())
		Expect(le.lease).To(BeNil())
		Expect(le.identity).To(Equal(identity))
	})

	It("should return leaderElector with correct lease when annotation is 'true'", func() {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret",
				Namespace: "test-namespace",
				UID:       "test-uid",
				Annotations: map[string]string{
					v1beta1constants.AnnotationNodeAgentSerialOSCReconciliation: "true",
				},
			},
		}

		le := newLeaderElectorForSecret(log, fakeClient, fakeClock, secret, identity)
		Expect(le).NotTo(BeNil())
		Expect(le.identity).To(Equal(identity))
		Expect(le.lease).NotTo(BeNil())
		Expect(le.lease.Name).To(Equal("test-secret"))
		Expect(le.lease.Namespace).To(Equal("test-namespace"))
		Expect(le.lease.OwnerReferences).To(ConsistOf(metav1.OwnerReference{
			APIVersion:         "v1",
			Kind:               "Secret",
			Name:               "test-secret",
			UID:                "test-uid",
			Controller:         ptr.To(true),
			BlockOwnerDeletion: ptr.To(true),
		}))
	})
})

var _ = Describe("leaderElector", func() {
	var (
		ctx        context.Context
		now        time.Time
		clk        *testclock.FakeClock
		fakeClient client.Client
		le         *leaderElector
		identity   = "test-node"
		lease      *coordinationv1.Lease
	)

	BeforeEach(func() {
		ctx = context.Background()
		now = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		clk = testclock.NewFakeClock(now)
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		lease = &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{Name: "test-lease", Namespace: "test-ns"}}
		le = &leaderElector{log: logr.Discard(), client: fakeClient, clock: clk, identity: identity, lease: lease}
	})

	Describe("#acquiredByMe", func() {
		It("should return true when HolderIdentity matches identity", func() {
			le.lease.Spec.HolderIdentity = ptr.To(identity)
			Expect(le.acquiredByMe()).To(BeTrue())
		})

		It("should return false when HolderIdentity is nil", func() {
			Expect(le.acquiredByMe()).To(BeFalse())
		})

		It("should return false when HolderIdentity is another identity", func() {
			le.lease.Spec.HolderIdentity = ptr.To("other")
			Expect(le.acquiredByMe()).To(BeFalse())
		})

		It("should return false when HolderIdentity is empty string", func() {
			le.lease.Spec.HolderIdentity = ptr.To("")
			Expect(le.acquiredByMe()).To(BeFalse())
		})
	})

	Describe("#acquiredByAnotherInstance", func() {
		It("should return false when HolderIdentity is nil", func() {
			Expect(le.acquiredByAnotherInstance()).To(BeFalse())
		})

		It("should return false when HolderIdentity is empty", func() {
			le.lease.Spec.HolderIdentity = ptr.To("")
			Expect(le.acquiredByAnotherInstance()).To(BeFalse())
		})

		It("should return false when HolderIdentity is own identity", func() {
			le.lease.Spec.HolderIdentity = ptr.To(identity)
			Expect(le.acquiredByAnotherInstance()).To(BeFalse())
		})

		It("should return true when HolderIdentity is a different non-empty identity", func() {
			le.lease.Spec.HolderIdentity = ptr.To("other")
			Expect(le.acquiredByAnotherInstance()).To(BeTrue())
		})
	})

	Describe("#leaseDuration", func() {
		It("should return zero when LeaseDurationSeconds is nil", func() {
			Expect(le.leaseDuration()).To(Equal(time.Duration(0)))
		})

		It("should return zero when LeaseDurationSeconds is 0", func() {
			le.lease.Spec.LeaseDurationSeconds = ptr.To[int32](0)
			Expect(le.leaseDuration()).To(Equal(time.Duration(0)))
		})

		It("should return the correct duration", func() {
			le.lease.Spec.LeaseDurationSeconds = ptr.To[int32](600)
			Expect(le.leaseDuration()).To(Equal(600 * time.Second))
		})
	})

	Describe("#renewedInTime", func() {
		It("should return false when LeaseDurationSeconds is nil", func() {
			le.lease.Spec.RenewTime = &metav1.MicroTime{Time: now}
			Expect(le.renewedInTime()).To(BeFalse())
		})

		It("should return false when LeaseDurationSeconds is negative", func() {
			le.lease.Spec.LeaseDurationSeconds = ptr.To[int32](-1)
			le.lease.Spec.RenewTime = &metav1.MicroTime{Time: now}
			Expect(le.renewedInTime()).To(BeFalse())
		})

		It("should return false when RenewTime is nil", func() {
			le.lease.Spec.LeaseDurationSeconds = ptr.To[int32](600)
			Expect(le.renewedInTime()).To(BeFalse())
		})

		It("should return true when within lease duration", func() {
			le.lease.Spec.LeaseDurationSeconds = ptr.To[int32](600)
			le.lease.Spec.RenewTime = &metav1.MicroTime{Time: now.Add(-300 * time.Second)}
			Expect(le.renewedInTime()).To(BeTrue())
		})

		It("should return false when lease has expired", func() {
			le.lease.Spec.LeaseDurationSeconds = ptr.To[int32](600)
			le.lease.Spec.RenewTime = &metav1.MicroTime{Time: now.Add(-700 * time.Second)}
			Expect(le.renewedInTime()).To(BeFalse())
		})

		It("should return false when renewTime+duration exactly equals now", func() {
			le.lease.Spec.LeaseDurationSeconds = ptr.To[int32](600)
			le.lease.Spec.RenewTime = &metav1.MicroTime{Time: now.Add(-600 * time.Second)}
			Expect(le.renewedInTime()).To(BeFalse())
		})
	})

	Describe("#durationUntilLeaseExpires", func() {
		It("should return zero when LeaseDurationSeconds is nil", func() {
			le.lease.Spec.RenewTime = &metav1.MicroTime{Time: now}
			Expect(le.durationUntilLeaseExpires()).To(Equal(time.Duration(0)))
		})

		It("should return zero when LeaseDurationSeconds is negative", func() {
			le.lease.Spec.LeaseDurationSeconds = ptr.To[int32](-1)
			le.lease.Spec.RenewTime = &metav1.MicroTime{Time: now}
			Expect(le.durationUntilLeaseExpires()).To(Equal(time.Duration(0)))
		})

		It("should return zero when RenewTime is nil", func() {
			le.lease.Spec.LeaseDurationSeconds = ptr.To[int32](600)
			Expect(le.durationUntilLeaseExpires()).To(Equal(time.Duration(0)))
		})

		It("should return the remaining duration when lease has not expired", func() {
			le.lease.Spec.LeaseDurationSeconds = ptr.To[int32](600)
			le.lease.Spec.RenewTime = &metav1.MicroTime{Time: now.Add(-100 * time.Second)}
			Expect(le.durationUntilLeaseExpires()).To(Equal(500 * time.Second))
		})

		It("should return a negative duration when lease has already expired", func() {
			le.lease.Spec.LeaseDurationSeconds = ptr.To[int32](600)
			le.lease.Spec.RenewTime = &metav1.MicroTime{Time: now.Add(-700 * time.Second)}
			Expect(le.durationUntilLeaseExpires()).To(BeNumerically("<", 0))
		})
	})

	Describe("#reload", func() {
		It("should reload an existing lease", func() {
			existing := &coordinationv1.Lease{
				ObjectMeta: metav1.ObjectMeta{Name: lease.Name, Namespace: lease.Namespace},
				Spec:       coordinationv1.LeaseSpec{HolderIdentity: ptr.To("some-holder")},
			}
			Expect(fakeClient.Create(ctx, existing)).To(Succeed())
			Expect(le.reload(ctx)).To(Succeed())
			Expect(le.lease.Spec.HolderIdentity).To(Equal(ptr.To("some-holder")))
		})

		It("should not error when lease does not exist (IgnoreNotFound)", func() {
			Expect(le.reload(ctx)).To(Succeed())
			Expect(le.lease.Spec.HolderIdentity).To(BeNil())
		})
	})

	Describe("#tryAcquireOrRenew", func() {
		When("lease does not yet exist", func() {
			It("should create lease with holder identity, duration and timings", func() {
				Expect(le.tryAcquireOrRenew(ctx)).To(Succeed())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(lease), lease)).To(Succeed())
				Expect(lease.Spec.HolderIdentity).To(Equal(ptr.To(identity)))
				Expect(lease.Spec.LeaseDurationSeconds).To(PointTo(Equal(int32(600))))
				Expect(lease.Spec.AcquireTime).NotTo(BeNil())
				Expect(lease.Spec.AcquireTime.UTC()).To(Equal(now.UTC()))
				Expect(lease.Spec.RenewTime).NotTo(BeNil())
				Expect(lease.Spec.RenewTime.UTC()).To(Equal(now.UTC()))
			})

			It("should not overwrite AcquireTime when already the holder", func() {
				acquireTime := now.Add(-5 * time.Minute)
				le.lease.Spec.HolderIdentity = ptr.To(identity)
				le.lease.Spec.LeaseDurationSeconds = ptr.To[int32](600)
				le.lease.Spec.AcquireTime = &metav1.MicroTime{Time: acquireTime}

				Expect(le.tryAcquireOrRenew(ctx)).To(Succeed())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(lease), lease)).To(Succeed())
				Expect(lease.Spec.AcquireTime.UTC()).To(Equal(acquireTime.UTC()))
				Expect(lease.Spec.RenewTime.UTC()).To(Equal(now.UTC()))
			})

			It("should reset AcquireTime when taking over from another holder", func() {
				le.lease.Spec.HolderIdentity = ptr.To("other")
				le.lease.Spec.AcquireTime = &metav1.MicroTime{Time: now.Add(-10 * time.Minute)}

				Expect(le.tryAcquireOrRenew(ctx)).To(Succeed())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(lease), lease)).To(Succeed())
				Expect(lease.Spec.HolderIdentity).To(Equal(ptr.To(identity)))
				Expect(lease.Spec.AcquireTime.UTC()).To(Equal(now.UTC()))
				Expect(lease.Spec.RenewTime.UTC()).To(Equal(now.UTC()))
			})
		})

		When("lease already exists", func() {
			BeforeEach(func() {
				Expect(fakeClient.Create(ctx, lease)).To(Succeed())
			})

			It("should update the lease with holder identity and timings", func() {
				Expect(le.tryAcquireOrRenew(ctx)).To(Succeed())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(lease), lease)).To(Succeed())
				Expect(lease.Spec.HolderIdentity).To(Equal(ptr.To(identity)))
				Expect(lease.Spec.LeaseDurationSeconds).To(PointTo(Equal(int32(600))))
				Expect(lease.Spec.AcquireTime.UTC()).To(Equal(now.UTC()))
				Expect(lease.Spec.RenewTime.UTC()).To(Equal(now.UTC()))
			})

			It("should only update RenewTime when already the holder", func() {
				acquireTime := now.Add(-5 * time.Minute)
				le.lease.Spec.HolderIdentity = ptr.To(identity)
				le.lease.Spec.LeaseDurationSeconds = ptr.To[int32](600)
				le.lease.Spec.AcquireTime = &metav1.MicroTime{Time: acquireTime}
				Expect(fakeClient.Update(ctx, le.lease)).To(Succeed())

				Expect(le.tryAcquireOrRenew(ctx)).To(Succeed())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(lease), lease)).To(Succeed())
				Expect(lease.Spec.AcquireTime.UTC()).To(Equal(acquireTime.UTC()))
				Expect(lease.Spec.RenewTime.UTC()).To(Equal(now.UTC()))
			})
		})
	})

	Describe("#release", func() {
		It("should do nothing when lease is nil", func() {
			le.lease = nil
			Expect(le.release(ctx)).To(Succeed())
		})

		It("should do nothing when not the holder", func() {
			le.lease.Spec.HolderIdentity = ptr.To("other")
			Expect(fakeClient.Create(ctx, le.lease)).To(Succeed())

			Expect(le.release(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(lease), lease)).To(Succeed())
			Expect(lease.Spec.HolderIdentity).To(Equal(ptr.To("other")))
		})

		It("should clear all lease spec fields when the current holder releases", func() {
			t := metav1.NewMicroTime(now)
			le.lease.Spec.HolderIdentity = ptr.To(identity)
			le.lease.Spec.LeaseDurationSeconds = ptr.To[int32](600)
			le.lease.Spec.AcquireTime = &t
			le.lease.Spec.RenewTime = &t
			Expect(fakeClient.Create(ctx, le.lease)).To(Succeed())

			Expect(le.release(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(lease), lease)).To(Succeed())
			Expect(lease.Spec.HolderIdentity).To(BeNil())
			Expect(lease.Spec.LeaseDurationSeconds).To(BeNil())
			Expect(lease.Spec.AcquireTime).To(BeNil())
			Expect(lease.Spec.RenewTime).To(BeNil())
		})
	})
})

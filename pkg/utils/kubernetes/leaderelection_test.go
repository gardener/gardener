// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/gardener/gardener/pkg/utils/kubernetes"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("LeaderElection", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		ctx       = context.TODO()
		fakeErr   = errors.New("fake err")
		namespace = "namespace"
		name      = "name"
		lock      string

		holderIdentity             = "leader1"
		leaseDurationSeconds int32 = 42
		acquireTime                = metav1.Time{Time: time.Date(2020, 12, 14, 13, 18, 29, 176023, time.Local)}
		renewTime                  = metav1.Time{Time: time.Date(2020, 12, 14, 13, 18, 29, 176023, time.Local)}
		acquireTimeMicro           = metav1.MicroTime{Time: time.Date(2020, 12, 14, 13, 18, 29, 176023, time.Local)}
		renewTimeMicro             = metav1.MicroTime{Time: time.Date(2020, 12, 14, 13, 18, 29, 176023, time.Local)}
		leaderTransitions    int32 = 24

		objectMetaInvalid = metav1.ObjectMeta{Annotations: map[string]string{"control-plane.alpha.kubernetes.io/leader": "[foo]"}}
		objectMetaValid   = metav1.ObjectMeta{Annotations: map[string]string{"control-plane.alpha.kubernetes.io/leader": fmt.Sprintf(`{
  "holderIdentity":%q,
  "leaseDurationSeconds":%d,
  "acquireTime":%q,
  "renewTime":%q,
  "leaderTransitions":%d
}`,
			holderIdentity,
			leaseDurationSeconds,
			acquireTime.Format(time.RFC3339Nano),
			renewTime.Format(time.RFC3339Nano),
			leaderTransitions,
		)}}
		leaseSpecValid = coordinationv1.LeaseSpec{
			HolderIdentity:       &holderIdentity,
			LeaseDurationSeconds: &leaseDurationSeconds,
			AcquireTime:          &acquireTimeMicro,
			RenewTime:            &renewTimeMicro,
			LeaseTransitions:     &leaderTransitions,
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#ReadLeaderElectionRecord", func() {
		Context("endpoints lock", func() {
			BeforeEach(func() {
				lock = "endpoints"
			})

			It("should fail if the object cannot be retrieved", func() {
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Endpoints{})).Return(fakeErr)

				lock, err := ReadLeaderElectionRecord(ctx, c, lock, namespace, name)
				Expect(lock).To(BeNil())
				Expect(err).To(MatchError(fakeErr))
			})

			It("should fail if the object has no leader election annotation", func() {
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Endpoints{}))

				lock, err := ReadLeaderElectionRecord(ctx, c, lock, namespace, name)
				Expect(lock).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("could not find key \"control-plane.alpha.kubernetes.io/leader\" in annotations")))
			})

			It("should fail if the leader election annotation cannot be unmarshalled", func() {
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Endpoints{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Endpoints, _ ...client.GetOption) error {
					(&corev1.Endpoints{ObjectMeta: objectMetaInvalid}).DeepCopyInto(obj)
					return nil
				})

				lock, err := ReadLeaderElectionRecord(ctx, c, lock, namespace, name)
				Expect(lock).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("failed to unmarshal leader election record")))
			})

			It("should successfully return the leader election record", func() {
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Endpoints{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Endpoints, _ ...client.GetOption) error {
					(&corev1.Endpoints{ObjectMeta: objectMetaValid}).DeepCopyInto(obj)
					return nil
				})

				lock, err := ReadLeaderElectionRecord(ctx, c, lock, namespace, name)
				Expect(lock).To(Equal(&resourcelock.LeaderElectionRecord{
					HolderIdentity:       holderIdentity,
					LeaseDurationSeconds: int(leaseDurationSeconds),
					AcquireTime:          acquireTime,
					RenewTime:            renewTime,
					LeaderTransitions:    int(leaderTransitions),
				}))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("configmaps lock", func() {
			BeforeEach(func() {
				lock = "configmaps"
			})

			It("should fail if the object cannot be retrieved", func() {
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).Return(fakeErr)

				lock, err := ReadLeaderElectionRecord(ctx, c, lock, namespace, name)
				Expect(lock).To(BeNil())
				Expect(err).To(MatchError(fakeErr))
			})

			It("should fail if the object has no leader election annotation", func() {
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.ConfigMap{}))

				lock, err := ReadLeaderElectionRecord(ctx, c, lock, namespace, name)
				Expect(lock).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("could not find key \"control-plane.alpha.kubernetes.io/leader\" in annotations")))
			})

			It("should fail if the leader election annotation cannot be unmarshalled", func() {
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.ConfigMap, _ ...client.GetOption) error {
					(&corev1.ConfigMap{ObjectMeta: objectMetaInvalid}).DeepCopyInto(obj)
					return nil
				})

				lock, err := ReadLeaderElectionRecord(ctx, c, lock, namespace, name)
				Expect(lock).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("failed to unmarshal leader election record")))
			})

			It("should successfully return the leader election record", func() {
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.ConfigMap, _ ...client.GetOption) error {
					(&corev1.ConfigMap{ObjectMeta: objectMetaValid}).DeepCopyInto(obj)
					return nil
				})

				lock, err := ReadLeaderElectionRecord(ctx, c, lock, namespace, name)
				Expect(lock).To(Equal(&resourcelock.LeaderElectionRecord{
					HolderIdentity:       holderIdentity,
					LeaseDurationSeconds: int(leaseDurationSeconds),
					AcquireTime:          acquireTime,
					RenewTime:            renewTime,
					LeaderTransitions:    int(leaderTransitions),
				}))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("leases lock", func() {
			BeforeEach(func() {
				lock = resourcelock.LeasesResourceLock
			})

			It("should fail if the object cannot be retrieved", func() {
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&coordinationv1.Lease{})).Return(fakeErr)

				lock, err := ReadLeaderElectionRecord(ctx, c, lock, namespace, name)
				Expect(lock).To(BeNil())
				Expect(err).To(MatchError(fakeErr))
			})

			It("should successfully return the leader election record", func() {
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&coordinationv1.Lease{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *coordinationv1.Lease, _ ...client.GetOption) error {
					(&coordinationv1.Lease{Spec: leaseSpecValid}).DeepCopyInto(obj)
					return nil
				})

				lock, err := ReadLeaderElectionRecord(ctx, c, lock, namespace, name)
				Expect(lock).To(Equal(&resourcelock.LeaderElectionRecord{
					HolderIdentity:       holderIdentity,
					LeaseDurationSeconds: int(leaseDurationSeconds),
					AcquireTime:          acquireTime,
					RenewTime:            renewTime,
					LeaderTransitions:    int(leaderTransitions),
				}))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})

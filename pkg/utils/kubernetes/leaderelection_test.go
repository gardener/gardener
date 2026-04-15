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
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	. "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("LeaderElection", func() {
	var (
		ctx       = context.TODO()
		fakeErr   = errors.New("fake err")
		namespace = "namespace"
		name      = "name"
		lock      string

		holderIdentity             = "leader1"
		leaseDurationSeconds int32 = 42
		acquireTime                = metav1.Time{Time: time.Date(2020, 12, 14, 13, 18, 29, 176023000, time.Local)}
		renewTime                  = metav1.Time{Time: time.Date(2020, 12, 14, 13, 18, 29, 176023000, time.Local)}
		acquireTimeMicro           = metav1.MicroTime{Time: time.Date(2020, 12, 14, 13, 18, 29, 176023000, time.Local)}
		renewTimeMicro             = metav1.MicroTime{Time: time.Date(2020, 12, 14, 13, 18, 29, 176023000, time.Local)}
		leaderTransitions    int32 = 24

		objectMetaInvalid = metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: map[string]string{"control-plane.alpha.kubernetes.io/leader": "[foo]"},
		}
		objectMetaValid = metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{"control-plane.alpha.kubernetes.io/leader": fmt.Sprintf(`{
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
			)},
		}
		leaseSpecValid = coordinationv1.LeaseSpec{
			HolderIdentity:       &holderIdentity,
			LeaseDurationSeconds: &leaseDurationSeconds,
			AcquireTime:          &acquireTimeMicro,
			RenewTime:            &renewTimeMicro,
			LeaseTransitions:     &leaderTransitions,
		}
	)

	Describe("#ReadLeaderElectionRecord", func() {
		var fakeClient client.Client

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
		})

		Context("endpoints lock", func() {
			BeforeEach(func() {
				lock = "endpoints"
			})

			It("should fail if the object cannot be retrieved", func() {
				fakeClient = fakeclient.NewClientBuilder().
					WithScheme(kubernetesscheme.Scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
							return fakeErr
						},
					}).
					Build()

				lock, err := ReadLeaderElectionRecord(ctx, fakeClient, lock, namespace, name)
				Expect(lock).To(BeNil())
				Expect(err).To(MatchError(fakeErr))
			})

			It("should fail if the object has no leader election annotation", func() {
				endpoints := &corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
					},
				}

				Expect(fakeClient.Create(ctx, endpoints)).To(Succeed())

				lock, err := ReadLeaderElectionRecord(ctx, fakeClient, lock, namespace, name)
				Expect(lock).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("could not find key \"control-plane.alpha.kubernetes.io/leader\" in annotations")))
			})

			It("should fail if the leader election annotation cannot be unmarshalled", func() {
				Expect(fakeClient.Create(ctx, &corev1.Endpoints{ObjectMeta: objectMetaInvalid})).To(Succeed())

				lock, err := ReadLeaderElectionRecord(ctx, fakeClient, lock, namespace, name)
				Expect(lock).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("failed to unmarshal leader election record")))
			})

			It("should successfully return the leader election record", func() {
				Expect(fakeClient.Create(ctx, &corev1.Endpoints{ObjectMeta: objectMetaValid})).To(Succeed())

				lock, err := ReadLeaderElectionRecord(ctx, fakeClient, lock, namespace, name)
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
				fakeClient = fakeclient.NewClientBuilder().
					WithScheme(kubernetesscheme.Scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
							return fakeErr
						},
					}).
					Build()

				lock, err := ReadLeaderElectionRecord(ctx, fakeClient, lock, namespace, name)
				Expect(lock).To(BeNil())
				Expect(err).To(MatchError(fakeErr))
			})

			It("should fail if the object has no leader election annotation", func() {
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
					},
				}

				Expect(fakeClient.Create(ctx, configMap)).To(Succeed())

				lock, err := ReadLeaderElectionRecord(ctx, fakeClient, lock, namespace, name)
				Expect(lock).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("could not find key \"control-plane.alpha.kubernetes.io/leader\" in annotations")))
			})

			It("should fail if the leader election annotation cannot be unmarshalled", func() {
				Expect(fakeClient.Create(ctx, &corev1.ConfigMap{ObjectMeta: objectMetaInvalid})).To(Succeed())

				lock, err := ReadLeaderElectionRecord(ctx, fakeClient, lock, namespace, name)
				Expect(lock).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("failed to unmarshal leader election record")))
			})

			It("should successfully return the leader election record", func() {
				Expect(fakeClient.Create(ctx, &corev1.ConfigMap{ObjectMeta: objectMetaValid})).To(Succeed())

				lock, err := ReadLeaderElectionRecord(ctx, fakeClient, lock, namespace, name)
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
				fakeClient = fakeclient.NewClientBuilder().
					WithScheme(kubernetesscheme.Scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
							return fakeErr
						},
					}).
					Build()

				lock, err := ReadLeaderElectionRecord(ctx, fakeClient, lock, namespace, name)
				Expect(lock).To(BeNil())
				Expect(err).To(MatchError(fakeErr))
			})

			It("should successfully return the leader election record", func() {
				lease := &coordinationv1.Lease{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
					},
					Spec: leaseSpecValid,
				}

				Expect(fakeClient.Create(ctx, lease)).To(Succeed())

				lock, err := ReadLeaderElectionRecord(ctx, fakeClient, lock, namespace, name)
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

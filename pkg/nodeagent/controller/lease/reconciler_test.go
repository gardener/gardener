// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package lease_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/nodeagent/controller/lease"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("Reconciler", func() {
	Describe("#ObjectName", func() {
		It("should return the expected name", func() {
			Expect(gardenerutils.NodeAgentLeaseName("foo")).To(Equal("gardener-node-agent-foo"))
		})
	})

	Describe("#Reconcile", func() {
		const nodeName = "foo"

		var (
			ctx        = context.Background()
			c          client.Client
			node       *corev1.Node
			fakeClock  *testclock.FakeClock
			reconciler *lease.Reconciler
			request    reconcile.Request
			leaseKey   client.ObjectKey
		)

		BeforeEach(func() {
			node = &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName, UID: types.UID("node-uid")}}
			c = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.SeedScheme).
				WithObjects(node).
				Build()
			fakeClock = testclock.NewFakeClock(metav1.Now().Time)

			reconciler = &lease.Reconciler{
				Client:               c,
				APIReader:            c,
				LeaseDurationSeconds: 40,
				Namespace:            metav1.NamespaceSystem,
				Clock:                fakeClock,
			}

			request = reconcile.Request{NamespacedName: client.ObjectKey{Name: nodeName}}
			leaseKey = client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: gardenerutils.NodeAgentLeaseName(nodeName)}
		})

		It("should create the heartbeat lease with the expected fields and owner reference", func() {
			result, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(10 * time.Second)) // 40s / 4 = 10s

			l := &coordinationv1.Lease{}
			Expect(c.Get(ctx, leaseKey, l)).To(Succeed())
			Expect(l.Spec.HolderIdentity).To(PointTo(Equal(l.Name)))
			Expect(l.Spec.LeaseDurationSeconds).To(PointTo(Equal(int32(40))))
			Expect(l.Spec.RenewTime).NotTo(BeNil())
			Expect(l.OwnerReferences).To(ConsistOf(metav1.OwnerReference{
				APIVersion:         "v1",
				Kind:               "Node",
				Name:               nodeName,
				UID:                types.UID("node-uid"),
				Controller:         new(true),
				BlockOwnerDeletion: new(true),
			}))
		})

		It("should renew the lease on a subsequent reconcile", func() {
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			before := &coordinationv1.Lease{}
			Expect(c.Get(ctx, leaseKey, before)).To(Succeed())

			fakeClock.Step(10 * time.Second)

			_, err = reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			after := &coordinationv1.Lease{}
			Expect(c.Get(ctx, leaseKey, after)).To(Succeed())
			Expect(after.Spec.RenewTime.Time).To(BeTemporally(">", before.Spec.RenewTime.Time))
		})
	})
})

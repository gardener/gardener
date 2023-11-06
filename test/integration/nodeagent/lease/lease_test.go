// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package lease_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/clock/testing"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	leasecontroller "github.com/gardener/gardener/pkg/nodeagent/controller/lease"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Reconcile", func() {
	Describe("Lease controller tests", func() {
		var (
			node      *corev1.Node
			nodeName  string
			fakeClock *testing.FakeClock
		)

		BeforeEach(func() {
			nodeName = "test-" + gardenerutils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]
			fakeClock = testing.NewFakeClock(time.Now().UTC())

			By("Setup manager")
			mgr, err := manager.New(restConfig, manager.Options{
				Metrics: metricsserver.Options{BindAddress: "0"},
				Cache: cache.Options{
					DefaultNamespaces: map[string]cache.Config{testNamespace.Name: {}},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			By("Register controller")
			leaseReconciler := &leasecontroller.Reconciler{
				Clock:                fakeClock,
				NodeName:             nodeName,
				Namespace:            testNamespace.Name,
				RenewIntervalSeconds: 3,
			}
			Expect(leaseReconciler.AddToManager(mgr)).To(Succeed())

			By("Start manager")
			mgrContext, mgrCancel := context.WithCancel(ctx)

			go func() {
				defer GinkgoRecover()
				Expect(mgr.Start(mgrContext)).To(Succeed())
			}()

			DeferCleanup(func() {
				By("Stop manager")
				mgrCancel()
			})

			node = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   nodeName,
					Labels: map[string]string{testID: testRunID},
				},
			}

			By("Create Node")
			Expect(testClient.Create(ctx, node)).To(Succeed())
			DeferCleanup(func() {
				By("Delete Node")
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, node))).To(Succeed())
				By("Delete Lease")
				lease := &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace.Name, Name: "gardener-node-agent-" + nodeName}}
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, lease))).To(Succeed())
			})
		})

		It("should create Lease", func() {
			lease := &coordinationv1.Lease{}
			Eventually(func() error {
				return testClient.Get(ctx, types.NamespacedName{Namespace: testNamespace.Name, Name: "gardener-node-agent-" + nodeName}, lease)
			}).Should(Succeed())
			validateOwnerReference(lease, node)
		})

		It("should update the Lease time", func() {
			lease := &coordinationv1.Lease{}
			Eventually(func() error {
				return testClient.Get(ctx, types.NamespacedName{Namespace: testNamespace.Name, Name: "gardener-node-agent-" + nodeName}, lease)
			}).Should(Succeed())
			validateOwnerReference(lease, node)

			oldRenewTime := lease.Spec.RenewTime
			// wait a nit more than RenewIntervalSeconds
			fakeClock.Step(5 * time.Second)

			Eventually(func() bool {
				Expect(testClient.Get(ctx, types.NamespacedName{Namespace: testNamespace.Name, Name: "gardener-node-agent-" + nodeName}, lease)).To(Succeed())
				return lease.Spec.RenewTime.After(oldRenewTime.Time)
			}).Should(BeTrue())
			validateOwnerReference(lease, node)
		})

		It("should not update the Lease time if no node is present", func() {
			lease := &coordinationv1.Lease{}
			Eventually(func() error {
				return testClient.Get(ctx, types.NamespacedName{Namespace: testNamespace.Name, Name: "gardener-node-agent-" + nodeName}, lease)
			}).Should(Succeed())
			validateOwnerReference(lease, node)

			oldRenewTime := lease.Spec.RenewTime
			fakeClock.Step(1 * time.Minute)

			Expect(testClient.Delete(ctx, node)).To(Succeed())
			Eventually(func() error {
				return testClient.Get(ctx, types.NamespacedName{Name: nodeName}, node)
			}).Should(BeNotFoundError())

			// In a real world scenario the lease would be deleted by garbage collection because of its owner reference.
			Consistently(func() bool {
				Expect(testClient.Get(ctx, types.NamespacedName{Namespace: testNamespace.Name, Name: "gardener-node-agent-" + nodeName}, lease)).To(Succeed())
				return lease.Spec.RenewTime.Equal(oldRenewTime)
			}).Should(BeTrue())
			validateOwnerReference(lease, node)
		})
	})
})

func validateOwnerReference(lease *coordinationv1.Lease, node *corev1.Node) {
	ExpectWithOffset(1, lease.OwnerReferences).To(ConsistOf(metav1.OwnerReference{
		APIVersion:         "v1",
		Kind:               "Node",
		Name:               node.GetName(),
		UID:                node.GetUID(),
		BlockOwnerDeletion: pointer.Bool(true),
		Controller:         pointer.Bool(true),
	}))
}

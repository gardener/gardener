// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package node_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	nodecontroller "github.com/gardener/gardener/pkg/nodeagent/controller/node"
	"github.com/gardener/gardener/pkg/nodeagent/dbus/fake"
)

var _ = Describe("Node controller tests", func() {
	var (
		fakeDBus *fake.DBus
		nodeName = testRunID
		node     *corev1.Node
	)

	BeforeEach(func() {
		By("Setup manager")
		mgr, err := manager.New(restConfig, manager.Options{
			Metrics: metricsserver.Options{BindAddress: "0"},
			Cache: cache.Options{
				DefaultLabelSelector: labels.SelectorFromSet(labels.Set{testID: testRunID}),
			},
		})
		Expect(err).NotTo(HaveOccurred())

		By("Register controller")
		fakeDBus = fake.New()
		Expect((&nodecontroller.Reconciler{
			DBus: fakeDBus,
		}).AddToManager(mgr)).To(Succeed())

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
			Expect(testClient.Delete(ctx, node)).To(Succeed())
		})
	})

	It("should do nothing because node has no restart annotation", func() {
		Consistently(func() []fake.SystemdAction {
			return fakeDBus.Actions
		}).Should(BeEmpty())
	})

	It("should restart the systemd services specified in the restart annotation", func() {
		By("Adding restart annotation to node")
		svc1, svc2, svc3 := "gardener-node-agent", "foo", "bar"
		metav1.SetMetaDataAnnotation(&node.ObjectMeta, "worker.gardener.cloud/restart-systemd-services", svc1+","+svc2+","+svc3)
		Expect(testClient.Update(ctx, node)).To(Succeed())

		By("Wait for restart annotation to disappear")
		Eventually(func(g Gomega) map[string]string {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
			return node.Annotations
		}).ShouldNot(HaveKey("worker.gardener.cloud/restart-systemd-services"))

		By("Assert that the systemd services were restarted")
		Eventually(func(g Gomega) {
			g.Expect(fakeDBus.Actions).To(HaveLen(3))
			g.Expect(fakeDBus.Actions[0].Action).To(Equal(fake.ActionRestart))
			g.Expect(fakeDBus.Actions[0].UnitNames).To(ConsistOf(svc2))
			g.Expect(fakeDBus.Actions[1].Action).To(Equal(fake.ActionRestart))
			g.Expect(fakeDBus.Actions[1].UnitNames).To(ConsistOf(svc3))
			g.Expect(fakeDBus.Actions[2].Action).To(Equal(fake.ActionRestart))
			g.Expect(fakeDBus.Actions[2].UnitNames).To(ConsistOf(svc1 + ".service"))
		}).Should(Succeed())
	})
})

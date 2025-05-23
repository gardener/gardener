// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package node_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

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
			Controller: controllerconfig.Controller{
				SkipNameValidation: ptr.To(true),
			},
		})
		Expect(err).NotTo(HaveOccurred())

		By("Register controller")
		fakeDBus = fake.New()
		Expect((&nodecontroller.Reconciler{
			DBus: fakeDBus,
		}).AddToManager(mgr, predicate.NewPredicateFuncs(func(client.Object) bool { return true }))).To(Succeed())

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
		svc1, svc2, svc3 := "gardener-node-agent", "foo.service", "bar"
		metav1.SetMetaDataAnnotation(&node.ObjectMeta, "worker.gardener.cloud/restart-systemd-services", svc1+","+svc2+","+svc3)
		Expect(testClient.Update(ctx, node)).To(Succeed())

		By("Wait for restart annotation to disappear")
		Eventually(func(g Gomega) map[string]string {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
			return node.Annotations
		}).ShouldNot(HaveKey("worker.gardener.cloud/restart-systemd-services"))

		By("Assert that the systemd services were restarted")
		Eventually(func() []fake.SystemdAction {
			return fakeDBus.Actions
		}).Should(ConsistOf(
			fake.SystemdAction{Action: fake.ActionRestart, UnitNames: []string{svc2}},
			fake.SystemdAction{Action: fake.ActionRestart, UnitNames: []string{svc3 + ".service"}},
			fake.SystemdAction{Action: fake.ActionRestart, UnitNames: []string{svc1 + ".service"}},
		))
	})
})

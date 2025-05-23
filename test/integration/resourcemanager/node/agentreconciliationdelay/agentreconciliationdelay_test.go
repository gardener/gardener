// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package agentreconciliationdelay_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("AgentReconciliationDelay tests", func() {
	When("#nodes = 1", func() {
		BeforeEach(func() {
			prepareNodes(1)

			By("Wait until manager has observed all nodes")
			Eventually(func(g Gomega) int {
				nodeList := &corev1.NodeList{}
				g.Expect(mgrClient.List(ctx, nodeList)).To(Succeed())
				return len(nodeList.Items)
			}).Should(Equal(1))
		})

		It("should assign the minimum delay", func() {
			Eventually(func(g Gomega) {
				nodeList := &corev1.NodeList{}
				g.Expect(testClient.List(ctx, nodeList)).To(Succeed())

				g.Expect(nodeList.Items[0].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("5s"))
			}).Should(Succeed())
		})
	})

	When("1 < #nodes <= max-delay-seconds", func() {
		BeforeEach(func() {
			prepareNodes(10)

			By("Wait until manager has observed all nodes")
			Eventually(func(g Gomega) int {
				nodeList := &corev1.NodeList{}
				g.Expect(mgrClient.List(ctx, nodeList)).To(Succeed())
				return len(nodeList.Items)
			}).Should(Equal(10))
		})

		It("should assign delays based on linear mapping approach", func() {
			Eventually(func(g Gomega) {
				nodeList := &corev1.NodeList{}
				g.Expect(testClient.List(ctx, nodeList)).To(Succeed())

				g.Expect(nodeList.Items[0].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("5s"))
				g.Expect(nodeList.Items[1].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("7.5s"))
				g.Expect(nodeList.Items[2].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("10s"))
				g.Expect(nodeList.Items[3].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("12.5s"))
				g.Expect(nodeList.Items[4].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("15s"))
				g.Expect(nodeList.Items[5].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("17.5s"))
				g.Expect(nodeList.Items[6].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("20s"))
				g.Expect(nodeList.Items[7].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("22.5s"))
				g.Expect(nodeList.Items[8].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("25s"))
				g.Expect(nodeList.Items[9].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("27.5s"))
			}).Should(Succeed())
		})
	})

	When("#nodes > max-delay-seconds", func() {
		BeforeEach(func() {
			prepareNodes(31)

			By("Wait until manager has observed all nodes")
			Eventually(func(g Gomega) int {
				nodeList := &corev1.NodeList{}
				g.Expect(mgrClient.List(ctx, nodeList)).To(Succeed())
				return len(nodeList.Items)
			}).Should(Equal(31))
		})

		It("should assign delays based on linear mapping approach", func() {
			Eventually(func(g Gomega) {
				nodeList := &corev1.NodeList{}
				g.Expect(testClient.List(ctx, nodeList)).To(Succeed())

				g.Expect(nodeList.Items[0].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("5s"))
				g.Expect(nodeList.Items[1].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("5.806451612s"))
				g.Expect(nodeList.Items[2].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("6.612903225s"))
				g.Expect(nodeList.Items[3].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("7.419354838s"))
				g.Expect(nodeList.Items[4].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("8.225806451s"))
				g.Expect(nodeList.Items[5].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("9.032258064s"))
				g.Expect(nodeList.Items[6].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("9.838709677s"))
				g.Expect(nodeList.Items[7].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("10.64516129s"))
				g.Expect(nodeList.Items[8].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("11.451612903s"))
				g.Expect(nodeList.Items[9].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("12.258064516s"))
				g.Expect(nodeList.Items[10].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("13.064516129s"))
				g.Expect(nodeList.Items[11].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("13.870967741s"))
				g.Expect(nodeList.Items[12].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("14.677419354s"))
				g.Expect(nodeList.Items[13].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("15.483870967s"))
				g.Expect(nodeList.Items[14].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("16.29032258s"))
				g.Expect(nodeList.Items[15].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("17.096774193s"))
				g.Expect(nodeList.Items[16].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("17.903225806s"))
				g.Expect(nodeList.Items[17].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("18.709677419s"))
				g.Expect(nodeList.Items[18].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("19.516129032s"))
				g.Expect(nodeList.Items[19].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("20.322580645s"))
				g.Expect(nodeList.Items[20].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("21.129032258s"))
				g.Expect(nodeList.Items[21].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("21.93548387s"))
				g.Expect(nodeList.Items[22].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("22.741935483s"))
				g.Expect(nodeList.Items[23].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("23.548387096s"))
				g.Expect(nodeList.Items[24].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("24.354838709s"))
				g.Expect(nodeList.Items[25].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("25.161290322s"))
				g.Expect(nodeList.Items[26].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("25.967741935s"))
				g.Expect(nodeList.Items[27].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("26.774193548s"))
				g.Expect(nodeList.Items[28].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("27.580645161s"))
				g.Expect(nodeList.Items[29].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("28.387096774s"))
			}).Should(Succeed())
		})
	})
})

func prepareNodes(count int) {
	for i := 0; i < count; i++ {
		node := newNode()

		ExpectWithOffset(1, testClient.Create(ctx, node)).To(Succeed(), "node "+node.Name)
		By("Created Node " + node.Name + " for test")

		DeferCleanup(func() {
			ExpectWithOffset(1, client.IgnoreNotFound(testClient.Delete(ctx, node))).To(Succeed(), "node "+node.Name)
			By("Deleted Node " + node.Name)
		})
	}
}

func newNode() *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "node-",
			Labels:       map[string]string{testID: testRunID},
		},
	}
}

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package agentreconciliationdelay_test

import (
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("AgentReconciliationDelay tests", func() {
	When("#nodes = 1", func() {
		BeforeEach(func() {
			prepareNodes(1, false)

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
			prepareNodes(10, false)

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
			prepareNodes(31, false)

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

	Context("ignore nodes with serial operating system config rollout", func() {
		BeforeEach(func() {
			prepareNodes(5, true)   // first group of nodes should be excluded
			prepareNodes(10, false) // second group of nodes should be considered
			prepareNodes(5, true)   // third group of nodes should be excluded

			By("Wait until manager has observed all objects")
			Eventually(func(g Gomega) {
				secretList := &corev1.SecretList{}
				g.Expect(mgrClient.List(ctx, secretList)).To(Succeed())
				g.Expect(secretList.Items).To(HaveLen(2))

				nodeList := &corev1.NodeList{}
				g.Expect(mgrClient.List(ctx, nodeList)).To(Succeed())
				g.Expect(nodeList.Items).To(HaveLen(20))
			}).Should(Succeed())
		})

		It("should not assign any delay for the nodes with serial rollout", func() {
			Eventually(func(g Gomega) {
				nodeList := &corev1.NodeList{}
				g.Expect(testClient.List(ctx, nodeList)).To(Succeed())

				g.Expect(nodeList.Items[5].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("5s"))
				g.Expect(nodeList.Items[6].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("7.5s"))
				g.Expect(nodeList.Items[7].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("10s"))
				g.Expect(nodeList.Items[8].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("12.5s"))
				g.Expect(nodeList.Items[9].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("15s"))
				g.Expect(nodeList.Items[10].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("17.5s"))
				g.Expect(nodeList.Items[11].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("20s"))
				g.Expect(nodeList.Items[12].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("22.5s"))
				g.Expect(nodeList.Items[13].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("25s"))
				g.Expect(nodeList.Items[14].Annotations["node-agent.gardener.cloud/reconciliation-delay"]).To(Equal("27.5s"))
			}).Should(Succeed())

			Consistently(func(g Gomega) {
				nodeList := &corev1.NodeList{}
				g.Expect(testClient.List(ctx, nodeList)).To(Succeed())

				g.Expect(nodeList.Items[0].Annotations).NotTo(HaveKey("node-agent.gardener.cloud/reconciliation-delay"))
				g.Expect(nodeList.Items[1].Annotations).NotTo(HaveKey("node-agent.gardener.cloud/reconciliation-delay"))
				g.Expect(nodeList.Items[2].Annotations).NotTo(HaveKey("node-agent.gardener.cloud/reconciliation-delay"))
				g.Expect(nodeList.Items[3].Annotations).NotTo(HaveKey("node-agent.gardener.cloud/reconciliation-delay"))
				g.Expect(nodeList.Items[4].Annotations).NotTo(HaveKey("node-agent.gardener.cloud/reconciliation-delay"))
				g.Expect(nodeList.Items[15].Annotations).NotTo(HaveKey("node-agent.gardener.cloud/reconciliation-delay"))
				g.Expect(nodeList.Items[16].Annotations).NotTo(HaveKey("node-agent.gardener.cloud/reconciliation-delay"))
				g.Expect(nodeList.Items[17].Annotations).NotTo(HaveKey("node-agent.gardener.cloud/reconciliation-delay"))
				g.Expect(nodeList.Items[18].Annotations).NotTo(HaveKey("node-agent.gardener.cloud/reconciliation-delay"))
				g.Expect(nodeList.Items[19].Annotations).NotTo(HaveKey("node-agent.gardener.cloud/reconciliation-delay"))
			}).Should(Succeed())
		})
	})
})

func prepareNodes(count int, withSerialOSCReconciliation bool) {
	GinkgoHelper()

	var gardenerNodeAgentSecret *corev1.Secret
	if withSerialOSCReconciliation {
		gardenerNodeAgentSecret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			GenerateName: "gardener-node-agent-",
			Namespace:    "kube-system",
			Annotations:  map[string]string{"reconciliation.osc.node-agent.gardener.cloud/serial": "true"},
			Labels:       map[string]string{"gardener.cloud/role": "operating-system-config"},
		}}

		Expect(testClient.Create(ctx, gardenerNodeAgentSecret)).To(Succeed())
		By("Created gardener-node-agent Secret " + gardenerNodeAgentSecret.Name + " with serial OSC reconciliation for test")

		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, gardenerNodeAgentSecret))).To(Succeed())
			By("Deleted gardener-node-agent Secret " + gardenerNodeAgentSecret.Name + " with serial OSC reconciliation")
		})
	}

	for suffix := range count {
		node := newNode(suffix, gardenerNodeAgentSecret)

		Expect(testClient.Create(ctx, node)).To(Succeed(), "node "+node.Name)
		By("Created Node " + node.Name + " for test")

		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, node))).To(Succeed(), "node "+node.Name)
			By("Deleted Node " + node.Name)
		})
	}

	nodeGroup++
}

var nodeGroup int

func newNode(suffix int, gardenerNodeAgentSecret *corev1.Secret) *corev1.Node {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-" + strconv.Itoa(nodeGroup) + "-" + strconv.Itoa(suffix),
			Labels: map[string]string{testID: testRunID},
		},
	}

	if gardenerNodeAgentSecret != nil {
		metav1.SetMetaDataLabel(&node.ObjectMeta, "worker.gardener.cloud/gardener-node-agent-secret-name", gardenerNodeAgentSecret.Name)
	}

	return node
}

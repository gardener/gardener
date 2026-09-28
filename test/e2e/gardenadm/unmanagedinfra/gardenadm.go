// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package unmanagedinfra

import (
	"fmt"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/gardener/gardener/test/e2e/gardenadm"
	e2egardener "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/utils/access"
	shootoperation "github.com/gardener/gardener/test/utils/shoots/operation"
)

var _ = Describe("gardenadm unmanaged infrastructure scenario tests", Label("gardenadm", "unmanaged-infra"), func() {
	var (
		shootNamespace        = "garden"
		shootName             = "root"
		shoot                 = &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace}}
		controlPlaneNamespace = "kube-system"
	)

	Describe("Single-node control plane", Ordered, Label("single"), func() {
		var (
			configDirectory = "/gardenadm/resources"

			shootClientSet  kubernetes.Interface
			gardenClientSet kubernetes.Interface
		)

		Context("gardenadm init + join", Ordered, Label("initjoin"), func() {
			gardenadm.ItShouldCreateShootClient(&shootClientSet)

			It("should be able to communicate with the API server and see the node and the control plane pods", func(ctx SpecContext) {
				Eventually(ctx, func(g Gomega) []corev1.Node {
					nodeList := &corev1.NodeList{}
					g.Expect(shootClientSet.Client().List(ctx, nodeList)).To(Succeed())
					return nodeList.Items
				}).Should(HaveLen(1))

				Eventually(ctx, func(g Gomega) []corev1.Pod {
					podList := &corev1.PodList{}
					g.Expect(shootClientSet.Client().List(ctx, podList, client.InNamespace(controlPlaneNamespace))).To(Succeed())
					return podList.Items
				}).Should(ContainElements(
					MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("etcd-events-gind-machine-0")})}),
					MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("etcd-main-gind-machine-0")})}),
					MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("kube-apiserver-gind-machine-0")})}),
					MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("kube-controller-manager-gind-machine-0")})}),
					MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("kube-scheduler-gind-machine-0")})}),
					MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": HavePrefix("kube-proxy")})}),
					MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": HavePrefix("gardener-resource-manager")})}),
					MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": HavePrefix("calico")})}),
					MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": HavePrefix("coredns")})}),
					MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": HavePrefix("local-path-provisioner")})}),
				))
			}, SpecTimeout(time.Minute))

			It("should ensure the control plane namespace is properly labeled", func(ctx SpecContext) {
				Eventually(ctx, func(g Gomega) map[string]string {
					namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: controlPlaneNamespace}}
					g.Expect(shootClientSet.Client().Get(ctx, client.ObjectKeyFromObject(namespace), namespace)).To(Succeed())
					return namespace.Labels
				}).Should(HaveKeyWithValue("gardener.cloud/role", "shoot"))
			}, SpecTimeout(time.Minute))

			It("should ensure extensions and gardener-resource-manager run in pod network", func(ctx SpecContext) {
				By("Check extensions")
				Eventually(ctx, func(g Gomega) {
					namespaceList := &corev1.NamespaceList{}
					g.Expect(shootClientSet.Client().List(ctx, namespaceList, client.MatchingLabels{"gardener.cloud/role": "extension"})).To(Succeed())

					for _, namespace := range namespaceList.Items {
						podList := &corev1.PodList{}
						g.Expect(shootClientSet.Client().List(ctx, podList, client.InNamespace(namespace.Name))).To(Succeed())

						for _, pod := range podList.Items {
							g.Expect(pod.Spec.HostNetwork).To(BeFalse(), "pod %s", client.ObjectKeyFromObject(&pod))
						}
					}
				}).Should(Succeed())

				By("Check gardener-resource-manager")
				Eventually(ctx, func(g Gomega) {
					podList := &corev1.PodList{}
					g.Expect(shootClientSet.Client().List(ctx, podList, client.InNamespace(controlPlaneNamespace), client.MatchingLabels{"app": "gardener-resource-manager"})).To(Succeed())

					for _, pod := range podList.Items {
						g.Expect(pod.Spec.HostNetwork).To(BeFalse(), "pod %s", client.ObjectKeyFromObject(&pod))
					}
				}).Should(Succeed())
			}, SpecTimeout(time.Minute))

			It("should ensure gardener-node-agent is running", func(ctx SpecContext) {
				Eventually(ctx, func(g Gomega) *gbytes.Buffer {
					stdOut, _, err := RunInMachine(ctx, 0, "systemctl", "status", "gardener-node-agent")
					g.Expect(err).NotTo(HaveOccurred())
					return stdOut
				}).Should(gbytes.Say(`Active: active \(running\)`))
			}, SpecTimeout(time.Minute))

			It("should ensure that extension webhooks on control plane components are functioning", func(ctx SpecContext) {
				Eventually(ctx, func(g Gomega) map[string]string {
					pod := &corev1.Pod{}
					g.Expect(shootClientSet.Client().Get(ctx, client.ObjectKey{Name: "kube-scheduler-gind-machine-0", Namespace: controlPlaneNamespace}, pod)).To(Succeed())
					return pod.Labels
				}).Should(HaveKeyWithValue("injected-by", "provider-local"))
			}, SpecTimeout(time.Minute))

			It("should ensure that the config dir location has been stored in the well-known location", func(ctx SpecContext) {
				Eventually(ctx, func(g Gomega) string {
					stdOut, _, err := RunInMachine(ctx, 0, "cat", "/var/lib/gardenadm/config-directory")
					g.Expect(err).NotTo(HaveOccurred())
					return string(stdOut.Contents())
				}).Should(Equal(configDirectory))
			}, SpecTimeout(time.Minute))

			itShouldJoinNode()

			itShouldSeeJoinedNodeAndCheckHealth(func() kubernetes.Interface { return shootClientSet })
		})

		Context("gardenadm reset + join", Ordered, Label("resetjoin"), func() {
			itShouldResetNode()

			It("should no longer see the node in the cluster", func(ctx SpecContext) {
				Eventually(ctx, func(g Gomega) {
					node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: machineContainerName(1)}}
					g.Expect(shootClientSet.Client().Get(ctx, client.ObjectKeyFromObject(node), node)).Should(HaveOccurred())
				}).Should(Succeed())
			}, SpecTimeout(time.Minute))

			itShouldJoinNode()

			itShouldSeeJoinedNodeAndCheckHealth(func() kubernetes.Interface { return shootClientSet })
		})

		Context("gardenadm connect", Ordered, Label("connect"), func() {
			var (
				gardenClusterKubeconfigPathOnMachine = "/tmp/virtual-garden-kubeconfig"

				clusterAdminStaticToken string
			)

			gardenadm.ItShouldCreateShootClient(&shootClientSet)
			gardenadm.ItShouldCreateGardenClient(&gardenClientSet)

			It("should store the cluster-admin static token for later assertions", func(ctx SpecContext) {
				secret, err := kubernetesutils.NewestObject(ctx, shootClientSet.Client(), &corev1.SecretList{}, nil, client.InNamespace(controlPlaneNamespace), client.MatchingLabels{
					"managed-by": "secrets-manager",
					"name":       "kube-apiserver-static-token",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(secret).NotTo(BeNil())

				staticToken, err := secretsutils.LoadStaticTokenFromCSV("kube-apiserver-static-token", secret.(*corev1.Secret).Data[secretsutils.DataKeyStaticTokenCSV])
				Expect(err).NotTo(HaveOccurred())
				token, err := staticToken.GetTokenForUsername("system:cluster-admin")
				Expect(err).NotTo(HaveOccurred())
				clusterAdminStaticToken = token.Token
			}, SpecTimeout(time.Minute))

			It("should copy the garden cluster kubeconfig to the machine pod", func(ctx SpecContext) {
				// In the test setup via Skaffold, we build the 'gardenadm' binary and copy it to the machine pods. Hence,
				// the binary is not available on the host without further ado. For simplicity, we copy the garden cluster
				// kubeconfig from the host into the machine pod here. This enables us to execute
				// 'gardenadm token create --print-connect-command' from the machine pod.
				By("Copy local garden cluster kubeconfig to file in machine pod")
				gardenClusterKubeconfig, err := os.ReadFile(gardenadm.GardenClusterKubeconfigPathOnHost) // #nosec: G304 -- variable points to a static file path
				Expect(err).NotTo(HaveOccurred())

				Eventually(ctx, func() error {
					_, _, err := RunInMachine(ctx, 0, "sh", "-c", fmt.Sprintf("echo '%s' > %s", string(gardenClusterKubeconfig), gardenClusterKubeconfigPathOnMachine))
					return err
				}).Should(Succeed())
			}, SpecTimeout(time.Minute))

			It("should generate a bootstrap token and connect the self-hosted shoot to Gardener", func(ctx SpecContext) {
				stdOut, _, err := RunInMachine(ctx, 0, "sh", "-c", fmt.Sprintf("KUBECONFIG=%s gardenadm token create --print-connect-command --shoot-namespace=%s --shoot-name=%s", gardenClusterKubeconfigPathOnMachine, shootNamespace, shootName))
				Expect(err).NotTo(HaveOccurred())
				connectCommand := strings.Split(strings.ReplaceAll(string(stdOut.Contents()), `"`, ``), " ")

				stdOut, _, err = RunInMachine(ctx, 0, append(connectCommand, "--log-level=debug")...)
				Expect(err).NotTo(HaveOccurred())

				Eventually(ctx, stdOut).Should(gbytes.Say("Your self-hosted shoot cluster has successfully been connected to Gardener!"))
			}, SpecTimeout(time.Minute))

			gardenadm.ItShouldConnectSuccessfully(&gardenClientSet, shoot, RunInMachine)
			gardenadm.ItShouldBeReconciledByGardenlet(&gardenClientSet, &shootClientSet, shoot, clusterAdminStaticToken, RunInMachine, RunInNode)
		})

		Context("self-hosted shoot -> seed promotion", Ordered, Label("seed"), func() {
			gardenadm.ItShouldCreateShootClient(&shootClientSet)
			gardenadm.ItShouldCreateGardenClient(&gardenClientSet)

			It("should ensure the ManagedSeed is healthy", func(ctx SpecContext) {
				managedSeed := &seedmanagementv1alpha1.ManagedSeed{ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace}}
				Eventually(ctx, func(g Gomega) {
					g.Expect(gardenClientSet.Client().Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)).To(Succeed())
					g.Expect(health.CheckManagedSeed(managedSeed)).To(Succeed())
				}).Should(Succeed())
			}, SpecTimeout(5*time.Minute))

			It("should ensure the Seed is healthy", func(ctx SpecContext) {
				seed := &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: shootName}}
				Eventually(ctx, func(g Gomega) {
					g.Expect(gardenClientSet.Client().Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
					g.Expect(health.CheckSeed(seed, seed.Status.Gardener)).To(Succeed())
				}).Should(Succeed())
			}, SpecTimeout(5*time.Minute))

			It("should ensure the seed gardenlet runs in the garden namespace", func(ctx SpecContext) {
				Eventually(ctx, func(g Gomega) []corev1.Pod {
					podList := &corev1.PodList{}
					g.Expect(shootClientSet.Client().List(ctx, podList, client.InNamespace("garden"), client.MatchingLabels{"app": "gardener", "role": "gardenlet"})).To(Succeed())
					return podList.Items
				}).ShouldNot(BeEmpty())
			}, SpecTimeout(2*time.Minute))
		})

		Context("hosted shoot on promoted seed", Ordered, Label("hosted-shoot"), func() {
			var s *e2egardener.ShootContext

			gardenadm.ItShouldCreateGardenClient(&gardenClientSet)

			It("should setup the test context", func() {
				hostedShoot := e2egardener.DefaultShoot("e2e-gardenadm")
				s = (&e2egardener.TestContext{
					GardenClientSet: gardenClientSet,
					GardenClient:    gardenClientSet.Client(),
					GardenKomega:    New(gardenClientSet.Client()),
				}).ForShoot(hostedShoot)
			})

			It("should create the hosted shoot", func(ctx SpecContext) {
				Eventually(ctx, func() error {
					return s.GardenClient.Create(ctx, s.Shoot)
				}).Should(Succeed())
			}, SpecTimeout(time.Minute))

			It("should wait for the hosted shoot to be reconciled and healthy", func(ctx SpecContext) {
				Eventually(ctx, func(g Gomega) bool {
					g.Expect(s.GardenKomega.Get(s.Shoot)()).To(Succeed())
					completed, _ := shootoperation.ReconciliationSuccessful(s.Shoot)
					return completed
				}).WithPolling(30 * time.Second).Should(BeTrue())
			}, SpecTimeout(30*time.Minute))

			It("should initialize the shoot client", func(ctx SpecContext) {
				Eventually(ctx, func() error {
					clientSet, err := access.CreateShootClientFromAdminKubeconfig(ctx, s.GardenClientSet, s.Shoot)
					if err != nil {
						return err
					}
					s.WithShootClientSet(clientSet)
					return nil
				}).Should(Succeed())
			}, SpecTimeout(time.Minute))

			It("should verify shoot access using admin kubeconfig", func(ctx SpecContext) {
				Eventually(ctx, s.ShootKomega.List(&corev1.NamespaceList{})).Should(Succeed())
			}, SpecTimeout(time.Minute))

			It("should delete the hosted shoot", func(ctx SpecContext) {
				Eventually(ctx, func(g Gomega) {
					g.Expect(gardenerutils.ConfirmDeletion(ctx, s.GardenClient, s.Shoot)).To(Succeed())
					g.Expect(s.GardenClient.Delete(ctx, s.Shoot)).To(Succeed())
				}).Should(Succeed())
			}, SpecTimeout(time.Minute))

			It("should wait for the hosted shoot to be deleted", func(ctx SpecContext) {
				Eventually(ctx, func() error {
					return s.GardenKomega.Get(s.Shoot)()
				}).WithPolling(30 * time.Second).Should(BeNotFoundError())
			}, SpecTimeout(20*time.Minute))
		})
	})
})

func itShouldJoinNode() {
	GinkgoHelper()

	itShouldResetOrJoinNode("join", "Your node has successfully joined the cluster as a worker!")
}

func itShouldResetNode() {
	GinkgoHelper()

	itShouldResetOrJoinNode("reset", "The node has been successfully removed from the cluster!")
}

func itShouldResetOrJoinNode(command, message string) {
	GinkgoHelper()

	It("should generate a token and "+command+" the worker node", func(ctx SpecContext) {
		stdOut, _, err := RunInMachine(ctx, 0, "gardenadm", "token", "create", "--print-"+command+"-command")
		Expect(err).NotTo(HaveOccurred())
		gardenadmCommand := strings.Split(strings.ReplaceAll(string(stdOut.Contents()), `"`, ``), " ")

		stdOut, _, err = RunInMachine(ctx, 1, append(gardenadmCommand, "--log-level=debug")...)
		Expect(err).NotTo(HaveOccurred())

		Eventually(ctx, stdOut).Should(gbytes.Say(message))
	}, SpecTimeout(time.Minute))
}

func itShouldSeeJoinedNodeAndCheckHealth(shootClientSet func() kubernetes.Interface) {
	GinkgoHelper()

	It("should see the joined node and observe its readiness", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: machineContainerName(1)}}
			g.Expect(shootClientSet().Client().Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())

			g.Expect(node.Status.Conditions).To(ContainCondition(
				MatchFields(IgnoreExtras, Fields{"Type": Equal(corev1.NodeReady)}),
				MatchFields(IgnoreExtras, Fields{"Status": Equal(corev1.ConditionTrue)}),
			))
			g.Expect(node.Spec.Taints).NotTo(ContainElement(corev1.Taint{
				Key:    v1beta1constants.TaintNodeCriticalComponentsNotReady,
				Effect: corev1.TaintEffectNoSchedule,
			}))
		}).Should(Succeed())
	}, SpecTimeout(2*time.Minute))
}

// WIP

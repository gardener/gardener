// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package hightouch

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/test/e2e/gardenadm/common"
)

var _ = Describe("gardenadm high-touch scenario tests", Label("gardenadm", "high-touch"), func() {
	BeforeEach(OncePerOrdered, func(ctx SpecContext) {
		testRunID := utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]

		By("Ensuring fresh machine pods for test execution")
		statefulSet := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: statefulSetName, Namespace: namespace}}
		Expect(RuntimeClient.Client().Get(ctx, client.ObjectKeyFromObject(statefulSet), statefulSet)).To(Succeed())

		patch := client.MergeFrom(statefulSet.DeepCopy())
		metav1.SetMetaDataAnnotation(&statefulSet.Spec.Template.ObjectMeta, "test-run-id", testRunID)
		Expect(RuntimeClient.Client().Patch(ctx, statefulSet, patch)).To(Succeed())

		Eventually(ctx, func(g Gomega) {
			g.Expect(RuntimeClient.Client().Get(ctx, client.ObjectKeyFromObject(statefulSet), statefulSet)).To(Succeed())
			progressing, _ := health.IsStatefulSetProgressing(statefulSet)
			g.Expect(progressing).To(BeFalse())
			g.Expect(health.CheckStatefulSet(statefulSet)).To(Succeed())
		}).Should(Succeed())
	}, NodeTimeout(2*time.Minute))

	Describe("Single-node control plane", Ordered, Label("single"), func() {
		var (
			portForwardCtx    context.Context
			cancelPortForward context.CancelFunc
			shootClientSet    kubernetes.Interface
		)

		BeforeAll(func() {
			portForwardCtx, cancelPortForward = context.WithCancel(context.Background())
			DeferCleanup(func() { cancelPortForward() })
		})

		It("should initialize as control plane node", func(ctx SpecContext) {
			stdOut, _, err := execute(ctx, 0, "gardenadm", "--log-level=debug", "init", "-d", "/gardenadm/resources")
			Expect(err).NotTo(HaveOccurred())

			Eventually(ctx, stdOut).Should(gbytes.Say("Your Shoot cluster control-plane has initialized successfully!"))
		}, SpecTimeout(5*time.Minute))

		It("copy admin kubeconfig and create client", func(ctx SpecContext) {
			tempDir, err := os.MkdirTemp("", "tmp")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() { Expect(os.RemoveAll(tempDir)).To(Succeed()) })
			adminKubeconfigFile := filepath.Join(tempDir, "admin.conf")

			By("Copy admin kubeconfig to local file")
			localPort := 6443
			Eventually(ctx, func(g Gomega) error {
				stdOut, _, err := execute(ctx, 0, "cat", "/etc/kubernetes/admin.conf")
				g.Expect(err).NotTo(HaveOccurred())

				kubeconfig := strings.ReplaceAll(string(stdOut.Contents()), "api.root.garden.internal.gardenadm.local", fmt.Sprintf("localhost:%d", localPort))
				return os.WriteFile(adminKubeconfigFile, []byte(kubeconfig), 0600)
			}).Should(Succeed())

			By("Forward port to control plane machine pod")
			fw, err := kubernetes.SetupPortForwarder(portForwardCtx, RuntimeClient.RESTConfig(), namespace, machinePodName(0), localPort, 443)
			Expect(err).NotTo(HaveOccurred())

			go func() {
				if err := fw.ForwardPorts(); err != nil {
					Fail("Error forwarding ports: " + err.Error())
				}
			}()

			Eventually(func() chan struct{} { return fw.Ready() }).Should(BeClosed())

			By("Create client set")
			Eventually(func() error {
				shootClientSet, err = kubernetes.NewClientFromFile("", adminKubeconfigFile,
					kubernetes.WithDisabledCachedClient(),
					kubernetes.WithClientOptions(client.Options{Scheme: kubernetes.SeedScheme}),
				)
				return err
			}).Should(Succeed())
		})

		It("should be able to communicate with the API server and see the node and the control plane pods", func(ctx SpecContext) {
			Eventually(ctx, func(g Gomega) []corev1.Node {
				nodeList := &corev1.NodeList{}
				g.Expect(shootClientSet.Client().List(ctx, nodeList)).To(Succeed())
				return nodeList.Items
			}).Should(HaveLen(1))

			Eventually(ctx, func(g Gomega) []corev1.Pod {
				podList := &corev1.PodList{}
				g.Expect(shootClientSet.Client().List(ctx, podList, client.InNamespace("kube-system"))).To(Succeed())
				return podList.Items
			}).Should(ContainElements(
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("etcd-bootstrap-events-machine-0")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("etcd-bootstrap-main-machine-0")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("kube-apiserver-machine-0")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("kube-controller-manager-machine-0")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("kube-scheduler-machine-0")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": HavePrefix("kube-proxy")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": HavePrefix("gardener-resource-manager")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": HavePrefix("calico")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": HavePrefix("coredns")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": HavePrefix("local-path-provisioner")})}),
			))
		}, SpecTimeout(time.Minute))

		It("should ensure the control plane namespace is properly labeled", func(ctx SpecContext) {
			Eventually(ctx, func(g Gomega) map[string]string {
				namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}}
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
				g.Expect(shootClientSet.Client().List(ctx, podList, client.InNamespace("kube-system"), client.MatchingLabels{"app": "gardener-resource-manager"})).To(Succeed())

				for _, pod := range podList.Items {
					g.Expect(pod.Spec.HostNetwork).To(BeFalse(), "pod %s", client.ObjectKeyFromObject(&pod))
				}
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		It("should ensure gardener-node-agent is running", func(ctx SpecContext) {
			Eventually(ctx, func(g Gomega) *gbytes.Buffer {
				stdOut, _, err := execute(ctx, 0, "systemctl", "status", "gardener-node-agent")
				g.Expect(err).NotTo(HaveOccurred())
				return stdOut
			}).Should(gbytes.Say(`Active: active \(running\)`))
		}, SpecTimeout(time.Minute))

		It("should ensure that extension webhooks on control plane components are functioning", func(ctx SpecContext) {
			Eventually(ctx, func(g Gomega) map[string]string {
				pod := &corev1.Pod{}
				g.Expect(shootClientSet.Client().Get(ctx, client.ObjectKey{Name: "kube-scheduler-machine-0", Namespace: "kube-system"}, pod)).To(Succeed())
				return pod.Labels
			}).Should(HaveKeyWithValue("injected-by", "provider-local"))
		}, SpecTimeout(time.Minute))

		It("should generate a bootstrap token and join the worker node", func(ctx SpecContext) {
			stdOut, _, err := execute(ctx, 0, "gardenadm", "token", "create", "--print-join-command")
			Expect(err).NotTo(HaveOccurred())
			joinCommand := strings.Split(strings.ReplaceAll(string(stdOut.Contents()), `"`, ``), " ")

			stdOut, _, err = execute(ctx, 1, append(joinCommand, "--log-level=debug")...)
			Expect(err).NotTo(HaveOccurred())

			Eventually(ctx, stdOut).Should(gbytes.Say("Your node has successfully been instructed to join the cluster as a worker!"))
		}, SpecTimeout(time.Minute))

		It("should see the joined node and observe its readiness", func(ctx SpecContext) {
			Eventually(ctx, func(g Gomega) {
				node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: machinePodName(1)}}
				g.Expect(shootClientSet.Client().Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())

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
	})
})

// nolint:unparam
func execute(ctx context.Context, ordinal int, command ...string) (*gbytes.Buffer, *gbytes.Buffer, error) {
	var stdOutBuffer, stdErrBuffer = gbytes.NewBuffer(), gbytes.NewBuffer()

	return stdOutBuffer, stdErrBuffer, RuntimeClient.PodExecutor().ExecuteWithStreams(
		ctx,
		namespace,
		machinePodName(ordinal),
		ContainerName,
		nil,
		io.MultiWriter(stdOutBuffer, gexec.NewPrefixedWriter("[out] ", GinkgoWriter)),
		io.MultiWriter(stdErrBuffer, gexec.NewPrefixedWriter("[err] ", GinkgoWriter)),
		command...,
	)
}

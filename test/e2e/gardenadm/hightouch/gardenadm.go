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

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
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
	}, NodeTimeout(time.Minute))

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
			stdOut, _, err := execute(ctx, 0,
				"gardenadm", "init", "-d", "/gardenadm/resources",
			)
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

				kubeconfig := strings.ReplaceAll(string(stdOut.Contents()), "localhost", fmt.Sprintf("localhost:%d", localPort))
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
			}).Should(ConsistOf(
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("etcd-events-0-machine-0")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("etcd-main-0-machine-0")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("kube-apiserver-machine-0")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("kube-controller-manager-machine-0")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("kube-scheduler-machine-0")})}),
			))
		}, SpecTimeout(5*time.Second))

		It("should join as worker node", func(ctx SpecContext) {
			_, stdErr, err := execute(ctx, 1,
				"gardenadm", "join",
			)
			Expect(err).NotTo(HaveOccurred())

			Eventually(ctx, stdErr).Should(gbytes.Say("Not implemented either"))
		}, SpecTimeout(time.Minute))
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

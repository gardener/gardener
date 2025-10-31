// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedinfra

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/provider-local/local"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/test/e2e/gardenadm/common"
	shootmigration "github.com/gardener/gardener/test/utils/shoots/migration"
)

var _ = Describe("gardenadm managed infrastructure scenario tests", Label("gardenadm", "managed-infra"), func() {
	BeforeEach(OncePerOrdered, func(SpecContext) {
		PrepareBinary()
	}, NodeTimeout(5*time.Minute))

	Describe("Prepare infrastructure and machines", Ordered, func() {
		const (
			shootName   = "root"
			technicalID = "shoot--garden--" + shootName
			localPort   = 6443
		)

		var (
			session *gexec.Session

			kubeconfigOutputFile string

			portForwardCtx    context.Context
			cancelPortForward context.CancelFunc
		)

		BeforeAll(func() {
			DeferCleanup(test.WithTempFile("", "kubeconfig", nil, &kubeconfigOutputFile))

			portForwardCtx, cancelPortForward = context.WithCancel(context.Background())
			DeferCleanup(func() { cancelPortForward() })
		})

		AfterAll(func(ctx SpecContext) {
			// Ensure that the gardenadm process is stopped at the end of the test, even in case it didn't finish successfully.
			// Interrupting gardenadm should output the current error where it is stuck, if any.
			Eventually(ctx, session.Interrupt()).Should(gexec.Exit())
		}, NodeTimeout(time.Minute))

		It("should start the bootstrap flow", func() {
			// Start the gardenadm process but don't wait for it to complete so that we can asynchronously perform assertions
			// on individual steps in the test specs below.
			session = Run("bootstrap", "-d", "../../../dev-setup/gardenadm/resources/generated/managed-infra", "--kubeconfig-output", kubeconfigOutputFile)
		})

		It("should auto-detect the system's public IPs", func(ctx SpecContext) {
			Eventually(ctx, session.Err).Should(gbytes.Say("Using auto-detected public IP addresses as bastion ingress CIDRs"))
		}, SpecTimeout(time.Minute))

		It("should find the cloud provider secret", func(ctx SpecContext) {
			cloudProviderSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "cloudprovider", Namespace: technicalID}}
			Eventually(ctx, Object(cloudProviderSecret)).Should(HaveField("ObjectMeta.Labels", HaveKeyWithValue("gardener.cloud/purpose", "cloudprovider")))
		}, SpecTimeout(time.Minute))

		It("should deploy gardener-resource-manager", func(ctx SpecContext) {
			deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameGardenerResourceManager, Namespace: technicalID}}
			Eventually(ctx, Object(deployment)).Should(BeHealthy(health.CheckDeployment))
		}, SpecTimeout(time.Minute))

		It("should deploy the provider extension", func(ctx SpecContext) {
			deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "gardener-extension-" + local.Name, Namespace: "extension-" + local.Name}}
			Eventually(ctx, Object(deployment)).Should(BeHealthy(health.CheckDeployment))
		}, SpecTimeout(time.Minute))

		It("should deploy the infrastructure", func(ctx SpecContext) {
			infra := &extensionsv1alpha1.Infrastructure{ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: technicalID}}
			Eventually(ctx, Object(infra)).Should(BeHealthy(health.CheckExtensionObject))
		}, SpecTimeout(time.Minute))

		It("should deploy machine-controller-manager", func(ctx SpecContext) {
			deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameMachineControllerManager, Namespace: technicalID}}
			Eventually(ctx, Object(deployment)).Should(BeHealthy(health.CheckDeployment))
		}, SpecTimeout(time.Minute))

		It("should deploy the worker", func(ctx SpecContext) {
			worker := &extensionsv1alpha1.Worker{ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: technicalID}}
			Eventually(ctx, Object(worker)).Should(BeHealthy(health.CheckExtensionObject))
		}, SpecTimeout(5*time.Minute))

		It("should deploy a control plane machine", func(ctx SpecContext) {
			podList := &corev1.PodList{}
			Eventually(ctx, ObjectList(podList, client.InNamespace(technicalID), client.MatchingLabels{"app": "machine"})).
				Should(HaveField("Items", ConsistOf(HaveField("Status.Phase", corev1.PodRunning))))
		}, SpecTimeout(time.Minute))

		It("should download gardenadm in the control plane machine", func(ctx SpecContext) {
			Eventually(ctx, func(g Gomega) {
				stdOut, _, err := RunInMachine(ctx, technicalID, 0, "/opt/bin/gardenadm", "version")
				g.Expect(err).NotTo(HaveOccurred())
				g.Eventually(ctx, stdOut).Should(gbytes.Say("gardenadm version"))
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		It("should stop machine-controller-manager", func(ctx SpecContext) {
			deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameMachineControllerManager, Namespace: technicalID}}
			Eventually(ctx, Object(deployment)).Should(HaveField("Spec.Replicas", HaveValue(BeEquivalentTo(0))))
		}, SpecTimeout(time.Minute))

		It("should deploy the DNSRecord", func(ctx SpecContext) {
			dnsRecord := &extensionsv1alpha1.DNSRecord{ObjectMeta: metav1.ObjectMeta{Name: shootName + "-external", Namespace: technicalID}}
			Eventually(ctx, Object(dnsRecord)).Should(BeHealthy(health.CheckExtensionObject))
		}, SpecTimeout(time.Minute))

		It("should prepare extension resources for migration", func(ctx SpecContext) {
			extensionKinds := map[string]client.ObjectList{
				extensionsv1alpha1.InfrastructureResource: &extensionsv1alpha1.InfrastructureList{},
				extensionsv1alpha1.WorkerResource:         &extensionsv1alpha1.WorkerList{},
				extensionsv1alpha1.DNSRecordResource:      &extensionsv1alpha1.DNSRecordList{},
			}

			for kind, list := range extensionKinds {
				Eventually(ctx, ObjectList(list, client.InNamespace(technicalID))).Should(
					HaveField("Items", matchAllElements(
						HaveField("Status.DefaultStatus.LastOperation", And(
							HaveField("Type", gardencorev1beta1.LastOperationTypeMigrate),
							HaveField("State", gardencorev1beta1.LastOperationStateSucceeded),
						)),
					)),
					"should prepare %s resources for migration", kind,
				)
			}
		}, SpecTimeout(time.Minute))

		It("should deploy a bastion with the detected public IPs", func(ctx SpecContext) {
			bastion := &extensionsv1alpha1.Bastion{ObjectMeta: metav1.ObjectMeta{Name: "gardenadm-bootstrap", Namespace: technicalID}}
			Eventually(ctx, Object(bastion)).Should(And(
				HaveField("Spec.Ingress", Not(Or(
					ContainElement(HaveField("IPBlock.CIDR", "0.0.0.0/0")),
					ContainElement(HaveField("IPBlock.CIDR", "::/0")),
				))),
				BeHealthy(health.CheckExtensionObject),
			), "should be healthy and not have default (open) ingress CIDRs")
		}, SpecTimeout(time.Minute))

		It("should copy the manifests to the first control plane machine", func(ctx SpecContext) {
			Eventually(ctx, session.Err).Should(gbytes.Say("Copying manifests to the first control plane machine"))

			for file, content := range map[string]string{
				"manifests/shootstate.yaml":  "apiVersion: core.gardener.cloud/v1beta1\nkind: ShootState\n",
				"manifests/manifests.yaml":   "apiVersion: core.gardener.cloud/v1beta1\nkind: Shoot\n",
				"imagevector-overwrite.yaml": "garden.local.gardener.cloud:5001/local-skaffold_gardenadm",
			} {
				Eventually(ctx, func(g Gomega) *gbytes.Buffer {
					stdOut, _, err := RunInMachine(ctx, technicalID, 0, "cat", "/var/lib/gardenadm/"+file)
					g.Expect(err).NotTo(HaveOccurred())
					return stdOut
				}).Should(gbytes.Say(content), "expected file %s to have the right content", file)
			}
		}, SpecTimeout(time.Minute))

		It("should bootstrap the control plane", func(ctx SpecContext) {
			Eventually(ctx, session.Err).Should(gbytes.Say("Bootstrapping control plane on the first control plane machine"))
			Eventually(ctx, session.Out).Should(gbytes.Say("Your Shoot cluster control-plane has initialized successfully!"))
		}, SpecTimeout(15*time.Minute))

		It("should write the shoot kubeconfig to the specified file", func(ctx SpecContext) {
			Eventually(ctx, session.Err).Should(gbytes.Say("Writing kubeconfig of the self-hosted shoot to file"))

			// #nosec G304 -- kubeconfigOutputFile is controlled by the test
			Expect(os.ReadFile(kubeconfigOutputFile)).To(ContainSubstring("server: https://api.root.garden.local.gardener.cloud"))
		}, SpecTimeout(time.Minute))

		var kubeconfig string
		It("should store the shoot kubeconfig in the bootstrap cluster", func(ctx SpecContext) {
			kubeconfigSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "kubeconfig", Namespace: technicalID}}
			Eventually(ctx, Object(kubeconfigSecret)).Should(HaveField("Data", HaveKey("kubeconfig")))

			kubeconfig = strings.ReplaceAll(string(kubeconfigSecret.Data["kubeconfig"]), "api.root.garden.local.gardener.cloud", fmt.Sprintf("localhost:%d", localPort))
		}, SpecTimeout(time.Minute))

		var shootClientSet kubernetes.Interface
		It("should connect to the shoot", func(ctx SpecContext) {
			By("Forward port to control plane machine pod")
			fw, err := kubernetes.SetupPortForwarder(portForwardCtx, RuntimeClient.RESTConfig(), technicalID, machinePodName(ctx, technicalID, 0), localPort, 443)
			Expect(err).NotTo(HaveOccurred())

			go func() {
				if err := fw.ForwardPorts(); err != nil {
					Fail("Error forwarding ports: " + err.Error())
				}
			}()

			Eventually(fw.Ready()).Should(BeClosed())

			By("Create client set")
			Eventually(func() error {
				shootClientSet, err = kubernetes.NewClientFromBytes([]byte(kubeconfig),
					kubernetes.WithClientOptions(client.Options{Scheme: kubernetes.SeedScheme}),
					kubernetes.WithDisabledCachedClient(),
				)
				return err
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		It("should restore all persisted secrets in the shoot", func(ctx SpecContext) {
			var persistedSecrets map[string]corev1.Secret
			Eventually(ctx, func(g Gomega) {
				var err error
				persistedSecrets, err = shootmigration.GetPersistedSecrets(ctx, RuntimeClient.Client(), technicalID)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(persistedSecrets).NotTo(BeEmpty())
			}).Should(Succeed(), "bootstrap cluster should have persisted secrets")

			var shootSecrets map[string]corev1.Secret
			Eventually(ctx, func(g Gomega) {
				var err error
				shootSecrets, err = shootmigration.GetPersistedSecrets(ctx, shootClientSet.Client(), "kube-system")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(persistedSecrets).NotTo(BeEmpty())
			}).Should(Succeed())

			Expect(shootmigration.ComparePersistedSecrets(persistedSecrets, shootSecrets)).To(Succeed())
		}, SpecTimeout(time.Minute))

		It("should finish successfully", func(ctx SpecContext) {
			Wait(ctx, session)
			Eventually(ctx, session.Out).Should(gbytes.Say("work in progress"))
		}, SpecTimeout(time.Minute))

		It("should run successfully a second time (should be idempotent)", func(ctx SpecContext) {
			RunAndWait(ctx, "bootstrap", "-d", "../../../dev-setup/gardenadm/resources/generated/managed-infra")
		}, SpecTimeout(2*time.Minute))
	})
})

// matchAllElements returns a matcher that must succeed for all elements in a slice.
func matchAllElements(matcher gomegatypes.GomegaMatcher) gomegatypes.GomegaMatcher {
	// map all elements to the same given matcher
	return gstruct.MatchAllElements(func(any) string { return "" }, gstruct.Elements{
		"": matcher,
	})
}

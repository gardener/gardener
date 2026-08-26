// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package unmanagedinfra

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	. "github.com/onsi/gomega/gstruct"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/nodeagent/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/nodeagent"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
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

			shootClientSet                   kubernetes.Interface
			shootClusterKubeconfigPathOnHost = filepath.Join("..", "..", "..", "dev-setup", "kubeconfigs", "self-hosted-shoot", "kubeconfig")

			gardenClientSet                   kubernetes.Interface
			gardenClusterKubeconfigPathOnHost = filepath.Join("..", "..", "..", "dev-setup", "kubeconfigs", "virtual-garden", "kubeconfig")
		)

		initClientSet := func(ctx context.Context, clientSet *kubernetes.Interface, kubeconfigPath string, scheme client.Options) {
			if *clientSet != nil {
				return
			}
			Eventually(ctx, func() error {
				var err error
				*clientSet, err = kubernetes.NewClientFromFile("", kubeconfigPath,
					kubernetes.WithDisabledCachedClient(),
					kubernetes.WithClientOptions(scheme),
				)
				return err
			}).Should(Succeed())
		}

		initGardenClientSet := func(ctx context.Context) {
			initClientSet(ctx, &gardenClientSet, gardenClusterKubeconfigPathOnHost, client.Options{Scheme: kubernetes.GardenScheme})
		}

		initShootClientSet := func(ctx context.Context) {
			initClientSet(ctx, &shootClientSet, shootClusterKubeconfigPathOnHost, client.Options{Scheme: kubernetes.SeedScheme})
		}

		Context("gardenadm init + join", Ordered, Label("initjoin"), func() {
			It("should create a client for the self-hosted shoot API server", func(ctx SpecContext) {
				initShootClientSet(ctx)
			}, SpecTimeout(time.Minute))

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
					stdOut, _, err := execute(ctx, 0, "systemctl", "status", "gardener-node-agent")
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
					stdOut, _, err := execute(ctx, 0, "cat", "/var/lib/gardenadm/config-directory")
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
				gardenKomega                         Komega
				gardenClusterKubeconfigPathOnMachine = "/tmp/virtual-garden-kubeconfig"

				clusterAdminStaticToken string
			)

			It("should create a client for the self-hosted shoot API server", func(ctx SpecContext) {
				initShootClientSet(ctx)
			}, SpecTimeout(time.Minute))

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

			It("should create a client for garden cluster", func(ctx SpecContext) {
				initGardenClientSet(ctx)
				gardenKomega = New(gardenClientSet.Client())
			}, SpecTimeout(time.Minute))

			It("should copy the garden cluster kubeconfig to the machine pod", func(ctx SpecContext) {
				// In the test setup via Skaffold, we build the 'gardenadm' binary and copy it to the machine pods. Hence,
				// the binary is not available on the host without further ado. For simplicity, we copy the garden cluster
				// kubeconfig from the host into the machine pod here. This enables us to execute
				// 'gardenadm token create --print-connect-command' from the machine pod.
				By("Copy local garden cluster kubeconfig to file in machine pod")
				gardenClusterKubeconfig, err := os.ReadFile(gardenClusterKubeconfigPathOnHost) // #nosec: G304 -- variable points to a static file path
				Expect(err).NotTo(HaveOccurred())

				Eventually(ctx, func() error {
					_, _, err := execute(ctx, 0, "sh", "-c", fmt.Sprintf("echo '%s' > %s", string(gardenClusterKubeconfig), gardenClusterKubeconfigPathOnMachine))
					return err
				}).Should(Succeed())
			}, SpecTimeout(time.Minute))

			It("should generate a bootstrap token and connect the self-hosted shoot to Gardener", func(ctx SpecContext) {
				stdOut, _, err := execute(ctx, 0, "sh", "-c", fmt.Sprintf("KUBECONFIG=%s gardenadm token create --print-connect-command --shoot-namespace=%s --shoot-name=%s", gardenClusterKubeconfigPathOnMachine, shootNamespace, shootName))
				Expect(err).NotTo(HaveOccurred())
				connectCommand := strings.Split(strings.ReplaceAll(string(stdOut.Contents()), `"`, ``), " ")

				stdOut, _, err = execute(ctx, 0, append(connectCommand, "--log-level=debug")...)
				Expect(err).NotTo(HaveOccurred())

				Eventually(ctx, stdOut).Should(gbytes.Say("Your self-hosted shoot cluster has successfully been connected to Gardener!"))

				Eventually(ctx, func(g Gomega) {
					csrList := &certificatesv1.CertificateSigningRequestList{}
					g.Expect(gardenClientSet.Client().List(ctx, csrList)).To(Succeed())

					var gardenletCSR *certificatesv1.CertificateSigningRequest
					for i := range csrList.Items {
						if strings.HasPrefix(csrList.Items[i].Name, "shoot-csr") {
							gardenletCSR = &csrList.Items[i]
							break
						}
					}

					g.Expect(gardenletCSR).NotTo(BeNil())
					g.Expect(gardenletCSR.Status.Conditions).To(ContainCondition(
						MatchFields(IgnoreExtras, Fields{"Type": Equal(certificatesv1.CertificateApproved)}),
						MatchFields(IgnoreExtras, Fields{"Status": Equal(corev1.ConditionTrue)}),
						MatchFields(IgnoreExtras, Fields{"Message": Equal("Auto approving gardenlet client certificate after SubjectAccessReview.")}),
						MatchFields(IgnoreExtras, Fields{"Reason": Equal("AutoApproved")}),
					))
				}).Should(Succeed())
			}, SpecTimeout(time.Minute))

			It("should see the Shoot resource in the Gardener API with the correct UID", func(ctx SpecContext) {
				stdOut, _, err := execute(ctx, 0, "cat", "/var/lib/gardenadm/shoot-uid")
				Expect(err).NotTo(HaveOccurred())
				expectedShootStatusUID := types.UID(stdOut.Contents())

				Eventually(ctx, func(g Gomega) types.UID {
					g.Expect(gardenClientSet.Client().Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					return shoot.Status.UID
				}).Should(Equal(expectedShootStatusUID))
			}, SpecTimeout(time.Minute))

			It("should deploy and reconcile the BackupBucket resource", func(ctx SpecContext) {
				backupBucket := &gardencorev1beta1.BackupBucket{ObjectMeta: metav1.ObjectMeta{Name: string(shoot.Status.UID)}}
				Eventually(ctx, gardenKomega.Object(backupBucket)).Should(BeHealthy(health.CheckBackupBucket))
			}, SpecTimeout(time.Minute))

			It("should deploy and reconcile the BackupEntry resource", func(ctx SpecContext) {
				backupEntryName, err := gardenerutils.GenerateBackupEntryName(controlPlaneNamespace, shoot.Status.UID, shoot.UID)
				Expect(err).NotTo(HaveOccurred())

				backupEntry := &gardencorev1beta1.BackupEntry{ObjectMeta: metav1.ObjectMeta{Name: backupEntryName, Namespace: shootNamespace}}
				Eventually(ctx, gardenKomega.Object(backupEntry)).Should(BeHealthy(health.CheckBackupEntry))
			}, SpecTimeout(time.Minute))

			It("should deploy and reconcile the ControllerInstallations for the self-hosted Shoot", func(ctx SpecContext) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				Eventually(ctx, func(g Gomega) []gardencorev1beta1.ControllerInstallation {
					g.Expect(gardenClientSet.Client().List(ctx, controllerInstallationList, client.MatchingFields{
						core.ShootRefName:      shoot.Name,
						core.ShootRefNamespace: shoot.Namespace,
					})).To(Succeed())
					return controllerInstallationList.Items
				}).Should(ConsistOf(
					MatchFields(IgnoreExtras, Fields{"Spec": MatchFields(IgnoreExtras, Fields{"RegistrationRef": MatchFields(IgnoreExtras, Fields{"Name": Equal("provider-local")})})}),
					MatchFields(IgnoreExtras, Fields{"Spec": MatchFields(IgnoreExtras, Fields{"RegistrationRef": MatchFields(IgnoreExtras, Fields{"Name": Equal("networking-calico")})})}),
				))

				for _, controllerInstallation := range controllerInstallationList.Items {
					By("Waiting for ControllerInstallation " + controllerInstallation.Name + " to become healthy")
					Eventually(ctx, func(g Gomega) []gardencorev1beta1.Condition {
						g.Expect(gardenClientSet.Client().Get(ctx, client.ObjectKeyFromObject(&controllerInstallation), &controllerInstallation)).To(Succeed())
						return controllerInstallation.Status.Conditions
					}).Should(And(
						ContainCondition(OfType(gardencorev1beta1.ControllerInstallationValid), WithStatus(gardencorev1beta1.ConditionTrue)),
						ContainCondition(OfType(gardencorev1beta1.ControllerInstallationInstalled), WithStatus(gardencorev1beta1.ConditionTrue)),
						ContainCondition(OfType(gardencorev1beta1.ControllerInstallationHealthy), WithStatus(gardencorev1beta1.ConditionTrue)),
						ContainCondition(OfType(gardencorev1beta1.ControllerInstallationProgressing), WithStatus(gardencorev1beta1.ConditionFalse)),
					))
				}
			}, SpecTimeout(5*time.Minute))

			// `gardenadm connect` deploys the shoot gardenlet and registers the `Shoot` in the garden cluster. The
			// `gardenlet` reconciles immediately after it came up, and we should make sure that this (the `Shoot`
			// controller) works as expected and finishes its operation successfully.
			Context("shoot gardenlet reconciles self-hosted shoot", func() {
				var s *e2egardener.ShootContext

				BeforeAll(func() {
					s = (&e2egardener.TestContext{
						GardenClientSet: gardenClientSet,
						GardenClient:    gardenClientSet.Client(),
						GardenKomega:    New(gardenClientSet.Client()),
					}).ForShoot(shoot)
				})

				It("should wait for the self-hosted shoot to be reconciled and healthy", func(ctx SpecContext) {
					Eventually(ctx, func(g Gomega) bool {
						g.Expect(s.GardenKomega.Get(s.Shoot)()).To(Succeed())
						g.Expect(s.Shoot.Status.Gardener.Name).To(ContainSubstring("gardenlet"))
						// TODO(rfranzke): Uncomment this code and remove the manual checks once the Shoot controller
						//  has progressed and the .status.conditions properly reflect healthiness.
						//
						// completed, _ := shootoperation.ReconciliationSuccessful(s.Shoot)
						// return completed

						if s.Shoot.Generation != s.Shoot.Status.ObservedGeneration {
							return false
						}
						if len(s.Shoot.Status.Conditions) == 0 && s.Shoot.Status.LastOperation == nil {
							return false
						}
						if shoot.Status.LastOperation != nil {
							switch shoot.Status.LastOperation.Type {
							case gardencorev1beta1.LastOperationTypeCreate, gardencorev1beta1.LastOperationTypeReconcile, gardencorev1beta1.LastOperationTypeRestore:
								if shoot.Status.LastOperation.State != gardencorev1beta1.LastOperationStateSucceeded {
									return false
								}
							}
						}
						return true
					}).WithPolling(30 * time.Second).Should(BeTrue())

					By("Verifying ShootTaskUpdateGardenerNodeAgentSecretName task annotation has been removed")
					Expect(controllerutils.HasTask(s.Shoot.Annotations, v1beta1constants.ShootTaskUpdateGardenerNodeAgentSecretName)).To(BeFalse())
				}, SpecTimeout(30*time.Minute))

				It("should ensure the static token secret only contains the health-check token (cluster-admin token eliminated)", func(ctx SpecContext) {
					newestSecret, err := kubernetesutils.NewestObject(ctx, shootClientSet.Client(), &corev1.SecretList{}, nil, client.InNamespace(controlPlaneNamespace), client.MatchingLabels{
						"managed-by": "secrets-manager",
						"name":       "kube-apiserver-static-token",
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(newestSecret).NotTo(BeNil())

					secret := newestSecret.(*corev1.Secret)
					staticToken, err := secretsutils.LoadStaticTokenFromCSV("kube-apiserver-static-token", secret.Data[secretsutils.DataKeyStaticTokenCSV])
					Expect(err).NotTo(HaveOccurred())
					Expect(staticToken.Tokens).To(HaveLen(1))
					Expect(staticToken.Tokens[0].Username).To(Equal("health-check"))

					By("Verifying kube-apiserver no longer trusts the old cluster-admin static token")
					unauthenticatedClient, err := client.New(&rest.Config{
						Host:            shootClientSet.RESTConfig().Host,
						TLSClientConfig: rest.TLSClientConfig{CAData: shootClientSet.RESTConfig().CAData},
						BearerToken:     clusterAdminStaticToken,
					}, client.Options{})
					Expect(err).NotTo(HaveOccurred())
					Expect(unauthenticatedClient.List(ctx, &corev1.NamespaceList{})).To(MatchError(apierrors.IsUnauthorized, "IsUnauthorized"))
				}, SpecTimeout(time.Minute))

				It("should ensure the new admin kubeconfig uses a dynamic token and the token files contain JWTs", func(ctx SpecContext) {
					By("Verifying /etc/kubernetes/admin.conf uses tokenFile instead of an inline token")
					stdOut, _, err := execute(ctx, 0, "cat", "/etc/kubernetes/admin.conf")
					Expect(err).NotTo(HaveOccurred())

					adminKubeconfig, err := clientcmd.Load(stdOut.Contents())
					Expect(err).NotTo(HaveOccurred())
					Expect(adminKubeconfig.AuthInfos).To(HaveLen(1))
					for _, authInfo := range adminKubeconfig.AuthInfos {
						Expect(authInfo.Token).To(BeEmpty(), "admin.conf should not contain an inline token")
						Expect(authInfo.TokenFile).To(Equal("/etc/kubernetes/admin-token"))
					}

					By("Verifying /etc/kubernetes/admin-token contains a JWT")
					stdOut, _, err = execute(ctx, 0, "cat", "/etc/kubernetes/admin-token")
					Expect(err).NotTo(HaveOccurred())
					Expect(strings.Split(strings.TrimSpace(string(stdOut.Contents())), ".")).To(HaveLen(3), "admin-token should be a JWT (3 dot-separated parts)")

					for _, component := range []string{"kube-controller-manager", "kube-scheduler"} {
						By("Verifying " + component + " token file contains a JWT")
						tokenPath := "/var/lib/static-pods/" + component + "/kubeconfig/token"
						stdOut, _, err = execute(ctx, 0, "cat", tokenPath)
						Expect(err).NotTo(HaveOccurred(), "failed to read token file for %s", component)
						Expect(strings.Split(strings.TrimSpace(string(stdOut.Contents())), ".")).To(HaveLen(3), "%s token should be a JWT (3 dot-separated parts)", component)
					}
				}, SpecTimeout(time.Minute))

				It("should ensure node labels and gardener-node-agent config reflect the correct OSC secret name", func(ctx SpecContext) {
					By("Build map of worker pool -> newest original OSC name (the newest OSC is the one desired by gardenlet)")
					oscList := &extensionsv1alpha1.OperatingSystemConfigList{}
					Expect(shootClientSet.Client().List(ctx, oscList, client.InNamespace(controlPlaneNamespace))).To(Succeed())

					poolToNewestOSC := make(map[string]*extensionsv1alpha1.OperatingSystemConfig)
					for _, osc := range oscList.Items {
						if osc.Spec.Purpose != extensionsv1alpha1.OperatingSystemConfigPurposeReconcile {
							continue
						}
						poolName := osc.Labels[v1beta1constants.LabelWorkerPool]
						existing, ok := poolToNewestOSC[poolName]
						if !ok || osc.CreationTimestamp.After(existing.CreationTimestamp.Time) {
							poolToNewestOSC[poolName] = &osc
						}
					}

					By("List nodes and verify labels and GNA config")
					nodeList := &corev1.NodeList{}
					Expect(shootClientSet.Client().List(ctx, nodeList)).To(Succeed())

					for _, node := range nodeList.Items {
						poolName := node.Labels[v1beta1constants.LabelWorkerPool]
						newestOSC, ok := poolToNewestOSC[poolName]
						Expect(ok).To(BeTrue(), "no original OSC found for worker pool %q (node %s)", poolName, node.Name)

						// The GNA secret name is the OSC name without the "-original" suffix.
						expectedSecretName := strings.TrimSuffix(newestOSC.Name, "-original")

						By("Verifying node label for node " + node.Name)
						Expect(node.Labels[v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName]).To(Equal(expectedSecretName), "node %s has wrong %s label", node.Name, v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName)

						By("Verifying GNA config on machine " + node.Name)
						// Use find to locate the config file instead of constructing the path from the test binary's
						// version string, which may differ from the GNA binary's version string on the node.
						stdOut, _, err := execute(ctx, machineOrdinalFromNodeName(node.Name), "find", nodeagentconfigv1alpha1.BaseDir, "-maxdepth", "1", "-name", "config-*.yaml")
						Expect(err).NotTo(HaveOccurred(), "failed to list GNA config files on node %s", node.Name)
						configPath := strings.TrimSpace(string(stdOut.Contents()))
						Expect(configPath).NotTo(BeEmpty(), "no GNA config file found in %s on node %s", nodeagentconfigv1alpha1.BaseDir, node.Name)
						stdOut, _, err = execute(ctx, machineOrdinalFromNodeName(node.Name), "cat", configPath)
						Expect(err).NotTo(HaveOccurred(), "failed to read GNA config from node %s at path %s", node.Name, configPath)

						gnaConfig := &nodeagentconfigv1alpha1.NodeAgentConfiguration{}
						Expect(runtime.DecodeInto(nodeagent.Codec, stdOut.Contents(), gnaConfig)).To(Succeed(), "failed to decode GNA config on node %s", node.Name)
						Expect(gnaConfig.Controllers.OperatingSystemConfig.SecretName).To(Equal(expectedSecretName), "GNA config on node %s has wrong secret name", node.Name)
					}
				}, SpecTimeout(time.Minute))
			})
		})

		Context("self-hosted shoot -> seed promotion", Ordered, Label("seed"), func() {
			It("should create a client for garden cluster", func(ctx SpecContext) {
				initGardenClientSet(ctx)
			}, SpecTimeout(time.Minute))

			It("should create a client for the self-hosted shoot API server", func(ctx SpecContext) {
				initShootClientSet(ctx)
			}, SpecTimeout(time.Minute))

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

			BeforeAll(func(ctx SpecContext) {
				initGardenClientSet(ctx)

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

// nolint:unparam
func execute(ctx context.Context, ordinal int, command ...string) (*gbytes.Buffer, *gbytes.Buffer, error) {
	var stdOutBuffer, stdErrBuffer = gbytes.NewBuffer(), gbytes.NewBuffer()

	args := append([]string{"exec", machineContainerName(ordinal)}, command...)
	cmd := exec.CommandContext(ctx, "docker", args...) // #nosec G204 -- Used for e2e tests only.
	cmd.Stdout = io.MultiWriter(stdOutBuffer, gexec.NewPrefixedWriter("[out] ", GinkgoWriter))
	cmd.Stderr = io.MultiWriter(stdErrBuffer, gexec.NewPrefixedWriter("[err] ", GinkgoWriter))

	return stdOutBuffer, stdErrBuffer, cmd.Run()
}

func machineContainerName(ordinal int) string {
	return "gind-machine-" + strconv.Itoa(ordinal)
}

func machineOrdinalFromNodeName(nodeName string) int {
	ordinal, err := strconv.Atoi(strings.TrimPrefix(nodeName, "gind-machine-"))
	Expect(err).NotTo(HaveOccurred(), "failed to parse ordinal from node name %s", nodeName)
	return ordinal
}

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
		stdOut, _, err := execute(ctx, 0, "gardenadm", "token", "create", "--print-"+command+"-command")
		Expect(err).NotTo(HaveOccurred())
		gardenadmCommand := strings.Split(strings.ReplaceAll(string(stdOut.Contents()), `"`, ``), " ")

		stdOut, _, err = execute(ctx, 1, append(gardenadmCommand, "--log-level=debug")...)
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

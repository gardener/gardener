// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package gardenadm

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
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

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/nodeagent/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/networking/istio"
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

const (
	controlPlaneNamespace = "kube-system"
)

var (
	// ShootClusterKubeconfigPathOnHost is the path to the kubeconfig file for the self-hosted shoot cluster on the host machine.
	ShootClusterKubeconfigPathOnHost = filepath.Join("..", "..", "..", "dev-setup", "kubeconfigs", "self-hosted-shoot", "kubeconfig")
	// GardenClusterKubeconfigPathOnHost is the path to the kubeconfig file for the garden cluster on the host machine.
	GardenClusterKubeconfigPathOnHost = filepath.Join("..", "..", "..", "dev-setup", "kubeconfigs", "virtual-garden", "kubeconfig")
)

// ItShouldCreateShootClient ensures that a client for the self-hosted shoot API server is created.
func ItShouldCreateShootClient(shootClientSet *kubernetes.Interface) {
	GinkgoHelper()
	It("should create a client for the self-hosted shoot API server", func(ctx SpecContext) {
		initClientSet(ctx, shootClientSet, ShootClusterKubeconfigPathOnHost, client.Options{Scheme: kubernetes.SeedScheme})
	})
}

// ItShouldCreateGardenClient ensures that a client for the garden cluster is created.
func ItShouldCreateGardenClient(gardenClientSet *kubernetes.Interface) {
	GinkgoHelper()
	It("should create a client for garden cluster", func(ctx SpecContext) {
		initClientSet(ctx, gardenClientSet, GardenClusterKubeconfigPathOnHost, client.Options{Scheme: kubernetes.GardenScheme})
	})
}

// ItShouldPromoteSelfHostedShootToSeed verifies that the self-hosted shoot can be promoted to a seed.
func ItShouldPromoteSelfHostedShootToSeed(gardenClientSet, shootClientSet *kubernetes.Interface, shoot *gardencorev1beta1.Shoot) {
	GinkgoHelper()

	Context("self-hosted shoot -> seed promotion", Ordered, Label("seed"), func() {
		ItShouldCreateShootClient(shootClientSet)
		ItShouldCreateGardenClient(gardenClientSet)

		It("should ensure the ManagedSeed is healthy", func(ctx SpecContext) {
			managedSeed := &seedmanagementv1alpha1.ManagedSeed{ObjectMeta: metav1.ObjectMeta{Name: shoot.Name, Namespace: shoot.Namespace}}
			Eventually(ctx, func(g Gomega) {
				g.Expect((*gardenClientSet).Client().Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)).To(Succeed())
				g.Expect(health.CheckManagedSeed(managedSeed)).To(Succeed())
			}).Should(Succeed())
		}, SpecTimeout(5*time.Minute))

		It("should ensure the Seed is healthy", func(ctx SpecContext) {
			seed := &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: shoot.Name}}
			Eventually(ctx, func(g Gomega) {
				g.Expect((*gardenClientSet).Client().Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
				g.Expect(health.CheckSeed(seed, seed.Status.Gardener)).To(Succeed())
			}).Should(Succeed())
		}, SpecTimeout(5*time.Minute))

		It("should ensure the seed gardenlet runs in the garden namespace", func(ctx SpecContext) {
			Eventually(ctx, func(g Gomega) []corev1.Pod {
				podList := &corev1.PodList{}
				g.Expect((*shootClientSet).Client().List(ctx, podList, client.InNamespace("garden"), client.MatchingLabels{"app": "gardener", "role": "gardenlet"})).To(Succeed())
				return podList.Items
			}).ShouldNot(BeEmpty())
		}, SpecTimeout(2*time.Minute))
	})
}

// ItShouldCreateHostedShootOnPromotedSeed verifies that a hosted shoot can be created, used and deleted on the promoted seed.
func ItShouldCreateHostedShootOnPromotedSeed(gardenClientSet *kubernetes.Interface) {
	GinkgoHelper()

	Context("hosted shoot on promoted seed", Ordered, Label("hosted-shoot"), func() {
		var s *e2egardener.ShootContext

		ItShouldCreateGardenClient(gardenClientSet)

		It("should setup the test context", func() {
			hostedShoot := e2egardener.DefaultShoot("e2e-gardenadm")
			s = (&e2egardener.TestContext{
				GardenClientSet: *gardenClientSet,
				GardenClient:    (*gardenClientSet).Client(),
				GardenKomega:    New((*gardenClientSet).Client()),
			}).ForShoot(hostedShoot)
		})

		It("should create the hosted shoot", func(ctx SpecContext) {
			Eventually(ctx, func() error {
				return s.GardenClient.Create(ctx, s.Shoot)
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		It("should wait for the hosted shoot to be reconciled and healthy", func(ctx SpecContext) {
			Eventually(ctx, func(g Gomega) {
				g.Expect(s.GardenKomega.Get(s.Shoot)()).To(Succeed())
				completed, reason := shootoperation.ReconciliationSuccessful(s.Shoot)
				g.Expect(completed).To(BeTrue(), reason)
			}).WithPolling(30 * time.Second).Should(Succeed())
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
}

func initClientSet(ctx context.Context, clientSet *kubernetes.Interface, kubeconfigPath string, scheme client.Options) {
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

// ItShouldVerifyAfterConnect verifies that the gardenadm connect command was successful.
func ItShouldVerifyAfterConnect(
	gardenClientSetPtr *kubernetes.Interface,
	shoot *gardencorev1beta1.Shoot,
	runInMachine func(ctx context.Context, ordinal int, command ...string) (*gbytes.Buffer, *gbytes.Buffer, error),
) {
	GinkgoHelper()
	var (
		gardenClientSet kubernetes.Interface
		gardenKomega    Komega
	)

	It("should initialize the garden client set and Komega", func() {
		gardenClientSet = *gardenClientSetPtr
		gardenKomega = New(gardenClientSet.Client())
	})

	It("should approve the gardenlet CSR automatically", func(ctx SpecContext) {
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
				HaveField("Type", Equal(certificatesv1.CertificateApproved)),
				HaveField("Status", Equal(corev1.ConditionTrue)),
				HaveField("Message", Equal("Auto approving gardenlet client certificate after SubjectAccessReview.")),
				HaveField("Reason", Equal("AutoApproved")),
			))
		}).Should(Succeed())
	})

	It("should see the Shoot resource in the Gardener API with the correct UID", func(ctx SpecContext) {
		stdOut, _, err := runInMachine(ctx, 0, "cat", "/var/lib/gardenadm/shoot-uid")
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

		backupEntry := &gardencorev1beta1.BackupEntry{ObjectMeta: metav1.ObjectMeta{Name: backupEntryName, Namespace: shoot.Namespace}}
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
			HaveField("Spec.RegistrationRef.Name", Equal("provider-local")),
			HaveField("Spec.RegistrationRef.Name", Equal("networking-calico")),
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
}

// ItShouldBeReconciledByGardenlet verifies that the initial reconcile of the `gardenlet` for the self-hosted shoot is successful.
func ItShouldBeReconciledByGardenlet(
	gardenClientSetPtr, shootClientSetPtr *kubernetes.Interface,
	shoot *gardencorev1beta1.Shoot,
	clusterAdminStaticToken string,
	runInMachine func(ctx context.Context, ordinal int, command ...string) (*gbytes.Buffer, *gbytes.Buffer, error),
	runInNode func(ctx context.Context, nodeName string, command ...string) (*gbytes.Buffer, *gbytes.Buffer, error),
) {
	GinkgoHelper()

	Context("shoot gardenlet reconciles self-hosted shoot", func() {
		var (
			gardenClientSet, shootClientSet kubernetes.Interface
			s                               *e2egardener.ShootContext
		)

		BeforeAll(func() {
			gardenClientSet, shootClientSet = *gardenClientSetPtr, *shootClientSetPtr
			s = (&e2egardener.TestContext{
				GardenClientSet: gardenClientSet,
				GardenClient:    gardenClientSet.Client(),
				GardenKomega:    New(gardenClientSet.Client()),
			}).ForShoot(shoot)
		})

		It("should wait for the self-hosted shoot to be reconciled and healthy", func(ctx SpecContext) {
			Eventually(ctx, func(g Gomega) {
				g.Expect(s.GardenKomega.Get(s.Shoot)()).To(Succeed())
				g.Expect(s.Shoot.Status.Gardener.Name).To(ContainSubstring("gardenlet"))
				completed, reason := shootoperation.ReconciliationSuccessful(s.Shoot)
				g.Expect(completed).To(BeTrue(), reason)
			}).WithPolling(30 * time.Second).Should(Succeed())

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
			stdOut, _, err := runInMachine(ctx, 0, "cat", "/etc/kubernetes/admin.conf")
			Expect(err).NotTo(HaveOccurred())

			adminKubeconfig, err := clientcmd.Load(stdOut.Contents())
			Expect(err).NotTo(HaveOccurred())
			Expect(adminKubeconfig.AuthInfos).To(HaveLen(1))
			for _, authInfo := range adminKubeconfig.AuthInfos {
				Expect(authInfo.Token).To(BeEmpty(), "admin.conf should not contain an inline token")
				Expect(authInfo.TokenFile).To(Equal("/etc/kubernetes/admin-token"))
			}

			By("Verifying /etc/kubernetes/admin-token contains a JWT")
			stdOut, _, err = runInMachine(ctx, 0, "cat", "/etc/kubernetes/admin-token")
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.Split(strings.TrimSpace(string(stdOut.Contents())), ".")).To(HaveLen(3), "admin-token should be a JWT (3 dot-separated parts)")

			for _, component := range []string{"kube-controller-manager", "kube-scheduler"} {
				By("Verifying " + component + " token file contains a JWT")
				tokenPath := "/var/lib/static-pods/" + component + "/kubeconfig/token"
				stdOut, _, err = runInMachine(ctx, 0, "cat", tokenPath)
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
				stdOut, _, err := runInNode(ctx, node.Name, "find", nodeagentconfigv1alpha1.BaseDir, "-maxdepth", "1", "-name", "config-*.yaml")
				Expect(err).NotTo(HaveOccurred(), "failed to list GNA config files on node %s", node.Name)
				configPath := strings.TrimSpace(string(stdOut.Contents()))
				Expect(configPath).NotTo(BeEmpty(), "no GNA config file found in %s on node %s", nodeagentconfigv1alpha1.BaseDir, node.Name)
				stdOut, _, err = runInNode(ctx, node.Name, "cat", configPath)
				Expect(err).NotTo(HaveOccurred(), "failed to read GNA config from node %s at path %s", node.Name, configPath)

				gnaConfig := &nodeagentconfigv1alpha1.NodeAgentConfiguration{}
				Expect(runtime.DecodeInto(nodeagent.Codec, stdOut.Contents(), gnaConfig)).To(Succeed(), "failed to decode GNA config on node %s", node.Name)
				Expect(gnaConfig.Controllers.OperatingSystemConfig.SecretName).To(Equal(expectedSecretName), "GNA config on node %s has wrong secret name", node.Name)
			}
		}, SpecTimeout(time.Minute))

		It("should deploy the Istio ingress gateway for the self-hosted shoot", func(ctx SpecContext) {
			mr := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{
				Name:      istio.SelfHostedShootNamePrefix + "istio",
				Namespace: v1beta1constants.IstioSystemNamespace,
			}}
			Eventually(ctx, func(g Gomega) {
				g.Expect(shootClientSet.Client().Get(ctx, client.ObjectKeyFromObject(mr), mr)).To(Succeed())
				g.Expect(health.CheckManagedResource(mr)).To(Succeed())
			}).Should(Succeed())
		}, SpecTimeout(5*time.Minute))

		It("should deploy the ingress DNS record for the self-hosted shoot", func(ctx SpecContext) {
			var providers []gardencorev1beta1.DNSProvider
			if shoot.Spec.DNS != nil {
				providers = shoot.Spec.DNS.Providers
			}
			primaryProvider := v1beta1helper.FindPrimaryDNSProvider(providers)
			if primaryProvider == nil || (primaryProvider.Type != nil && *primaryProvider.Type == "unmanaged") {
				Skip("shoot has no managed DNS provider, ingress DNS record is not deployed")
			}

			dnsRecord := &extensionsv1alpha1.DNSRecord{ObjectMeta: metav1.ObjectMeta{
				Name:      shoot.Name + "-" + v1beta1constants.LabelDNSRecordIngress,
				Namespace: controlPlaneNamespace,
			}}
			Eventually(ctx, func(g Gomega) {
				g.Expect(shootClientSet.Client().Get(ctx, client.ObjectKeyFromObject(dnsRecord), dnsRecord)).To(Succeed())
				g.Expect(health.CheckExtensionObject(dnsRecord)).To(Succeed())
			}).Should(Succeed())
		}, SpecTimeout(5*time.Minute))
	})
}

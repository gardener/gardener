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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	e2egardener "github.com/gardener/gardener/test/e2e/gardener"
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
	})
}

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"crypto/rand"
	"crypto/x509/pkix"
	"fmt"
	"net"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	certutil "k8s.io/client-go/util/cert"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	fakedbus "github.com/gardener/gardener/pkg/nodeagent/dbus/fake"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Nodeagent", func() {
	var (
		ctx         context.Context
		namespace   string
		hostName    string
		fooUsername string
		fooToken    string

		fakeSeedClient    client.Client
		fakeSecretManager secretsmanager.Interface
		fakeDBus          *fakedbus.DBus

		b *AutonomousBotanist
	)

	BeforeEach(func() {
		ctx = context.Background()

		namespace = "kube-system"
		hostName = "test"

		foo := "foo"
		fooUsername = "system:bootstrap:" + foo
		fooToken = foo + ".token"

		fakeSeedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeSecretManager = fakesecretsmanager.New(fakeSeedClient, namespace)
		fakeDBus = fakedbus.New()

		b = &AutonomousBotanist{
			Botanist: &botanistpkg.Botanist{
				Operation: &operation.Operation{
					Logger:         logr.Discard(),
					Shoot:          &shoot.Shoot{},
					SecretsManager: fakeSecretManager,
					SeedClientSet: fakekubernetes.
						NewClientSetBuilder().
						WithClient(fakeSeedClient).
						WithRESTConfig(&rest.Config{}).
						Build(),
				},
			},
			FS:       afero.Afero{Fs: afero.NewMemMapFs()},
			DBus:     fakeDBus,
			HostName: hostName,
		}
		b.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: namespace,
			},
		})

		Expect(fakeSeedClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: namespace}})).To(Succeed())
	})

	Describe("#WriteNodeAgentKubeconfig", func() {
		var kubeconfigRequested bool

		BeforeEach(func() {
			kubeconfigRequested = false

			DeferCleanup(test.WithVar(&RequestAndStoreKubeconfig, func(_ context.Context, _ logr.Logger, _ afero.Afero, _ *rest.Config, _ string) error {
				kubeconfigRequested = true
				return nil
			}))
		})

		It("should do nothing when bootstrap token file does not exist", func() {
			Expect(b.WriteNodeAgentKubeconfig(ctx)).To(Succeed())
			Expect(kubeconfigRequested).To(BeFalse())
		})

		It("should request a kubeconfig when bootstrap token file exists", func() {
			Expect(b.FS.WriteFile("/var/lib/gardener-node-agent/credentials/bootstrap-token", []byte(fooToken), 0o600)).To(Succeed())

			Expect(b.WriteNodeAgentKubeconfig(ctx)).To(Succeed())
			Expect(kubeconfigRequested).To(BeTrue())
		})

		It("should do nothing when kubeconfig and bootstrap token file exist", func() {
			Expect(b.FS.WriteFile("/var/lib/gardener-node-agent/credentials/bootstrap-token", []byte(fooToken), 0o600)).To(Succeed())
			Expect(b.FS.WriteFile("/var/lib/gardener-node-agent/credentials/kubeconfig", []byte("fooconfig"), 0o600)).To(Succeed())

			Expect(b.WriteNodeAgentKubeconfig(ctx)).To(Succeed())
			Expect(kubeconfigRequested).To(BeFalse())
		})
	})

	Describe("#ApproveNodeAgentCertificateSigningRequest", func() {
		It("should return an error when bootstrap token file does not exist", func() {
			Expect(b.ApproveNodeAgentCertificateSigningRequest(ctx)).To(MatchError(ContainSubstring("failed to read bootstrap token file")))
		})

		It("should return an error when no CSR was found for the node", func() {
			Expect(b.FS.WriteFile("/var/lib/gardener-node-agent/credentials/bootstrap-token", []byte(fooToken), 0o600)).To(Succeed())

			Expect(b.ApproveNodeAgentCertificateSigningRequest(ctx)).To(MatchError(Equal(fmt.Sprintf("no certificate signing request found for gardener-node-agent from username %q", fooUsername))))
		})

		It("should approve the CSR when not already approved", func() {
			Expect(b.FS.WriteFile("/var/lib/gardener-node-agent/credentials/bootstrap-token", []byte(fooToken), 0o600)).To(Succeed())

			privateKey, err := secretsutils.FakeGenerateKey(rand.Reader, 4096)
			Expect(err).NotTo(HaveOccurred())
			certificateSubject := &pkix.Name{
				CommonName: "gardener.cloud:node-agent:machine:" + hostName,
			}
			csrData, err := certutil.MakeCSR(privateKey, certificateSubject, []string{}, []net.IP{})
			Expect(err).NotTo(HaveOccurred())

			csr := &certificatesv1.CertificateSigningRequest{
				ObjectMeta: metav1.ObjectMeta{Name: "csr"},
				Spec: certificatesv1.CertificateSigningRequestSpec{
					Username:   fooUsername,
					Request:    csrData,
					SignerName: certificatesv1.KubeAPIServerClientSignerName,
				},
			}
			Expect(fakeSeedClient.Create(ctx, csr)).To(Succeed())

			Expect(b.ApproveNodeAgentCertificateSigningRequest(ctx)).To(Succeed())

			Expect(fakeSeedClient.Get(ctx, client.ObjectKeyFromObject(csr), csr)).To(Succeed())
			Expect(csr.Status.Conditions).To(HaveExactElements(certificatesv1.CertificateSigningRequestCondition{
				Type:    certificatesv1.CertificateApproved,
				Status:  corev1.ConditionTrue,
				Reason:  "RequestApproved",
				Message: "Approving gardener-node-agent client certificate signing request via gardenadm",
			}))
		})

		It("should not approve the CSR and return an error if the CSR is not for gardener-node-agent", func() {
			Expect(b.FS.WriteFile("/var/lib/gardener-node-agent/credentials/bootstrap-token", []byte(fooToken), 0o600)).To(Succeed())

			privateKey, err := secretsutils.FakeGenerateKey(rand.Reader, 4096)
			Expect(err).NotTo(HaveOccurred())
			certificateSubject := &pkix.Name{
				CommonName: "foobar",
			}
			csrData, err := certutil.MakeCSR(privateKey, certificateSubject, []string{}, []net.IP{})
			Expect(err).NotTo(HaveOccurred())

			csr := &certificatesv1.CertificateSigningRequest{
				ObjectMeta: metav1.ObjectMeta{Name: "csr"},
				Spec: certificatesv1.CertificateSigningRequestSpec{
					Username:   fooUsername,
					Request:    csrData,
					SignerName: certificatesv1.KubeAPIServerClientSignerName,
				},
			}
			Expect(fakeSeedClient.Create(ctx, csr)).To(Succeed())

			Expect(b.ApproveNodeAgentCertificateSigningRequest(ctx)).To(MatchError(Equal(fmt.Sprintf("no certificate signing request found for gardener-node-agent from username %q", fooUsername))))

			Expect(fakeSeedClient.Get(ctx, client.ObjectKeyFromObject(csr), csr)).To(Succeed())
			Expect(csr.Status.Conditions).To(BeEmpty())
		})
	})
})

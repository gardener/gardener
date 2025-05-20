// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
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
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Kubelet", func() {
	var (
		ctx context.Context

		namespace string
		hostName  string

		fakeSeedClient    client.Client
		fakeSecretManager secretsmanager.Interface
		fakeDBus          *fakedbus.DBus

		b *AutonomousBotanist
	)

	BeforeEach(func() {
		ctx = context.Background()
		namespace = "kube-system"
		hostName = "test"

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

	Describe("#WriteBootstrapToken", func() {
		It("should create a bootstrap token", func() {
			Expect(b.FS.Exists("/var/lib/gardener-node-agent/credentials/bootstrap-token")).To(BeFalse())
			Expect(b.WriteBootstrapToken(ctx)).To(Succeed())
			Expect(b.FS.Exists("/var/lib/gardener-node-agent/credentials/bootstrap-token")).To(BeTrue())
		})
	})

	Describe("#WriteKubeletBootstrapKubeconfig", func() {
		DescribeTable("should write the kubelet bootstrap kubeconfig",
			func(createToken bool) {
				Expect(b.FS.WriteFile("/var/lib/kubelet/kubeconfig-real", []byte{}, 0o600)).To(Succeed())
				Expect(b.FS.Exists("/var/lib/gardener-node-agent/tmp")).To(BeFalse())
				Expect(b.FS.Exists("/var/lib/gardener-node-agent/credentials")).To(BeFalse())
				Expect(b.FS.Exists("/var/lib/kubelet/kubeconfig-real")).To(BeTrue())
				Expect(b.FS.Exists("/var/lib/kubelet/kubeconfig-bootstrap")).To(BeFalse())
				if !createToken {
					Expect(b.FS.WriteFile("/var/lib/gardener-node-agent/credentials/bootstrap-token", []byte{}, 0o600)).To(Succeed())
				} else {
					Expect(b.FS.Exists("/var/lib/gardener-node-agent/credentials/bootstrap-token")).To(BeFalse())
				}
				Expect(b.WriteKubeletBootstrapKubeconfig(ctx)).To(Succeed())
				Expect(b.FS.Exists("/var/lib/gardener-node-agent/tmp")).To(BeTrue())
				Expect(b.FS.Exists("/var/lib/gardener-node-agent/credentials")).To(BeTrue())
				Expect(b.FS.Exists("/var/lib/gardener-node-agent/credentials/bootstrap-token")).To(BeTrue())
				Expect(b.FS.Exists("/var/lib/kubelet/kubeconfig-real")).To(BeFalse())
				Expect(b.FS.Exists("/var/lib/kubelet/kubeconfig-bootstrap")).To(BeTrue())
			},

			Entry("with creation of token file", true),
			Entry("with existing token file", false),
		)
	})

	Describe("#BootstrapKubelet", func() {
		BeforeEach(func() {
			DeferCleanup(test.WithVar(&RequestAndStoreKubeconfig, func(_ context.Context, _ logr.Logger, _ afero.Afero, _ *rest.Config, _ string) error { return nil }))
		})

		It("should do nothing when the node was already found", func() {
			Expect(fakeSeedClient.Create(ctx, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "foo", Labels: map[string]string{"kubernetes.io/hostname": hostName}}})).To(Succeed())

			Expect(b.BootstrapKubelet(ctx)).To(Succeed())

			Expect(fakeSeedClient.Get(ctx, client.ObjectKey{Name: "system:node-bootstrapper"}, &rbacv1.ClusterRoleBinding{})).To(BeNotFoundError())
			Expect(fakeSeedClient.Get(ctx, client.ObjectKey{Name: "system:certificates.k8s.io:certificatesigningrequests:nodeclient"}, &rbacv1.ClusterRoleBinding{})).To(BeNotFoundError())
			Expect(fakeSeedClient.Get(ctx, client.ObjectKey{Name: "system:certificates.k8s.io:certificatesigningrequests:selfnodeclient"}, &rbacv1.ClusterRoleBinding{})).To(BeNotFoundError())

			Expect(b.FS.Exists("/var/lib/gardener-node-agent/credentials/bootstrap-token")).To(BeFalse())
		})

		It("should write a bootstrap token and restart the kubelet unit", func() {
			Expect(b.BootstrapKubelet(ctx)).To(Succeed())

			Expect(fakeSeedClient.Get(ctx, client.ObjectKey{Name: "system:node-bootstrapper"}, &rbacv1.ClusterRoleBinding{})).To(Succeed())
			Expect(fakeSeedClient.Get(ctx, client.ObjectKey{Name: "system:certificates.k8s.io:certificatesigningrequests:nodeclient"}, &rbacv1.ClusterRoleBinding{})).To(Succeed())
			Expect(fakeSeedClient.Get(ctx, client.ObjectKey{Name: "system:certificates.k8s.io:certificatesigningrequests:selfnodeclient"}, &rbacv1.ClusterRoleBinding{})).To(Succeed())

			Expect(b.FS.Exists("/var/lib/gardener-node-agent/credentials/bootstrap-token")).To(BeTrue())
			Expect(fakeDBus.Actions).To(HaveExactElements(fakedbus.SystemdAction{Action: fakedbus.ActionRestart, UnitNames: []string{"kubelet.service"}}))
		})
	})

	Describe("#ApproveKubeletServerCertificateSigningRequest", func() {
		It("should do nothing because the server certificate file already exists", func() {
			file, err := b.FS.Create("/var/lib/kubelet/pki/kubelet-server-current.pem")
			Expect(err).NotTo(HaveOccurred())
			Expect(file).NotTo(BeNil())

			Expect(b.ApproveKubeletServerCertificateSigningRequest(ctx)).To(Succeed())
		})

		It("should return an error when no CSR was found for the node", func() {
			Expect(b.ApproveKubeletServerCertificateSigningRequest(ctx)).To(MatchError(ContainSubstring("no certificate signing request found for node")))
		})

		It("should approve the CSR when not already approved", func() {
			csr := &certificatesv1.CertificateSigningRequest{
				ObjectMeta: metav1.ObjectMeta{Name: "csr"},
				Spec:       certificatesv1.CertificateSigningRequestSpec{Username: "system:node:" + hostName},
			}
			Expect(fakeSeedClient.Create(ctx, csr)).To(Succeed())

			Expect(b.ApproveKubeletServerCertificateSigningRequest(ctx)).To(Succeed())

			Expect(fakeSeedClient.Get(ctx, client.ObjectKeyFromObject(csr), csr)).To(Succeed())
			Expect(csr.Status.Conditions).To(HaveExactElements(certificatesv1.CertificateSigningRequestCondition{
				Type:    certificatesv1.CertificateApproved,
				Status:  corev1.ConditionTrue,
				Reason:  "RequestApproved",
				Message: "Approving kubelet server certificate signing request via gardenadm",
			}))
		})
	})
})

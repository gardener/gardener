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
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	certificatesv1 "k8s.io/api/certificates/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	certutil "k8s.io/client-go/util/cert"
	testclock "k8s.io/utils/clock/testing"
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
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("NodeAgent", func() {
	var (
		ctx         context.Context
		namespace   string
		hostName    string
		fooUsername string
		fooToken    string

		fakeSeedClient    client.Client
		fakeSecretManager secretsmanager.Interface
		fakeDBus          *fakedbus.DBus
		fakeFS            afero.Afero
		fakeClock         *testclock.FakeClock

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
		fakeFS = afero.Afero{Fs: afero.NewMemMapFs()}
		fakeClock = testclock.NewFakeClock(time.Now())

		b = &AutonomousBotanist{
			Botanist: &botanistpkg.Botanist{
				Operation: &operation.Operation{
					Logger:         logr.Discard(),
					Clock:          fakeClock,
					Shoot:          &shoot.Shoot{},
					SecretsManager: fakeSecretManager,
					SeedClientSet: fakekubernetes.
						NewClientSetBuilder().
						WithClient(fakeSeedClient).
						WithRESTConfig(&rest.Config{}).
						Build(),
				},
			},
			FS:       fakeFS,
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

	Describe("#ActivateGardenerNodeAgent", func() {
		It("should do nothing because the node-agent is already bootstrapped", func() {
			_, err := fakeFS.Create("/var/lib/gardener-node-agent/credentials/kubeconfig")
			Expect(err).NotTo(HaveOccurred())

			Expect(b.ActivateGardenerNodeAgent(ctx)).To(Succeed())

			_, err = fakeFS.Stat("/var/lib/gardener-node-agent/machine-name")
			Expect(err).To(MatchError(ContainSubstring("file does not exist")))
		})

		It("should write the machine name and the bootstrap token", func() {
			Expect(b.ActivateGardenerNodeAgent(ctx)).To(Succeed())

			machineName, err := fakeFS.ReadFile("/var/lib/gardener-node-agent/machine-name")
			Expect(err).NotTo(HaveOccurred())
			Expect(machineName).To(Equal([]byte(hostName)))

			bootstrapToken, err := fakeFS.ReadFile("/var/lib/gardener-node-agent/credentials/bootstrap-token")
			Expect(err).NotTo(HaveOccurred())
			Expect(bootstrapToken).NotTo(BeEmpty())
		})

		It("should create the temporary cluster-admin binding for bootstrapping the node-agent", func() {
			Expect(b.ActivateGardenerNodeAgent(ctx)).To(Succeed())

			clusterRoleBinding := &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "zzz-temporary-cluster-admin-access-for-bootstrapping"}}
			Expect(fakeSeedClient.Get(ctx, client.ObjectKeyFromObject(clusterRoleBinding), clusterRoleBinding)).To(Succeed())

			Expect(clusterRoleBinding.RoleRef).To(Equal(rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "cluster-admin",
			}))
			Expect(clusterRoleBinding.Subjects).To(HaveExactElements(
				rbacv1.Subject{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "Group",
					Name:     "system:bootstrappers",
				},
				rbacv1.Subject{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "Group",
					Name:     "gardener.cloud:node-agents",
				},
			))
		})

		It("should remove the real kubelet kubeconfig file to make sure it bootstraps", func() {
			_, err := fakeFS.Create("/var/lib/kubelet/kubeconfig-real")
			Expect(err).NotTo(HaveOccurred())

			Expect(b.ActivateGardenerNodeAgent(ctx)).To(Succeed())

			_, err = fakeFS.Stat("/var/lib/kubelet/kubeconfig-real")
			Expect(err).To(MatchError(ContainSubstring("file does not exist")))
		})

		It("should start the gardener-node-agent systemd unit", func() {
			Expect(b.ActivateGardenerNodeAgent(ctx)).To(Succeed())

			Expect(fakeDBus.Actions).To(ContainElement(fakedbus.SystemdAction{
				Action:    fakedbus.ActionStart,
				UnitNames: []string{"gardener-node-agent.service"},
			}))
		})
	})

	Describe("#ApproveNodeAgentCertificateSigningRequest", func() {
		It("should return an error when bootstrap token file does not exist", func() {
			Expect(b.ApproveNodeAgentCertificateSigningRequest(ctx)).To(Succeed())
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

	Describe("#FinalizeGardenerNodeAgentBootstrapping", func() {
		It("should delete the temporary cluster-admin ClusterRoleBinding", func() {
			clusterRoleBinding := &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "zzz-temporary-cluster-admin-access-for-bootstrapping"}}
			Expect(fakeSeedClient.Create(ctx, clusterRoleBinding)).To(Succeed())

			Expect(b.FinalizeGardenerNodeAgentBootstrapping(ctx)).To(Succeed())

			Expect(fakeSeedClient.Get(ctx, client.ObjectKeyFromObject(clusterRoleBinding), clusterRoleBinding)).To(BeNotFoundError())
		})
	})

	Describe("#WaitUntilGardenerNodeAgentLeaseIsRenewed", func() {
		It("should return an error there is no node for the hostname", func() {
			Expect(b.WaitUntilGardenerNodeAgentLeaseIsRenewed(ctx)).To(MatchError(ContainSubstring("was not created yet")))
		})

		When("there is a node for the hostname", func() {
			var lease *coordinationv1.Lease

			BeforeEach(func() {
				Expect(fakeSeedClient.Create(ctx, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "node",
						Labels: map[string]string{"kubernetes.io/hostname": hostName},
					},
				})).To(Succeed())

				lease = &coordinationv1.Lease{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gardener-node-agent-node",
						Namespace: "kube-system",
					},
				}

				DeferCleanup(test.WithVars(
					&WaitForNodeAgentLeaseInterval, 50*time.Millisecond,
					&WaitForNodeAgentLeaseTimeout, 500*time.Millisecond,
				))
			})

			It("should return an error when the lease is not found", func() {
				Expect(b.WaitUntilGardenerNodeAgentLeaseIsRenewed(ctx)).To(MatchError(ContainSubstring("not found, gardener-node-agent might not be ready yet")))
			})

			It("should return an error when the renew time in the lease is empty", func() {
				Expect(fakeSeedClient.Create(ctx, lease)).To(Succeed())

				Expect(b.WaitUntilGardenerNodeAgentLeaseIsRenewed(ctx)).To(MatchError(ContainSubstring("not renewed yet, gardener-node-agent might not be ready yet")))
			})

			It("should return an error when the lease is not renewed", func() {
				lease.Spec.RenewTime = &metav1.MicroTime{Time: fakeClock.Now().Add(-time.Minute)}
				Expect(fakeSeedClient.Create(ctx, lease)).To(Succeed())

				Expect(b.WaitUntilGardenerNodeAgentLeaseIsRenewed(ctx)).To(MatchError(ContainSubstring("not renewed yet, gardener-node-agent might not be ready yet")))
			})

			It("should succeed when the lease is renewed", func() {
				lease.Spec.RenewTime = &metav1.MicroTime{Time: fakeClock.Now().Add(time.Minute)}
				Expect(fakeSeedClient.Create(ctx, lease)).To(Succeed())

				Expect(b.WaitUntilGardenerNodeAgentLeaseIsRenewed(ctx)).To(Succeed())
			})
		})
	})
})

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package csrapprover_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509/pkix"
	"net"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	certutil "k8s.io/client-go/util/cert"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcemanagerclient "github.com/gardener/gardener/pkg/resourcemanager/client"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("CertificateSigningRequest Approver Controller tests", func() {
	var (
		privateKey         *rsa.PrivateKey
		certificateSubject *pkix.Name

		ip1       = "1.2.3.4"
		ip2       = "5.6.7.8"
		ip3       = "9.0.1.2"
		ipv6      = "2080:1000:1000:8ff0:0:0:0:0"
		ipv6short = "2080:1000:1000:8ff0::"
		ips       []net.IP
		dnsName1  = "foo.bar"
		dnsName2  = "bar.baz"
		dnsName3  = "baz.foo"
		dnsName4  = "baz.bar"
		dnsNames  []string

		csr     *certificatesv1.CertificateSigningRequest
		node    *corev1.Node
		machine *machinev1alpha1.Machine
	)

	BeforeEach(func() {
		privateKey, _ = secretsutils.FakeGenerateKey(rand.Reader, 4096)
		certificateSubject = &pkix.Name{
			CommonName:   userNameKubelet,
			Organization: []string{user.NodesGroup},
		}

		ips = []net.IP{net.ParseIP(ip1), net.ParseIP(ip2), net.ParseIP(ip3), net.ParseIP(ipv6short)}
		dnsNames = []string{dnsName1, dnsName2, dnsName3, dnsName4}

		csr = &certificatesv1.CertificateSigningRequest{
			// Username, UID, Groups will be injected by API server.
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testID + "-",
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: certificatesv1.CertificateSigningRequestSpec{
				Usages: []certificatesv1.KeyUsage{
					certificatesv1.UsageDigitalSignature,
					certificatesv1.UsageKeyEncipherment,
					certificatesv1.UsageClientAuth,
				},
			},
		}

		node = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   nodeName,
				Labels: map[string]string{testID: testRunID},
			},
		}

		machine = &machinev1alpha1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      machineName,
				Namespace: testNamespace.Name,
				Labels: map[string]string{
					testID: testRunID,
				},
			},
		}
	})

	JustBeforeEach(func() {
		By("Generate CSR data")
		csrData, err := certutil.MakeCSR(privateKey, certificateSubject, dnsNames, ips)
		Expect(err).NotTo(HaveOccurred())
		csr.Spec.Request = csrData
	})

	Context("unhandled CSR", func() {
		BeforeEach(func() {
			csr.Spec.SignerName = certificatesv1.KubeAPIServerClientSignerName
		})

		JustBeforeEach(func() {
			By("Create CertificateSigningRequest")
			Expect(testClientKubelet.Create(ctx, csr)).To(Succeed())
			log.Info("Created CertificateSigningRequest for test", "certificateSigningRequest", client.ObjectKeyFromObject(csr))

			DeferCleanup(func() {
				By("Delete CertificateSigningRequest")
				Expect(client.IgnoreNotFound(testClientKubelet.Delete(ctx, csr))).To(Succeed())
			})
		})

		It("should ignore the CSR and do nothing", func() {
			Consistently(func(g Gomega) {
				g.Expect(testClientKubelet.Get(ctx, client.ObjectKeyFromObject(csr), csr)).To(Succeed())
				g.Expect(csr.Status.Conditions).To(BeEmpty())
			}).Should(Succeed())
		})
	})

	Context("kubelet server certificate", func() {
		BeforeEach(func() {
			csr.Spec.SignerName = certificatesv1.KubeletServingSignerName
		})

		JustBeforeEach(func() {
			By("Create CertificateSigningRequest")
			Expect(testClientKubelet.Create(ctx, csr)).To(Succeed())
			log.Info("Created CertificateSigningRequest for test", "certificateSigningRequest", client.ObjectKeyFromObject(csr))

			DeferCleanup(func() {
				By("Delete CertificateSigningRequest")
				Expect(client.IgnoreNotFound(testClientKubelet.Delete(ctx, csr))).To(Succeed())
			})
		})

		Context("constraints fulfilled", func() {
			BeforeEach(func() {
				createNode(node)
				createMachine(machine, true)
				patchNodeAddresses(node,
					corev1.NodeAddress{Type: corev1.NodeHostName, Address: dnsName1},
					corev1.NodeAddress{Type: corev1.NodeHostName, Address: dnsName2},
					corev1.NodeAddress{Type: corev1.NodeInternalDNS, Address: dnsName3},
					corev1.NodeAddress{Type: corev1.NodeExternalDNS, Address: dnsName4},
					corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: ip1},
					corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: ip2},
					corev1.NodeAddress{Type: corev1.NodeExternalIP, Address: ip3},
					corev1.NodeAddress{Type: corev1.NodeExternalIP, Address: ipv6},
				)
			})

			It("should approve the CSR", func() {
				Eventually(func(g Gomega) {
					g.Expect(testClientKubelet.Get(ctx, client.ObjectKeyFromObject(csr), csr)).To(Succeed())
					g.Expect(csr.Status.Conditions).To(ContainElement(And(
						HaveField("Type", certificatesv1.CertificateApproved),
						HaveField("Reason", "RequestApproved"),
						HaveField("Message", "Approving kubelet server certificate CSR (all checks passed)"),
					)))
				}).Should(Succeed())
			})
		})

		Context("constraints violated", func() {
			runTest := func(expectedReason string) {
				It("should deny the CSR", func() {
					EventuallyWithOffset(1, func(g Gomega) {
						g.Expect(testClientKubelet.Get(ctx, client.ObjectKeyFromObject(csr), csr)).To(Succeed())
						g.Expect(csr.Status.Conditions).To(ContainElement(And(
							HaveField("Type", certificatesv1.CertificateDenied),
							HaveField("Reason", "RequestDenied"),
							HaveField("Message", And(
								ContainSubstring("Denying kubelet server certificate CSR"),
								ContainSubstring(expectedReason),
							)),
						)))
					}).Should(Succeed())
				})
			}

			Context("username not prefixed with system:node:", func() {
				BeforeEach(func() {
					clientWithAdminUsername, err := client.New(restConfig, client.Options{Scheme: resourcemanagerclient.CombinedScheme})
					Expect(err).NotTo(HaveOccurred())

					DeferCleanup(test.WithVar(&testClientKubelet, clientWithAdminUsername))
				})

				runTest("is not prefixed with")
			})

			Context("SANs don't contain any DNS names or IP addresses", func() {
				BeforeEach(func() {
					dnsNames, ips = nil, nil
				})

				runTest("no DNS names or IP addresses in the SANs found")
			})

			Context("common name does not match username", func() {
				BeforeEach(func() {
					certificateSubject.CommonName = "some-other-name"
				})

				runTest("common name in CSR does not match username")
			})

			Context("organization is empty", func() {
				BeforeEach(func() {
					certificateSubject.Organization = nil
				})

				runTest("organization in CSR does not match nodes group")
			})

			Context("organization contains too many items", func() {
				BeforeEach(func() {
					certificateSubject.Organization = []string{"foo", "bar"}
				})

				runTest("organization in CSR does not match nodes group")
			})

			Context("organization does not contain nodes group", func() {
				BeforeEach(func() {
					certificateSubject.Organization = []string{"foo"}
				})

				runTest("organization in CSR does not match nodes group")
			})

			Context("node object not found", func() {
				runTest("could not find node object with name")
			})

			Context("machine object not found", func() {
				BeforeEach(func() {
					createNode(node)
				})

				runTest("Expected exactly one machine in namespace")
			})

			Context("too many machine objects found", func() {
				BeforeEach(func() {
					machine2 := machine.DeepCopy()
					machine2.Name = machine2.Name + "-2"

					createNode(node)
					createMachine(machine, true)
					createMachine(machine2, true)
				})

				runTest("Expected exactly one machine in namespace")
			})

			Context("DNS names do not match addresses", func() {
				BeforeEach(func() {
					createNode(node)
					createMachine(machine, true)
				})

				runTest("DNS names in CSR do not match addresses of type 'Hostname' or 'InternalDNS' or 'ExternalDNS' in node object")
			})

			Context("IP addresses do not match addresses", func() {
				BeforeEach(func() {
					createNode(node)
					createMachine(machine, true)
					patchNodeAddresses(node,
						corev1.NodeAddress{Type: corev1.NodeHostName, Address: dnsName1},
						corev1.NodeAddress{Type: corev1.NodeHostName, Address: dnsName2},
						corev1.NodeAddress{Type: corev1.NodeInternalDNS, Address: dnsName3},
						corev1.NodeAddress{Type: corev1.NodeExternalDNS, Address: dnsName4},
					)
				})

				runTest("IP addresses in CSR do not match addresses of type 'InternalIP' or 'ExternalIP' in node object")
			})
		})
	})

	Context("gardener-node-agent client certificate", func() {
		BeforeEach(func() {
			csr.Spec.SignerName = certificatesv1.KubeAPIServerClientSignerName
			certificateSubject = &pkix.Name{
				CommonName: userNameNodeAgent,
			}
		})

		Context("gardener-node-agent user", func() {
			JustBeforeEach(func() {
				By("Create CertificateSigningRequest")
				Expect(testClientNodeAgent.Create(ctx, csr)).To(Succeed())
				log.Info("Created CertificateSigningRequest for test", "certificateSigningRequest", client.ObjectKeyFromObject(csr))

				DeferCleanup(func() {
					By("Delete CertificateSigningRequest")
					Expect(client.IgnoreNotFound(testClientNodeAgent.Delete(ctx, csr))).To(Succeed())
				})
			})

			Context("constraints fulfilled", func() {
				BeforeEach(func() {
					createMachine(machine, false)
				})

				It("should approve the CSR", func() {
					Eventually(func(g Gomega) {
						g.Expect(testClientNodeAgent.Get(ctx, client.ObjectKeyFromObject(csr), csr)).To(Succeed())
						g.Expect(csr.Status.Conditions).To(ContainElement(And(
							HaveField("Type", certificatesv1.CertificateApproved),
							HaveField("Reason", "RequestApproved"),
							HaveField("Message", "Approving gardener-node-agent certificate CSR (all checks passed)"),
						)))
					}).Should(Succeed())
				})
			})

			Context("constraints violated", func() {
				Context("no machine", func() {
					It("should deny the CSR", func() {
						runDenyNodeAgentCSRTest(testClientNodeAgent, csr, "machine %q does not exist", &machineName)
					})
				})

				Context("a certificate for a different machine is requested", func() {
					var otherMachineName string

					BeforeEach(func() {
						machine2 := machine.DeepCopy()
						machine2.Name = "foo-bar-machine"
						otherMachineName = "gardener.cloud:node-agent:machine:" + machine2.Name
						certificateSubject = &pkix.Name{
							CommonName: otherMachineName,
						}
						createMachine(machine, false)
						createMachine(machine2, false)
					})

					It("should deny the CSR", func() {
						runDenyNodeAgentCSRTest(testClientNodeAgent, csr, "username %q and commonName %q do not match", &userNameNodeAgent, &otherMachineName)
					})
				})
			})
		})

		Context("bootstrap user", func() {
			JustBeforeEach(func() {
				By("Create CertificateSigningRequest")
				Expect(testClientBootstrap.Create(ctx, csr)).To(Succeed())
				log.Info("Created CertificateSigningRequest for test", "certificateSigningRequest", client.ObjectKeyFromObject(csr))

				DeferCleanup(func() {
					By("Delete CertificateSigningRequest")
					Expect(client.IgnoreNotFound(testClientBootstrap.Delete(ctx, csr))).To(Succeed())
				})
			})

			Context("constraints fulfilled - machine without node label", func() {
				BeforeEach(func() {
					createMachine(machine, false)
				})

				It("should approve the CSR", func() {
					Eventually(func(g Gomega) {
						g.Expect(testClientBootstrap.Get(ctx, client.ObjectKeyFromObject(csr), csr)).To(Succeed())
						g.Expect(csr.Status.Conditions).To(ContainElement(And(
							HaveField("Type", certificatesv1.CertificateApproved),
							HaveField("Reason", "RequestApproved"),
							HaveField("Message", "Approving gardener-node-agent certificate CSR (all checks passed)"),
						)))
					}).Should(Succeed())
				})
			})

			Context("constraints fulfilled - machine with node label but node is not created yet", func() {
				BeforeEach(func() {
					createMachine(machine, true)
				})

				It("should approve the CSR", func() {
					Eventually(func(g Gomega) {
						g.Expect(testClientBootstrap.Get(ctx, client.ObjectKeyFromObject(csr), csr)).To(Succeed())
						g.Expect(csr.Status.Conditions).To(ContainElement(And(
							HaveField("Type", certificatesv1.CertificateApproved),
							HaveField("Reason", "RequestApproved"),
							HaveField("Message", "Approving gardener-node-agent certificate CSR (all checks passed)"),
						)))
					}).Should(Succeed())
				})
			})

			Context("constraints violated", func() {
				Context("no machine", func() {
					It("should deny the CSR", func() {
						runDenyNodeAgentCSRTest(testClientBootstrap, csr, "machine %q does not exist", &machineName)
					})
				})

				Context("node already registered", func() {
					BeforeEach(func() {
						createNode(node)
						createMachine(machine, true)
					})

					It("should deny the CSR", func() {
						runDenyNodeAgentCSRTest(testClientBootstrap, csr, "Cannot use bootstrap token since gardener-node-agent for machine %q is already bootstrapped", &machineName)
					})
				})
			})
		})

		// TODO(oliver-goetz): remove this context when NodeAgentAuthorizer feature gate is removed. Remove "testClientNodeAgentSA" and "userNameNodeAgentSA" from test suite too.
		Context("gardener-node-agent service account", func() {
			JustBeforeEach(func() {
				By("Create CertificateSigningRequest")
				Expect(testClientNodeAgentSA.Create(ctx, csr)).To(Succeed())
				log.Info("Created CertificateSigningRequest for test", "certificateSigningRequest", client.ObjectKeyFromObject(csr))

				DeferCleanup(func() {
					By("Delete CertificateSigningRequest")
					Expect(client.IgnoreNotFound(testClientNodeAgentSA.Delete(ctx, csr))).To(Succeed())
				})
			})

			Context("constraints fulfilled", func() {
				BeforeEach(func() {
					createNode(node)
					createMachine(machine, true)
				})

				It("should approve the CSR", func() {
					Eventually(func(g Gomega) {
						g.Expect(testClientBootstrap.Get(ctx, client.ObjectKeyFromObject(csr), csr)).To(Succeed())
						g.Expect(csr.Status.Conditions).To(ContainElement(And(
							HaveField("Type", certificatesv1.CertificateApproved),
							HaveField("Reason", "RequestApproved"),
							HaveField("Message", "Approving gardener-node-agent certificate CSR (all checks passed)"),
						)))
					}).Should(Succeed())
				})
			})

			Context("constraints violated", func() {
				Context("no machine", func() {
					It("should deny the CSR", func() {
						runDenyNodeAgentCSRTest(testClientBootstrap, csr, "machine %q does not exist", &machineName)
					})
				})

				Context("node not registered yet", func() {
					BeforeEach(func() {
						createMachine(machine, false)
					})

					It("should deny the CSR", func() {
						runDenyNodeAgentCSRTest(testClientBootstrap, csr, "gardener-node-agent service account is allowed to create CSRs for machines with existing nodes only")
					})
				})
			})
		})

		Context("kubelet user tries to get gardener-node-agent certificate", func() {
			BeforeEach(func() {
				createNode(node)
				createMachine(machine, true)
				patchNodeAddresses(node,
					corev1.NodeAddress{Type: corev1.NodeHostName, Address: dnsName1},
					corev1.NodeAddress{Type: corev1.NodeHostName, Address: dnsName2},
					corev1.NodeAddress{Type: corev1.NodeInternalDNS, Address: dnsName3},
					corev1.NodeAddress{Type: corev1.NodeExternalDNS, Address: dnsName4},
					corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: ip1},
					corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: ip2},
					corev1.NodeAddress{Type: corev1.NodeExternalIP, Address: ip3},
				)
			})

			JustBeforeEach(func() {
				By("Create CertificateSigningRequest")
				Expect(testClientKubelet.Create(ctx, csr)).To(Succeed())
				log.Info("Created CertificateSigningRequest for test", "certificateSigningRequest", client.ObjectKeyFromObject(csr))

				DeferCleanup(func() {
					By("Delete CertificateSigningRequest")
					Expect(client.IgnoreNotFound(testClientKubelet.Delete(ctx, csr))).To(Succeed())
				})
			})

			It("should deny the CSR", func() {
				runDenyNodeAgentCSRTest(testClientBootstrap, csr, "username %q is not allowed to create CSRs for a gardener-node-agent", &userNameKubelet)
			})
		})
	})
})

func createNode(node *corev1.Node) {
	By("Create Node")
	ExpectWithOffset(1, mgrClient.Create(ctx, node)).To(Succeed())
	log.Info("Created Node for test", "node", client.ObjectKeyFromObject(node))

	By("Wait until manager has observed Node")
	EventuallyWithOffset(1, func() error {
		return mgrClient.Get(ctx, client.ObjectKeyFromObject(node), node)
	}).Should(Succeed())

	DeferCleanup(func() {
		By("Delete Node")
		ExpectWithOffset(1, client.IgnoreNotFound(mgrClient.Delete(ctx, node))).To(Succeed())

		By("Wait until manager has observed Node deletion")
		EventuallyWithOffset(1, func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(node), node)
		}).Should(BeNotFoundError())
	})
}

func createMachine(machine *machinev1alpha1.Machine, withNodeLabel bool) {
	By("Create Machine")
	if withNodeLabel {
		machine.Labels["node"] = nodeName
	}
	ExpectWithOffset(1, mgrClient.Create(ctx, machine)).To(Succeed())
	log.Info("Created Machine for test", "machine", client.ObjectKeyFromObject(machine))

	By("Wait until manager has observed Machine")
	EventuallyWithOffset(1, func() error {
		return mgrClient.Get(ctx, client.ObjectKeyFromObject(machine), machine)
	}).Should(Succeed())

	DeferCleanup(func() {
		By("Delete Machine")
		ExpectWithOffset(1, client.IgnoreNotFound(mgrClient.Delete(ctx, machine))).To(Succeed())

		By("Wait until manager has observed Machine deletion")
		EventuallyWithOffset(1, func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(machine), machine)
		}).Should(BeNotFoundError())
	})
}

func patchNodeAddresses(node *corev1.Node, addresses ...corev1.NodeAddress) {
	By("Patch node's addresses in status")
	patch := client.MergeFrom(node.DeepCopy())
	node.Status.Addresses = addresses
	ExpectWithOffset(1, mgrClient.Status().Patch(ctx, node, patch)).To(Succeed())

	By("Wait until manager has observed node status")
	EventuallyWithOffset(1, func(g Gomega) []corev1.NodeAddress {
		g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
		return node.Status.Addresses
	}).ShouldNot(BeEmpty())
}

func runDenyNodeAgentCSRTest(c client.Client, csr *certificatesv1.CertificateSigningRequest, expectedReason string, argPtrs ...*string) {
	var args []interface{}
	for _, arg := range argPtrs {
		args = append(args, *arg)
	}
	EventuallyWithOffset(1, func(g Gomega) {
		g.Expect(c.Get(ctx, client.ObjectKeyFromObject(csr), csr)).To(Succeed())
		g.Expect(csr.Status.Conditions).To(ContainElement(And(
			HaveField("Type", certificatesv1.CertificateDenied),
			HaveField("Reason", "RequestDenied"),
			HaveField("Message", And(
				ContainSubstring("Denying gardener-node-agent certificate CSR"),
				ContainSubstring(expectedReason, args...),
			)),
		)))
	}).Should(Succeed())
}

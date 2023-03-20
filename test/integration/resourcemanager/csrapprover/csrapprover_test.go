// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

var _ = Describe("Kubelet Server CertificateSigningRequest Approver Controller tests", func() {
	var (
		privateKey         *rsa.PrivateKey
		certificateSubject *pkix.Name

		ip1      = "1.2.3.4"
		ip2      = "5.6.7.8"
		ip3      = "9.0.1.2"
		ips      []net.IP
		dnsName1 = "foo.bar"
		dnsName2 = "bar.baz"
		dnsName3 = "baz.foo"
		dnsName4 = "baz.bar"
		dnsNames []string

		csr     *certificatesv1.CertificateSigningRequest
		node    *corev1.Node
		machine *machinev1alpha1.Machine
	)

	BeforeEach(func() {
		privateKey, _ = secretsutils.FakeGenerateKey(rand.Reader, 4096)
		certificateSubject = &pkix.Name{
			CommonName:   userName,
			Organization: []string{user.NodesGroup},
		}

		ips = []net.IP{net.ParseIP(ip1), net.ParseIP(ip2), net.ParseIP(ip3)}
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
				SignerName: certificatesv1.KubeletServingSignerName,
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
				GenerateName: "machine-",
				Namespace:    testNamespace.Name,
				Labels: map[string]string{
					testID: testRunID,
					"node": node.Name,
				},
			},
		}
	})

	JustBeforeEach(func() {
		By("Generate CSR data")
		csrData, err := certutil.MakeCSR(privateKey, certificateSubject, dnsNames, ips)
		Expect(err).NotTo(HaveOccurred())
		csr.Spec.Request = csrData

		By("Create CertificateSigningRequest")
		Expect(testClient.Create(ctx, csr)).To(Succeed())
		log.Info("Created CertificateSigningRequest for test", "certificateSigningRequest", client.ObjectKeyFromObject(csr))

		DeferCleanup(func() {
			By("Delete CertificateSigningRequest")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, csr))).To(Succeed())
		})
	})

	Context("non kubelet server certificate", func() {
		BeforeEach(func() {
			csr.Spec.SignerName = certificatesv1.KubeAPIServerClientSignerName
		})

		It("should ignore the CSR and do nothing", func() {
			Consistently(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(csr), csr)).To(Succeed())
				g.Expect(csr.Status.Conditions).To(BeEmpty())
			}).Should(Succeed())
		})
	})

	Context("kubelet server certificate", func() {
		Context("constraints fulfilled", func() {
			BeforeEach(func() {
				createNode(node)
				createMachine(machine)
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

			It("should approve the CSR", func() {
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(csr), csr)).To(Succeed())
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
						g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(csr), csr)).To(Succeed())
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

					DeferCleanup(test.WithVar(&testClient, clientWithAdminUsername))
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

					createNode(node)
					createMachine(machine)
					createMachine(machine2)
				})

				runTest("Expected exactly one machine in namespace")
			})

			Context("DNS names do not match addresses", func() {
				BeforeEach(func() {
					createNode(node)
					createMachine(machine)
				})

				runTest("DNS names in CSR do not match addresses of type 'Hostname' or 'InternalDNS' or 'ExternalDNS' in node object")
			})

			Context("IP addresses do not match addresses", func() {
				BeforeEach(func() {
					createNode(node)
					createMachine(machine)
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
})

func createNode(node *corev1.Node) {
	By("Create Node")
	ExpectWithOffset(1, testClient.Create(ctx, node)).To(Succeed())
	log.Info("Created Node for test", "node", client.ObjectKeyFromObject(node))

	By("Wait until manager has observed Node")
	EventuallyWithOffset(1, func() error {
		return mgrClient.Get(ctx, client.ObjectKeyFromObject(node), node)
	}).Should(Succeed())

	DeferCleanup(func() {
		By("Delete Node")
		ExpectWithOffset(1, client.IgnoreNotFound(testClient.Delete(ctx, node))).To(Succeed())

		By("Wait until manager has observed Node deletion")
		EventuallyWithOffset(1, func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(node), node)
		}).Should(BeNotFoundError())
	})
}

func createMachine(machine *machinev1alpha1.Machine) {
	By("Create Machine")
	ExpectWithOffset(1, testClient.Create(ctx, machine)).To(Succeed())
	log.Info("Created Machine for test", "machine", client.ObjectKeyFromObject(machine))

	By("Wait until manager has observed Machine")
	EventuallyWithOffset(1, func() error {
		return mgrClient.Get(ctx, client.ObjectKeyFromObject(machine), machine)
	}).Should(Succeed())

	DeferCleanup(func() {
		By("Delete Machine")
		ExpectWithOffset(1, client.IgnoreNotFound(testClient.Delete(ctx, machine))).To(Succeed())

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
	ExpectWithOffset(1, testClient.Status().Patch(ctx, node, patch)).To(Succeed())

	By("Wait until manager has observed node status")
	EventuallyWithOffset(1, func(g Gomega) []corev1.NodeAddress {
		g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
		return node.Status.Addresses
	}).ShouldNot(BeEmpty())
}

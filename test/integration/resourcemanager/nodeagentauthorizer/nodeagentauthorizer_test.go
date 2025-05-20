// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeagentauthorizer_test

import (
	"context"
	"crypto/rand"
	"crypto/x509/pkix"
	"fmt"
	"net/http"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	certificatesv1 "k8s.io/api/certificates/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/nodeagentauthorizer"
	"github.com/gardener/gardener/pkg/utils"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("NodeAgentAuthorizer tests", func() {
	const (
		machineSecretName      = "foo-machine-secret"
		otherMachineName       = "bar-machine"
		otherMachineSecretName = "bar-machine-secret"
		valitailSecretName     = "gardener-valitail"
	)

	var (
		testRestConfigNodeAgent *rest.Config
		testClientNodeAgent     client.Client

		machineNamespace *string
		machine          *machinev1alpha1.Machine
		otherMachine     *machinev1alpha1.Machine
		node             *corev1.Node
	)

	runTests := func() {
		BeforeEach(func() {
			By("Setup manager")
			mgr, err := manager.New(testRestConfig, manager.Options{
				WebhookServer: webhook.NewServer(webhook.Options{
					Port:    testEnv.WebhookInstallOptions.LocalServingPort,
					Host:    testEnv.WebhookInstallOptions.LocalServingHost,
					CertDir: testEnv.WebhookInstallOptions.LocalServingCertDir,
				}),
				Metrics: metricsserver.Options{BindAddress: "0"},
				Cache: cache.Options{
					DefaultNamespaces: map[string]cache.Config{testNamespace.Name: {}},
				},
				Controller: controllerconfig.Controller{
					SkipNameValidation: ptr.To(true),
				},
			})
			Expect(err).NotTo(HaveOccurred())

			By("Register webhook")
			nodeAgentAuthorizer := &nodeagentauthorizer.Webhook{
				Logger: log,
				Config: resourcemanagerconfigv1alpha1.NodeAgentAuthorizerWebhookConfig{
					Enabled:          true,
					MachineNamespace: machineNamespace,
				},
			}
			Expect(nodeAgentAuthorizer.AddToManager(mgr, testClient, testClient)).To(Succeed())

			By("Start manager")
			mgrContext, mgrCancel := context.WithCancel(ctx)

			go func() {
				defer GinkgoRecover()
				Expect(mgr.Start(mgrContext)).To(Succeed())
			}()

			// Wait for the webhook server to start
			Eventually(func() error {
				checker := mgr.GetWebhookServer().StartedChecker()
				return checker(&http.Request{})
			}).Should(Succeed())

			DeferCleanup(func() {
				By("Stop manager")
				mgrCancel()
			})
		})

		Describe("#CertificateSigningRequests", func() {
			var (
				csr *certificatesv1.CertificateSigningRequest
			)

			BeforeEach(func() {
				certificateSubject := &pkix.Name{
					CommonName: "nodeagent-authorizer-test",
				}
				privateKey, err := secretsutils.FakeGenerateKey(rand.Reader, 4096)
				ExpectWithOffset(1, err).ToNot(HaveOccurred())
				csrData, err := certutil.MakeCSR(privateKey, certificateSubject, nil, nil)
				ExpectWithOffset(1, err).ToNot(HaveOccurred())

				csr = &certificatesv1.CertificateSigningRequest{
					ObjectMeta: metav1.ObjectMeta{Name: "foo-request"},
					Spec: certificatesv1.CertificateSigningRequestSpec{
						Usages: []certificatesv1.KeyUsage{
							certificatesv1.UsageDigitalSignature,
							certificatesv1.UsageKeyEncipherment,
							certificatesv1.UsageClientAuth,
						},
						Request:    csrData,
						SignerName: certificatesv1.KubeAPIServerClientSignerName,
					},
				}

				DeferCleanup(func() {
					By("Delete CertificateSigningRequest")
					ExpectWithOffset(1, testClient.Delete(ctx, csr)).To(Or(Succeed(), BeNotFoundError()))
				})
			})

			It("should be able to create a CertificateSigningRequest, get it but not change or delete it", func() {
				ExpectWithOffset(1, testClientNodeAgent.Create(ctx, csr)).To(Succeed())

				ExpectWithOffset(1, testClientNodeAgent.Get(ctx, client.ObjectKeyFromObject(csr), csr)).To(Succeed())

				csrUpdate := csr.DeepCopy()
				csrUpdate.SetLabels(map[string]string{"foo": "bar"})
				ExpectWithOffset(1, testClientNodeAgent.Update(ctx, csrUpdate)).To(BeForbiddenError())

				patch := client.MergeFrom(csr)
				csrPatch := csr.DeepCopy()
				csrPatch.SetLabels(map[string]string{"foo": "bar"})
				ExpectWithOffset(1, testClientNodeAgent.Patch(ctx, csrPatch, patch)).To(BeForbiddenError())

				ExpectWithOffset(1, testClientNodeAgent.Delete(ctx, csr)).To(BeForbiddenError())
			})

			It("should forbid to access a CertificateSigningRequest created by a different user", func() {
				ExpectWithOffset(1, testClient.Create(ctx, csr)).To(Succeed())

				ExpectWithOffset(1, testClientNodeAgent.Get(ctx, client.ObjectKeyFromObject(csr), csr)).To(BeForbiddenError())

				csrUpdate := csr.DeepCopy()
				csrUpdate.SetLabels(map[string]string{"foo": "bar"})
				ExpectWithOffset(1, testClientNodeAgent.Update(ctx, csrUpdate)).To(BeForbiddenError())

				patch := client.MergeFrom(csr)
				csrPatch := csr.DeepCopy()
				csrPatch.SetLabels(map[string]string{"foo": "bar"})
				ExpectWithOffset(1, testClientNodeAgent.Patch(ctx, csrPatch, patch)).To(BeForbiddenError())

				ExpectWithOffset(1, testClientNodeAgent.Delete(ctx, csr)).To(BeForbiddenError())
			})

			It("should forbid to list CertificateSigningRequests", func() {
				ExpectWithOffset(1, testClientNodeAgent.List(ctx, &certificatesv1.CertificateSigningRequestList{})).To(BeForbiddenError())
			})
		})

		Describe("#Events", func() {
			var (
				recorder record.EventRecorder
			)

			BeforeEach(func() {
				corev1Client, err := corev1client.NewForConfig(testRestConfigNodeAgent)
				ExpectWithOffset(1, err).ToNot(HaveOccurred())
				broadcaster := record.NewBroadcaster()
				broadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: corev1Client.Events("")})
				recorder = broadcaster.NewRecorder(testClientNodeAgent.Scheme(), corev1.EventSource{Component: "nodeagentauthorizer-test"})

				node.Name = fmt.Sprintf("event-node-%s", utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:16])
				createNode(node, machine)
			})

			It("should be able to create an event", func() {
				recorder.Event(node, "Normal", "test-reason", "test-message")

				Eventually(func(g Gomega) {
					eventList := &corev1.EventList{}
					g.ExpectWithOffset(1, testClient.List(ctx, eventList, client.MatchingFields{"involvedObject.name": node.Name})).To(Succeed())
					g.ExpectWithOffset(1, eventList.Items).To(HaveLen(1))
					g.ExpectWithOffset(1, eventList.Items[0].Type).To(Equal("Normal"))
					g.ExpectWithOffset(1, eventList.Items[0].Reason).To(Equal("test-reason"))
					g.ExpectWithOffset(1, eventList.Items[0].Message).To(Equal("test-message"))
					g.ExpectWithOffset(1, eventList.Items[0].Count).To(Equal(int32(1)))
				}).Should(Succeed())
			})

			It("should be able to create and patch an event", func() {
				// Recording the same event multiple times in a short period makes the event recorder patching the event.
				for range 3 {
					recorder.Event(node, "Normal", "test-reason", "test-message")
				}

				Eventually(func(g Gomega) {
					eventList := &corev1.EventList{}
					g.ExpectWithOffset(1, testClient.List(ctx, eventList, client.MatchingFields{"involvedObject.name": node.Name})).To(Succeed())
					g.ExpectWithOffset(1, eventList.Items).To(HaveLen(1))
					g.ExpectWithOffset(1, eventList.Items[0].Type).To(Equal("Normal"))
					g.ExpectWithOffset(1, eventList.Items[0].Reason).To(Equal("test-reason"))
					g.ExpectWithOffset(1, eventList.Items[0].Message).To(Equal("test-message"))
					g.ExpectWithOffset(1, eventList.Items[0].Count).To(Equal(int32(3)))
				}).Should(Succeed())
			})

			It("should forbid to list events", func() {
				ExpectWithOffset(1, testClientNodeAgent.List(ctx, &corev1.EventList{})).To(BeForbiddenError())
			})

			It("should forbid to update an event", func() {
				ExpectWithOffset(1, testClientNodeAgent.Update(ctx, &corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "foo-event", Namespace: "default"}})).To(BeForbiddenError())
			})

			It("should forbid to delete an event", func() {
				ExpectWithOffset(1, testClientNodeAgent.Delete(ctx, &corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "foo-event", Namespace: "default"}})).To(BeForbiddenError())
			})
		})

		Describe("#Leases", func() {
			const (
				otherNodeName           = "bar-node"
				nodeAgentLeaseName      = "gardener-node-agent-" + nodeName
				otherNodeAgentLeaseName = "gardener-node-agent-" + otherNodeName
			)

			var otherNode *corev1.Node

			BeforeEach(func() {
				otherNode = &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: otherNodeName,
					},
				}
				if machineNamespace != nil {
					otherNode.Labels = map[string]string{"node.gardener.cloud/machine-name": otherMachineName}
				}
				DeferCleanup(func() {
					By("Delete other Node")
					ExpectWithOffset(1, testClient.Delete(ctx, otherNode)).To(Or(Succeed(), BeNotFoundError()))
				})
				createNode(otherNode, otherMachine)
			})

			DescribeTable("#get",
				func(name, namespace string, withNode, allow bool) {
					matcher := BeForbiddenError()
					if allow {
						matcher = Succeed()
					}
					if withNode {
						createNode(node, machine)
					}
					lease := &coordinationv1.Lease{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: namespace,
						},
					}
					ExpectWithOffset(1, testClient.Create(ctx, lease)).To(Succeed())
					DeferCleanup(func() {
						ExpectWithOffset(1, testClient.Delete(ctx, lease)).To(Or(Succeed(), BeNotFoundError()))
					})
					ExpectWithOffset(1, testClientNodeAgent.Get(ctx, client.ObjectKeyFromObject(lease), &coordinationv1.Lease{})).To(matcher)
				},
				Entry("allow own gardener-node-agent", nodeAgentLeaseName, "kube-system", true, true),
				Entry("forbid if no own node", nodeAgentLeaseName, "kube-system", false, false),
				Entry("forbid other gardener-node-agent", otherNodeAgentLeaseName, "kube-system", true, false),
				Entry("forbid other gardener-node-agent if no own node", otherNodeAgentLeaseName, "kube-system", false, false),
				Entry("forbid in default namespace", "foo-bar", "default", true, false),
				Entry("forbid in default namespace without node", "foo-bar", "default", false, false),
			)

			DescribeTable("#update",
				func(name, namespace string, withNode, allow bool) {
					matcher := BeForbiddenError()
					if allow {
						matcher = Succeed()
					}
					if withNode {
						createNode(node, machine)
					}
					lease := &coordinationv1.Lease{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: namespace,
						},
					}
					ExpectWithOffset(1, testClient.Create(ctx, lease)).To(Succeed())
					DeferCleanup(func() {
						ExpectWithOffset(1, testClient.Delete(ctx, lease)).To(Or(Succeed(), BeNotFoundError()))
					})
					lease.SetLabels(map[string]string{"foo": "bar"})
					ExpectWithOffset(1, testClientNodeAgent.Update(ctx, lease.DeepCopy())).To(matcher)
				},
				Entry("allow own gardener-node-agent", nodeAgentLeaseName, "kube-system", true, true),
				Entry("forbid if no own node", nodeAgentLeaseName, "kube-system", false, false),
				Entry("forbid other gardener-node-agent", otherNodeAgentLeaseName, "kube-system", true, false),
				Entry("forbid other gardener-node-agent if no own node", otherNodeAgentLeaseName, "kube-system", false, false),
				Entry("forbid in default namespace", "foo-bar", "default", true, false),
				Entry("forbid in default namespace without node", "foo-bar", "default", false, false),
			)

			DescribeTable("#patch",
				func(name, namespace string, withNode bool) {
					if withNode {
						createNode(node, machine)
					}
					lease := &coordinationv1.Lease{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: namespace,
						},
					}
					ExpectWithOffset(1, testClient.Create(ctx, lease)).To(Succeed())
					DeferCleanup(func() {
						ExpectWithOffset(1, testClient.Delete(ctx, lease)).To(Or(Succeed(), BeNotFoundError()))
					})
					patch := client.MergeFrom(lease)
					lease.SetLabels(map[string]string{"foo": "bar"})
					ExpectWithOffset(1, testClientNodeAgent.Patch(ctx, lease.DeepCopy(), patch)).To(BeForbiddenError())
				},
				Entry("forbid own gardener-node-agent", nodeAgentLeaseName, "kube-system", true),
				Entry("forbid if no own node", nodeAgentLeaseName, "kube-system", false),
				Entry("forbid other gardener-node-agent", otherNodeAgentLeaseName, "kube-system", true),
				Entry("forbid other gardener-node-agent if no own node", otherNodeAgentLeaseName, "kube-system", false),
				Entry("forbid in default namespace", "foo-bar", "default", true),
				Entry("forbid in default namespace without node", "foo-bar", "default", false),
			)

			DescribeTable("#delete",
				func(name, namespace string, withNode bool) {
					if withNode {
						createNode(node, machine)
					}
					lease := &coordinationv1.Lease{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: namespace,
						},
					}
					ExpectWithOffset(1, testClient.Create(ctx, lease)).To(Succeed())
					DeferCleanup(func() {
						ExpectWithOffset(1, testClient.Delete(ctx, lease)).To(Or(Succeed(), BeNotFoundError()))
					})
					ExpectWithOffset(1, testClientNodeAgent.Delete(ctx, lease.DeepCopy())).To(BeForbiddenError())
				},
				Entry("forbid own gardener-node-agent", nodeAgentLeaseName, "kube-system", true),
				Entry("forbid if no own node", nodeAgentLeaseName, "kube-system", false),
				Entry("forbid other gardener-node-agent", otherNodeAgentLeaseName, "kube-system", true),
				Entry("forbid other gardener-node-agent if no own node", otherNodeAgentLeaseName, "kube-system", false),
				Entry("forbid in default namespace", "foo-bar", "default", true),
				Entry("forbid in default namespace without node", "foo-bar", "default", false),
			)

			DescribeTable("#list",
				func(name, namespace string, withNode, allow bool) {
					matcher := BeForbiddenError()
					if allow {
						matcher = Succeed()
					}
					if withNode {
						createNode(node, machine)
					}
					var listOptions []client.ListOption
					if name != "" {
						listOptions = append(listOptions, client.MatchingFields{"metadata.name": name})
					}
					if namespace != "" {
						listOptions = append(listOptions, client.InNamespace(namespace))
					}
					ExpectWithOffset(1, testClientNodeAgent.List(ctx, &coordinationv1.LeaseList{}, listOptions...)).To(matcher)
				},
				Entry("allow own gardener-node-agent", nodeAgentLeaseName, "kube-system", true, true),
				Entry("forbid if no own node", nodeAgentLeaseName, "kube-system", false, false),
				Entry("forbid other gardener-node-agent", otherNodeAgentLeaseName, "kube-system", true, false),
				Entry("forbid other gardener-node-agent if no own node", otherNodeAgentLeaseName, "kube-system", false, false),
				Entry("forbid in default namespace", "foo-bar", "default", true, false),
				Entry("forbid in default namespace without node", "foo-bar", "default", false, false),
				Entry("forbid whole kube-system namespace", "", "kube-system", true, false),
				Entry("forbid whole kube-system namespace if no own node", "", "kube-system", false, false),
				Entry("forbid cluster wide", "", "", true, false),
				Entry("forbid cluster wide if no own node", "", "", false, false),
			)

			DescribeTable("#create",
				func(name, namespace string, withNode, allow bool) {
					matcher := BeForbiddenError()
					if allow {
						matcher = Succeed()
					}
					if withNode {
						createNode(node, machine)
					}
					lease := &coordinationv1.Lease{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: namespace,
						},
					}
					ExpectWithOffset(1, testClientNodeAgent.Create(ctx, lease)).To(matcher)
					DeferCleanup(func() {
						ExpectWithOffset(1, testClient.Delete(ctx, lease)).To(Or(Succeed(), BeNotFoundError()))
					})
				},
				Entry("allow own gardener-node-agent", nodeAgentLeaseName, "kube-system", true, true),
				Entry("forbid if no own node", nodeAgentLeaseName, "kube-system", false, false),
				Entry("allow other gardener-node-agent - we cannot avoid this", otherNodeAgentLeaseName, "kube-system", true, true),
				Entry("forbid other gardener-node-agent if no own node", otherNodeAgentLeaseName, "kube-system", false, false),
				Entry("forbid in default namespace", "foo-bar", "default", true, false),
				Entry("forbid in default namespace without node", "foo-bar", "default", false, false),
			)
		})

		Describe("#Nodes", func() {
			const otherNodeName = "bar-node"

			var otherNode *corev1.Node

			BeforeEach(func() {
				otherNode = &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name:   otherNodeName,
						Labels: map[string]string{},
					},
				}
				if machineNamespace != nil {
					otherNode.Labels = map[string]string{"node.gardener.cloud/machine-name": otherMachineName}
				}
				DeferCleanup(func() {
					By("Delete other Node")
					ExpectWithOffset(1, testClient.Delete(ctx, otherNode)).To(Or(Succeed(), BeNotFoundError()))
				})
				createNode(otherNode, otherMachine)
			})

			DescribeTable("#get",
				func(name string, withNode, allow bool) {
					matcher := BeForbiddenError()
					if allow {
						matcher = Succeed()
					}
					if withNode {
						createNode(node, machine)
					}
					ExpectWithOffset(1, testClientNodeAgent.Get(ctx, client.ObjectKey{Name: name}, &corev1.Node{})).To(matcher)
				},
				Entry("allow own node", nodeName, true, true),
				Entry("forbid if no own node", nodeName, false, false),
				Entry("forbid other node", otherNodeName, true, false),
				Entry("forbid other node if no own node", otherNodeName, false, false),
			)

			DescribeTable("#update",
				func(name string, allow bool) {
					matcher := BeForbiddenError()
					if allow {
						matcher = Succeed()
					}

					createNode(node, machine)

					testNode := &corev1.Node{}
					ExpectWithOffset(1, testClient.Get(ctx, client.ObjectKey{Name: name}, testNode)).To(Succeed())
					if testNode.Labels == nil {
						testNode.Labels = map[string]string{}
					}
					testNode.Labels["foo"] = "bar"
					ExpectWithOffset(1, testClientNodeAgent.Update(ctx, testNode)).To(matcher)
				},
				Entry("allow own node", nodeName, true),
				Entry("forbid other node", otherNodeName, false),
			)

			DescribeTable("#patch",
				func(name string, allow bool) {
					matcher := BeForbiddenError()
					if allow {
						matcher = Succeed()
					}

					createNode(node, machine)

					testNode := &corev1.Node{}
					ExpectWithOffset(1, testClient.Get(ctx, client.ObjectKey{Name: name}, testNode)).To(Succeed())
					patch := client.MergeFrom(testNode)
					if testNode.Labels == nil {
						testNode.Labels = map[string]string{}
					}
					testNode.Labels["foo"] = "bar"
					ExpectWithOffset(1, testClientNodeAgent.Patch(ctx, testNode, patch)).To(matcher)
				},
				Entry("allow own node", nodeName, true),
				Entry("forbid other node", otherNodeName, false),
			)

			DescribeTable("#delete",
				func(name string) {
					createNode(node, machine)

					testNode := &corev1.Node{}
					ExpectWithOffset(1, testClient.Get(ctx, client.ObjectKey{Name: name}, testNode)).To(Succeed())
					ExpectWithOffset(1, testClientNodeAgent.Delete(ctx, testNode)).To(BeForbiddenError())
				},
				Entry("forbid own node", nodeName),
				Entry("forbid other node", otherNodeName),
			)

			It("should allow listing nodes unconditionally", func() {
				ExpectWithOffset(1, testClientNodeAgent.List(ctx, &corev1.NodeList{})).To(Succeed())
			})
		})

		Describe("#Secrets", func() {
			BeforeEach(func() {
				if machineNamespace == nil {
					createNode(node, nil)
				}

				machineSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      machineSecretName,
						Namespace: "kube-system",
					},
				}
				ExpectWithOffset(1, testClient.Create(ctx, machineSecret)).To(Succeed())
				DeferCleanup(func() {
					By("Delete machine Secret")
					ExpectWithOffset(1, testClient.Delete(ctx, machineSecret)).To(Or(Succeed(), BeNotFoundError()))
				})

				otherMachineSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      otherMachineSecretName,
						Namespace: "kube-system",
					},
				}
				ExpectWithOffset(1, testClient.Create(ctx, otherMachineSecret)).To(Succeed())
				DeferCleanup(func() {
					By("Delete other machine Secret")
					ExpectWithOffset(1, testClient.Delete(ctx, otherMachineSecret)).To(Or(Succeed(), BeNotFoundError()))
				})

				valitailSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      valitailSecretName,
						Namespace: "kube-system",
					},
				}
				ExpectWithOffset(1, testClient.Create(ctx, valitailSecret)).To(Succeed())
				DeferCleanup(func() {
					By("Delete valitail Secret")
					ExpectWithOffset(1, testClient.Delete(ctx, valitailSecret)).To(Or(Succeed(), BeNotFoundError()))
				})

				fooSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "default",
					},
				}
				ExpectWithOffset(1, testClient.Create(ctx, fooSecret)).To(Succeed())
				DeferCleanup(func() {
					By("Delete foo Secret")
					ExpectWithOffset(1, testClient.Delete(ctx, fooSecret)).To(Or(Succeed(), BeNotFoundError()))
				})
			})

			DescribeTable("#get",
				func(name, namespace string, allow bool) {
					matcher := BeForbiddenError()
					if allow {
						matcher = Succeed()
					}
					ExpectWithOffset(1, testClientNodeAgent.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &corev1.Secret{})).To(matcher)
				},
				Entry("allow valitail secret", valitailSecretName, "kube-system", true),
				Entry("allow machine secret", machineSecretName, "kube-system", true),
				Entry("forbid foo secret", "foo", "default", false),
				Entry("forbid other machine secret", otherMachineSecretName, "kube-system", false),
			)

			DescribeTable("#update",
				func(name, namespace string) {
					secret := &corev1.Secret{}
					ExpectWithOffset(1, testClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, secret)).To(Succeed())
					secret.SetLabels(map[string]string{"foo": "bar"})
					ExpectWithOffset(1, testClientNodeAgent.Update(ctx, secret)).To(BeForbiddenError())
				},
				Entry("forbid valitail secret", valitailSecretName, "kube-system"),
				Entry("forbid machine secret", machineSecretName, "kube-system"),
				Entry("forbid foo secret", "foo", "default"),
				Entry("forbid other machine secret", otherMachineSecretName, "kube-system"),
			)

			DescribeTable("#patch",
				func(name, namespace string) {
					secret := &corev1.Secret{}
					ExpectWithOffset(1, testClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, secret)).To(Succeed())
					patch := client.MergeFrom(secret.DeepCopy())
					secret.SetLabels(map[string]string{"foo": "bar"})
					ExpectWithOffset(1, testClientNodeAgent.Patch(ctx, secret, patch)).To(BeForbiddenError())
				},
				Entry("forbid valitail secret", valitailSecretName, "kube-system"),
				Entry("forbid machine secret", machineSecretName, "kube-system"),
				Entry("forbid foo secret", "foo", "default"),
				Entry("forbid other machine secret", otherMachineSecretName, "kube-system"),
			)

			DescribeTable("#delete",
				func(name, namespace string) {
					secret := &corev1.Secret{}
					ExpectWithOffset(1, testClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, secret)).To(Succeed())
					ExpectWithOffset(1, testClientNodeAgent.Delete(ctx, secret)).To(BeForbiddenError())
				},
				Entry("forbid valitail secret", valitailSecretName, "kube-system"),
				Entry("forbid machine secret", machineSecretName, "kube-system"),
				Entry("forbid foo secret", "foo", "default"),
				Entry("forbid other machine secret", otherMachineSecretName, "kube-system"),
			)

			DescribeTable("#list",
				func(name, namespace string, allow bool) {
					matcher := BeForbiddenError()
					if allow {
						matcher = Succeed()
					}
					var listOptions []client.ListOption
					if name != "" {
						listOptions = append(listOptions, client.MatchingFields{"metadata.name": name})
					}
					if namespace != "" {
						listOptions = append(listOptions, client.InNamespace(namespace))
					}
					ExpectWithOffset(1, testClientNodeAgent.List(ctx, &corev1.SecretList{}, listOptions...)).To(matcher)
				},
				Entry("allow valitail secret", valitailSecretName, "kube-system", true),
				Entry("allow machine secret", machineSecretName, "kube-system", true),
				Entry("forbid default namespace", "", "default", false),
				Entry("forbid kube-system namespace", "", "kube-system", false),
				Entry("forbid cluster wide", "", "", false),
			)
		})
	}

	When("machine namespace is set", func() {
		BeforeEach(func() {
			testClientNodeAgent = testClientNodeAgentMachine
			testRestConfigNodeAgent = testRestConfigNodeAgentMachine

			machineNamespace = &testNamespace.Name

			machine = &machinev1alpha1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      machineName,
					Namespace: testNamespace.Name,
				},
				Spec: machinev1alpha1.MachineSpec{
					NodeTemplateSpec: machinev1alpha1.NodeTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName: machineSecretName},
						},
					},
				},
			}
			Expect(testClient.Create(ctx, machine)).To(Succeed())
			DeferCleanup(func() {
				By("Delete Machine")
				Expect(testClient.Delete(ctx, machine)).To(Or(Succeed(), BeNotFoundError()))
			})

			otherMachine = &machinev1alpha1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      otherMachineName,
					Namespace: testNamespace.Name,
				},
				Spec: machinev1alpha1.MachineSpec{
					NodeTemplateSpec: machinev1alpha1.NodeTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName: otherMachineSecretName},
						},
					},
				},
			}
			Expect(testClient.Create(ctx, otherMachine)).To(Succeed())
			DeferCleanup(func() {
				By("Delete other Machine")
				Expect(testClient.Delete(ctx, otherMachine)).To(Or(Succeed(), BeNotFoundError()))
			})

			node = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   nodeName,
					Labels: map[string]string{"node.gardener.cloud/machine-name": machine.Name},
				},
			}
			DeferCleanup(func() {
				By("Delete Node")
				Expect(testClient.Delete(ctx, node)).To(Or(Succeed(), BeNotFoundError()))
			})
		})

		runTests()
	})

	When("machine namespace is unset", func() {
		BeforeEach(func() {
			testClientNodeAgent = testClientNodeAgentNode
			testRestConfigNodeAgent = testRestConfigNodeAgentNode

			machineNamespace = nil
			machine = nil
			otherMachine = nil

			node = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   nodeName,
					Labels: map[string]string{v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName: machineSecretName},
				},
			}
			DeferCleanup(func() {
				By("Delete Node")
				Expect(testClient.Delete(ctx, node)).To(Or(Succeed(), BeNotFoundError()))
			})
		})

		runTests()
	})
})

func createNode(node *corev1.Node, machine *machinev1alpha1.Machine) {
	ExpectWithOffset(2, testClient.Create(ctx, node)).To(Succeed())
	if machine != nil {
		machine.SetLabels(map[string]string{machinev1alpha1.NodeLabelKey: node.Name})
		ExpectWithOffset(2, testClient.Update(ctx, machine)).To(Succeed())
	}
}

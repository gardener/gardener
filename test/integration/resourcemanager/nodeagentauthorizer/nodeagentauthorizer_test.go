// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeagentauthorizer_test

import (
	"crypto/rand"
	"crypto/x509/pkix"
	"fmt"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	certificatesv1 "k8s.io/api/certificates/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	certutil "k8s.io/client-go/util/cert"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("NodeAgentAuthorizer tests", func() {
	const (
		machineSecretName      = "foo-machine-secret"
		nodeName               = "foo-node"
		otherMachineName       = "bar-machine"
		otherMachineSecretName = "bar-machine-secret"
		valitailSecretName     = "gardener-valitail"
	)

	var (
		machine      *machinev1alpha1.Machine
		otherMachine *machinev1alpha1.Machine
		node         *corev1.Node
	)

	BeforeEach(func() {
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
				Labels: map[string]string{"node.gardener.cloud/machine-name": machineName},
			},
		}
		DeferCleanup(func() {
			By("Delete Node")
			Expect(testClient.Delete(ctx, node)).To(Or(Succeed(), BeNotFoundError()))
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
			Expect(err).ToNot(HaveOccurred())
			csrData, err := certutil.MakeCSR(privateKey, certificateSubject, nil, nil)
			Expect(err).ToNot(HaveOccurred())

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
				Expect(testClient.Delete(ctx, csr)).To(Or(Succeed(), BeNotFoundError()))
			})
		})

		It("should be able to create a CertificateSigningRequest, get it but not change or delete it", func() {
			Expect(testClientNodeAgent.Create(ctx, csr)).To(Succeed())

			Expect(testClientNodeAgent.Get(ctx, client.ObjectKeyFromObject(csr), csr)).To(Succeed())

			csrUpdate := csr.DeepCopy()
			csrUpdate.SetLabels(map[string]string{"foo": "bar"})
			Expect(testClientNodeAgent.Update(ctx, csrUpdate)).To(BeForbiddenError())

			patch := client.MergeFrom(csr)
			csrPatch := csr.DeepCopy()
			csrPatch.SetLabels(map[string]string{"foo": "bar"})
			Expect(testClientNodeAgent.Patch(ctx, csrPatch, patch)).To(BeForbiddenError())

			Expect(testClientNodeAgent.Delete(ctx, csr)).To(BeForbiddenError())
		})

		It("should forbid to access a CertificateSigningRequest created by a different user", func() {
			Expect(testClient.Create(ctx, csr)).To(Succeed())

			Expect(testClientNodeAgent.Get(ctx, client.ObjectKeyFromObject(csr), csr)).To(BeForbiddenError())

			csrUpdate := csr.DeepCopy()
			csrUpdate.SetLabels(map[string]string{"foo": "bar"})
			Expect(testClientNodeAgent.Update(ctx, csrUpdate)).To(BeForbiddenError())

			patch := client.MergeFrom(csr)
			csrPatch := csr.DeepCopy()
			csrPatch.SetLabels(map[string]string{"foo": "bar"})
			Expect(testClientNodeAgent.Patch(ctx, csrPatch, patch)).To(BeForbiddenError())

			Expect(testClientNodeAgent.Delete(ctx, csr)).To(BeForbiddenError())
		})

		It("should forbid to list CertificateSigningRequests", func() {
			Expect(testClientNodeAgent.List(ctx, &certificatesv1.CertificateSigningRequestList{})).To(BeForbiddenError())
		})
	})

	Describe("#Events", func() {
		var (
			recorder record.EventRecorder
		)

		BeforeEach(func() {
			corev1Client, err := corev1client.NewForConfig(testRestConfigNodeAgent)
			Expect(err).ToNot(HaveOccurred())
			broadcaster := record.NewBroadcaster()
			broadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: corev1Client.Events("")})
			recorder = broadcaster.NewRecorder(testClientNodeAgent.Scheme(), corev1.EventSource{Component: "nodeagentauthorizer-test"})

			node.Name = fmt.Sprintf("event-node-%s", utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:16])
			createNodeForMachine(node, machine)
		})

		It("should be able to create an event", func() {
			recorder.Event(node, "Normal", "test-reason", "test-message")

			Eventually(func(g Gomega) {
				eventList := &corev1.EventList{}
				g.Expect(testClient.List(ctx, eventList, client.MatchingFields{"involvedObject.name": node.Name})).To(Succeed())
				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Type).To(Equal("Normal"))
				g.Expect(eventList.Items[0].Reason).To(Equal("test-reason"))
				g.Expect(eventList.Items[0].Message).To(Equal("test-message"))
				g.Expect(eventList.Items[0].Count).To(Equal(int32(1)))
			}).Should(Succeed())
		})

		It("should be able to create and patch an event", func() {
			// Recording the same event multiple times in a short period makes the event recorder patching the event.
			for range 3 {
				recorder.Event(node, "Normal", "test-reason", "test-message")
			}

			Eventually(func(g Gomega) {
				eventList := &corev1.EventList{}
				g.Expect(testClient.List(ctx, eventList, client.MatchingFields{"involvedObject.name": node.Name})).To(Succeed())
				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Type).To(Equal("Normal"))
				g.Expect(eventList.Items[0].Reason).To(Equal("test-reason"))
				g.Expect(eventList.Items[0].Message).To(Equal("test-message"))
				g.Expect(eventList.Items[0].Count).To(Equal(int32(3)))
			}).Should(Succeed())
		})

		It("should forbid to list events", func() {
			Expect(testClientNodeAgent.List(ctx, &corev1.EventList{})).To(BeForbiddenError())
		})

		It("should forbid to update an event", func() {
			Expect(testClientNodeAgent.Update(ctx, &corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "foo-event", Namespace: "default"}})).To(BeForbiddenError())
		})

		It("should forbid to delete an event", func() {
			Expect(testClientNodeAgent.Delete(ctx, &corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "foo-event", Namespace: "default"}})).To(BeForbiddenError())
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
					Name:   otherNodeName,
					Labels: map[string]string{"node.gardener.cloud/machine-name": otherMachineName},
				},
			}
			DeferCleanup(func() {
				By("Delete other Node")
				Expect(testClient.Delete(ctx, otherNode)).To(Or(Succeed(), BeNotFoundError()))
			})
			createNodeForMachine(otherNode, otherMachine)
		})

		DescribeTable("#get",
			func(name, namespace string, createNode, allow bool) {
				matcher := BeForbiddenError()
				if allow {
					matcher = Succeed()
				}
				if createNode {
					createNodeForMachine(node, machine)
				}
				lease := &coordinationv1.Lease{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
					},
				}
				Expect(testClient.Create(ctx, lease)).To(Succeed())
				DeferCleanup(func() {
					Expect(testClient.Delete(ctx, lease)).To(Or(Succeed(), BeNotFoundError()))
				})
				Expect(testClientNodeAgent.Get(ctx, client.ObjectKeyFromObject(lease), &coordinationv1.Lease{})).To(matcher)
			},
			Entry("allow own gardener-node-agent", nodeAgentLeaseName, "kube-system", true, true),
			Entry("forbid if no own node", nodeAgentLeaseName, "kube-system", false, false),
			Entry("forbid other gardener-node-agent", otherNodeAgentLeaseName, "kube-system", true, false),
			Entry("forbid other gardener-node-agent if no own node", otherNodeAgentLeaseName, "kube-system", false, false),
			Entry("forbid in default namespace", "foo-bar", "default", true, false),
			Entry("forbid in default namespace without node", "foo-bar", "default", false, false),
		)

		DescribeTable("#update",
			func(name, namespace string, createNode, allow bool) {
				matcher := BeForbiddenError()
				if allow {
					matcher = Succeed()
				}
				if createNode {
					createNodeForMachine(node, machine)
				}
				lease := &coordinationv1.Lease{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
					},
				}
				Expect(testClient.Create(ctx, lease)).To(Succeed())
				DeferCleanup(func() {
					Expect(testClient.Delete(ctx, lease)).To(Or(Succeed(), BeNotFoundError()))
				})
				lease.SetLabels(map[string]string{"foo": "bar"})
				Expect(testClientNodeAgent.Update(ctx, lease.DeepCopy())).To(matcher)
			},
			Entry("allow own gardener-node-agent", nodeAgentLeaseName, "kube-system", true, true),
			Entry("forbid if no own node", nodeAgentLeaseName, "kube-system", false, false),
			Entry("forbid other gardener-node-agent", otherNodeAgentLeaseName, "kube-system", true, false),
			Entry("forbid other gardener-node-agent if no own node", otherNodeAgentLeaseName, "kube-system", false, false),
			Entry("forbid in default namespace", "foo-bar", "default", true, false),
			Entry("forbid in default namespace without node", "foo-bar", "default", false, false),
		)

		DescribeTable("#patch",
			func(name, namespace string, createNode bool) {
				if createNode {
					createNodeForMachine(node, machine)
				}
				lease := &coordinationv1.Lease{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
					},
				}
				Expect(testClient.Create(ctx, lease)).To(Succeed())
				DeferCleanup(func() {
					Expect(testClient.Delete(ctx, lease)).To(Or(Succeed(), BeNotFoundError()))
				})
				patch := client.MergeFrom(lease)
				lease.SetLabels(map[string]string{"foo": "bar"})
				Expect(testClientNodeAgent.Patch(ctx, lease.DeepCopy(), patch)).To(BeForbiddenError())
			},
			Entry("forbid own gardener-node-agent", nodeAgentLeaseName, "kube-system", true),
			Entry("forbid if no own node", nodeAgentLeaseName, "kube-system", false),
			Entry("forbid other gardener-node-agent", otherNodeAgentLeaseName, "kube-system", true),
			Entry("forbid other gardener-node-agent if no own node", otherNodeAgentLeaseName, "kube-system", false),
			Entry("forbid in default namespace", "foo-bar", "default", true),
			Entry("forbid in default namespace without node", "foo-bar", "default", false),
		)

		DescribeTable("#delete",
			func(name, namespace string, createNode bool) {
				if createNode {
					createNodeForMachine(node, machine)
				}
				lease := &coordinationv1.Lease{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
					},
				}
				Expect(testClient.Create(ctx, lease)).To(Succeed())
				DeferCleanup(func() {
					Expect(testClient.Delete(ctx, lease)).To(Or(Succeed(), BeNotFoundError()))
				})
				Expect(testClientNodeAgent.Delete(ctx, lease.DeepCopy())).To(BeForbiddenError())
			},
			Entry("forbid own gardener-node-agent", nodeAgentLeaseName, "kube-system", true),
			Entry("forbid if no own node", nodeAgentLeaseName, "kube-system", false),
			Entry("forbid other gardener-node-agent", otherNodeAgentLeaseName, "kube-system", true),
			Entry("forbid other gardener-node-agent if no own node", otherNodeAgentLeaseName, "kube-system", false),
			Entry("forbid in default namespace", "foo-bar", "default", true),
			Entry("forbid in default namespace without node", "foo-bar", "default", false),
		)

		DescribeTable("#list",
			func(name, namespace string, createNode, allow bool) {
				matcher := BeForbiddenError()
				if allow {
					matcher = Succeed()
				}
				if createNode {
					createNodeForMachine(node, machine)
				}
				var listOptions []client.ListOption
				if name != "" {
					listOptions = append(listOptions, client.MatchingFields{"metadata.name": name})
				}
				if namespace != "" {
					listOptions = append(listOptions, client.InNamespace(namespace))
				}
				Expect(testClientNodeAgent.List(ctx, &coordinationv1.LeaseList{}, listOptions...)).To(matcher)
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
			func(name, namespace string, createNode, allow bool) {
				matcher := BeForbiddenError()
				if allow {
					matcher = Succeed()
				}
				if createNode {
					createNodeForMachine(node, machine)
				}
				lease := &coordinationv1.Lease{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
					},
				}
				Expect(testClientNodeAgent.Create(ctx, lease)).To(matcher)
				DeferCleanup(func() {
					Expect(testClient.Delete(ctx, lease)).To(Or(Succeed(), BeNotFoundError()))
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
					Labels: map[string]string{"node.gardener.cloud/machine-name": otherMachineName},
				},
			}
			DeferCleanup(func() {
				By("Delete other Node")
				Expect(testClient.Delete(ctx, otherNode)).To(Or(Succeed(), BeNotFoundError()))
			})
			createNodeForMachine(otherNode, otherMachine)
		})

		DescribeTable("#get",
			func(name string, createNode, allow bool) {
				matcher := BeForbiddenError()
				if allow {
					matcher = Succeed()
				}
				if createNode {
					createNodeForMachine(node, machine)
				}
				Expect(testClientNodeAgent.Get(ctx, client.ObjectKey{Name: name}, &corev1.Node{})).To(matcher)
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

				createNodeForMachine(node, machine)

				testNode := &corev1.Node{}
				Expect(testClient.Get(ctx, client.ObjectKey{Name: name}, testNode)).To(Succeed())
				testNode.Labels["foo"] = "bar"
				Expect(testClientNodeAgent.Update(ctx, testNode)).To(matcher)
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

				createNodeForMachine(node, machine)

				testNode := &corev1.Node{}
				Expect(testClient.Get(ctx, client.ObjectKey{Name: name}, testNode)).To(Succeed())
				patch := client.MergeFrom(testNode)
				testNode.Labels["foo"] = "bar"
				Expect(testClientNodeAgent.Patch(ctx, testNode, patch)).To(matcher)
			},
			Entry("allow own node", nodeName, true),
			Entry("forbid other node", otherNodeName, false),
		)

		DescribeTable("#delete",
			func(name string) {
				createNodeForMachine(node, machine)

				testNode := &corev1.Node{}
				Expect(testClient.Get(ctx, client.ObjectKey{Name: name}, testNode)).To(Succeed())
				Expect(testClientNodeAgent.Delete(ctx, testNode)).To(BeForbiddenError())
			},
			Entry("forbid own node", nodeName),
			Entry("forbid other node", otherNodeName),
		)

		It("should allow listing nodes unconditionally", func() {
			Expect(testClientNodeAgent.List(ctx, &corev1.NodeList{})).To(Succeed())
		})
	})

	Describe("#Secrets", func() {
		BeforeEach(func() {
			machineSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      machineSecretName,
					Namespace: "kube-system",
				},
			}
			Expect(testClient.Create(ctx, machineSecret)).To(Succeed())
			DeferCleanup(func() {
				By("Delete machine Secret")
				Expect(testClient.Delete(ctx, machineSecret)).To(Or(Succeed(), BeNotFoundError()))
			})

			otherMachineSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      otherMachineSecretName,
					Namespace: "kube-system",
				},
			}
			Expect(testClient.Create(ctx, otherMachineSecret)).To(Succeed())
			DeferCleanup(func() {
				By("Delete other machine Secret")
				Expect(testClient.Delete(ctx, otherMachineSecret)).To(Or(Succeed(), BeNotFoundError()))
			})

			valitailSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      valitailSecretName,
					Namespace: "kube-system",
				},
			}
			Expect(testClient.Create(ctx, valitailSecret)).To(Succeed())
			DeferCleanup(func() {
				By("Delete valitail Secret")
				Expect(testClient.Delete(ctx, valitailSecret)).To(Or(Succeed(), BeNotFoundError()))
			})

			fooSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
				},
			}
			Expect(testClient.Create(ctx, fooSecret)).To(Succeed())
			DeferCleanup(func() {
				By("Delete foo Secret")
				Expect(testClient.Delete(ctx, fooSecret)).To(Or(Succeed(), BeNotFoundError()))
			})
		})

		DescribeTable("#get",
			func(name, namespace string, allow bool) {
				matcher := BeForbiddenError()
				if allow {
					matcher = Succeed()
				}
				Expect(testClientNodeAgent.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &corev1.Secret{})).To(matcher)
			},
			Entry("allow valitail secret", valitailSecretName, "kube-system", true),
			Entry("allow machine secret", machineSecretName, "kube-system", true),
			Entry("forbid foo secret", "foo", "default", false),
			Entry("forbid other machine secret", otherMachineSecretName, "kube-system", false),
		)

		DescribeTable("#update",
			func(name, namespace string) {
				secret := &corev1.Secret{}
				Expect(testClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, secret)).To(Succeed())
				secret.SetLabels(map[string]string{"foo": "bar"})
				Expect(testClientNodeAgent.Update(ctx, secret)).To(BeForbiddenError())
			},
			Entry("forbid valitail secret", valitailSecretName, "kube-system"),
			Entry("forbid machine secret", machineSecretName, "kube-system"),
			Entry("forbid foo secret", "foo", "default"),
			Entry("forbid other machine secret", otherMachineSecretName, "kube-system"),
		)

		DescribeTable("#patch",
			func(name, namespace string) {
				secret := &corev1.Secret{}
				Expect(testClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, secret)).To(Succeed())
				patch := client.MergeFrom(secret.DeepCopy())
				secret.SetLabels(map[string]string{"foo": "bar"})
				Expect(testClientNodeAgent.Patch(ctx, secret, patch)).To(BeForbiddenError())
			},
			Entry("forbid valitail secret", valitailSecretName, "kube-system"),
			Entry("forbid machine secret", machineSecretName, "kube-system"),
			Entry("forbid foo secret", "foo", "default"),
			Entry("forbid other machine secret", otherMachineSecretName, "kube-system"),
		)

		DescribeTable("#delete",
			func(name, namespace string) {
				secret := &corev1.Secret{}
				Expect(testClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, secret)).To(Succeed())
				Expect(testClientNodeAgent.Delete(ctx, secret)).To(BeForbiddenError())
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
				Expect(testClientNodeAgent.List(ctx, &corev1.SecretList{}, listOptions...)).To(matcher)
			},
			Entry("allow valitail secret", valitailSecretName, "kube-system", true),
			Entry("allow machine secret", machineSecretName, "kube-system", true),
			Entry("forbid default namespace", "", "default", false),
			Entry("forbid kube-system namespace", "", "kube-system", false),
			Entry("forbid cluster wide", "", "", false),
		)
	})
})

func createNodeForMachine(node *corev1.Node, machine *machinev1alpha1.Machine) {
	ExpectWithOffset(1, testClient.Create(ctx, node)).To(Succeed())
	machine.SetLabels(map[string]string{machinev1alpha1.NodeLabelKey: node.Name})
	ExpectWithOffset(1, testClient.Update(ctx, machine)).To(Succeed())
}

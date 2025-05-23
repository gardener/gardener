// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeagentauthorizer_test

import (
	"context"
	"fmt"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apiserver/pkg/authentication/user"
	auth "k8s.io/apiserver/pkg/authorization/authorizer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/resourcemanager/webhook/nodeagentauthorizer"
)

var _ = Describe("Authorizer", func() {
	var (
		ctx context.Context

		authorizer       auth.Authorizer
		log              logr.Logger
		sourceClient     client.Client
		targetClient     client.Client
		machineNamespace string

		machineName          string
		machineSecretName    string
		machine              *machinev1alpha1.Machine
		nodeName             string
		node                 *corev1.Node
		newMachineName       string
		newMachineSecretName string
		newMachine           *machinev1alpha1.Machine
		nodeAgentUser        user.Info
		newNodeAgentUser     user.Info
	)

	BeforeEach(func() {
		ctx = context.Background()

		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		sourceClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		targetClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()
		machineNamespace = "shoot--foo"
		authorizer = NewAuthorizer(log, sourceClient, targetClient, machineNamespace, true)

		machineName = "foo-machine"
		machineSecretName = "foo-machine-secret"
		newMachineName = "bar-machine"
		newMachineSecretName = "bar-machine-secret"
		nodeName = "foo-node"
		nodeAgentUser = &user.DefaultInfo{
			Name:   fmt.Sprintf("%s%s", v1beta1constants.NodeAgentUserNamePrefix, machineName),
			Groups: []string{v1beta1constants.NodeAgentsGroup},
		}
		newNodeAgentUser = &user.DefaultInfo{
			Name:   fmt.Sprintf("%s%s", v1beta1constants.NodeAgentUserNamePrefix, newMachineName),
			Groups: []string{v1beta1constants.NodeAgentsGroup},
		}

		machine = &machinev1alpha1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      machineName,
				Namespace: machineNamespace,
				Labels:    map[string]string{machinev1alpha1.NodeLabelKey: nodeName},
			},
			Spec: machinev1alpha1.MachineSpec{
				NodeTemplateSpec: machinev1alpha1.NodeTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName: machineSecretName},
					},
				},
			},
		}
		Expect(sourceClient.Create(ctx, machine)).To(Succeed())

		newMachine = &machinev1alpha1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      newMachineName,
				Namespace: machineNamespace,
			},
			Spec: machinev1alpha1.MachineSpec{
				NodeTemplateSpec: machinev1alpha1.NodeTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName: newMachineSecretName},
					},
				},
			},
		}
		Expect(sourceClient.Create(ctx, newMachine)).To(Succeed())

		node = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   nodeName,
				Labels: map[string]string{"node.gardener.cloud/machine-name": machineName},
			},
		}
		Expect(targetClient.Create(ctx, node)).To(Succeed())

		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(sourceClient.Delete(ctx, machine))).To(Succeed())
			Expect(client.IgnoreNotFound(sourceClient.Delete(ctx, newMachine))).To(Succeed())
			Expect(client.IgnoreNotFound(targetClient.Delete(ctx, node))).To(Succeed())
		})
	})

	Describe("#Authorize", func() {
		Context("users", func() {
			It("should have no opinion for a non gardener-node-agent user", func() {
				attrs := auth.AttributesRecord{
					User: &user.DefaultInfo{
						Name: "foo",
					},
				}
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionNoOpinion))
				Expect(reason).To(BeEmpty())
			})

			It("should have no opinion for if the user is not member of the gardener-node-agent group", func() {
				attrs := auth.AttributesRecord{
					User: &user.DefaultInfo{
						Name: fmt.Sprintf("%s%s", v1beta1constants.NodeAgentUserNamePrefix, machineName),
					},
				}
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionNoOpinion))
				Expect(reason).To(BeEmpty())
			})

			It("should deny for a gardener-node-agent with an empty machine name", func() {
				attrs := auth.AttributesRecord{
					User: &user.DefaultInfo{
						Name:   v1beta1constants.NodeAgentUserNamePrefix,
						Groups: []string{v1beta1constants.NodeAgentsGroup},
					},
				}
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionDeny))
				Expect(reason).To(BeEmpty())
			})
		})

		Context("#CertificateSigningRequests", func() {
			It("should allow to create a certificate signing request", func() {
				attrs := &auth.AttributesRecord{
					User:            nodeAgentUser,
					Name:            "",
					APIGroup:        "certificates.k8s.io",
					Resource:        "certificatesigningrequests",
					ResourceRequest: true,
					Verb:            "create",
				}
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
			})

			It("should allow to get a certificate signing request created by the same user", func() {
				csr := &certificatesv1.CertificateSigningRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo-csr",
					},
					Spec: certificatesv1.CertificateSigningRequestSpec{
						Username: nodeAgentUser.GetName(),
					},
				}
				Expect(targetClient.Create(ctx, csr)).To(Succeed())

				attrs := &auth.AttributesRecord{
					User:            nodeAgentUser,
					Name:            "foo-csr",
					APIGroup:        "certificates.k8s.io",
					Resource:        "certificatesigningrequests",
					ResourceRequest: true,
					Verb:            "get",
				}
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
			})

			It("should deny to get a certificate signing request created by a different user", func() {
				csr := &certificatesv1.CertificateSigningRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo-csr",
					},
					Spec: certificatesv1.CertificateSigningRequestSpec{
						Username: newNodeAgentUser.GetName(),
					},
				}
				Expect(targetClient.Create(ctx, csr)).To(Succeed())

				attrs := &auth.AttributesRecord{
					User:            nodeAgentUser,
					Name:            "foo-csr",
					APIGroup:        "certificates.k8s.io",
					Resource:        "certificatesigningrequests",
					ResourceRequest: true,
					Verb:            "get",
				}
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionDeny))
				Expect(reason).To(Equal(fmt.Sprintf("gardener-node-agent is only allowed to read or request CSRs for its own user %q", nodeAgentUser.GetName())))
			})

			DescribeTable("should deny because no allowed verb", func(verb string) {
				attrs := &auth.AttributesRecord{
					User:            nodeAgentUser,
					Name:            "foo-csr",
					APIGroup:        "certificates.k8s.io",
					Resource:        "certificatesigningrequests",
					ResourceRequest: true,
					Verb:            verb,
				}
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionDeny))
				Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get create]"))
			},
				Entry("update", "update"),
				Entry("patch", "patch"),
				Entry("delete", "delete"),
				Entry("deletecollection", "deletecollection"),
				Entry("list", "list"),
				Entry("watch", "watch"),
			)
		})

		Context("#Events", func() {
			DescribeTable("should allow some verbs", func(verb string) {
				attrs := &auth.AttributesRecord{
					User:            nodeAgentUser,
					Name:            "",
					APIGroup:        "",
					Resource:        "events",
					ResourceRequest: true,
					Verb:            verb,
				}
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
			},
				Entry("create", "create"),
				Entry("patch", "patch"),
			)

			DescribeTable("should deny because no allowed verb", func(verb string) {
				attrs := &auth.AttributesRecord{
					User:            nodeAgentUser,
					Name:            "foo-event",
					APIGroup:        "",
					Resource:        "events",
					ResourceRequest: true,
					Verb:            verb,
				}
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionDeny))
				Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [create patch]"))
			},
				Entry("get", "get"),
				Entry("update", "update"),
				Entry("delete", "delete"),
				Entry("deletecollection", "deletecollection"),
				Entry("list", "list"),
				Entry("watch", "watch"),
			)
		})

		Context("#Leases", func() {
			It("should allow to create a lease for a machine with a node label", func() {
				attrs := &auth.AttributesRecord{
					User:            nodeAgentUser,
					Name:            "",
					Namespace:       "kube-system",
					APIGroup:        "coordination.k8s.io",
					Resource:        "leases",
					ResourceRequest: true,
					Verb:            "create",
				}
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
			})

			It("should deny creating a lease in a namespace other than kube-system", func() {
				attrs := &auth.AttributesRecord{
					User:            nodeAgentUser,
					Name:            "",
					Namespace:       "default",
					APIGroup:        "coordination.k8s.io",
					Resource:        "leases",
					ResourceRequest: true,
					Verb:            "create",
				}
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionDeny))
				Expect(reason).To(Equal(fmt.Sprintf("this gardener-node-agent can only access lease \"gardener-node-agent-%s\" in \"kube-system\" namespace", nodeName)))
			})

			DescribeTable("should deny accessing a lease for a machine without a node label", func(verb string) {
				attrs := &auth.AttributesRecord{
					User:            newNodeAgentUser,
					Name:            "",
					Namespace:       "kube-system",
					APIGroup:        "coordination.k8s.io",
					Resource:        "leases",
					ResourceRequest: true,
					Verb:            verb,
				}
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionDeny))
				Expect(reason).To(Equal(fmt.Sprintf("expecting \"node\" label on machine %q", newMachineName)))
			},
				Entry("create", "create"),
				Entry("get", "get"),
				Entry("update", "update"),
				Entry("list", "list"),
				Entry("watch", "watch"),
			)

			DescribeTable("should allow accessing the lease which belongs to the gardener-node-agent instance", func(verb string) {
				attrs := &auth.AttributesRecord{
					User:            nodeAgentUser,
					Name:            fmt.Sprintf("gardener-node-agent-%s", nodeName),
					Namespace:       "kube-system",
					APIGroup:        "coordination.k8s.io",
					Resource:        "leases",
					ResourceRequest: true,
					Verb:            verb,
				}
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
			},
				Entry("get", "get"),
				Entry("update", "update"),
				Entry("list", "list"),
				Entry("watch", "watch"),
			)

			DescribeTable("should deny accessing a lease which belongs to a different gardener-node-agent instance", func(verb string) {
				attrs := &auth.AttributesRecord{
					User:            nodeAgentUser,
					Name:            "gardener-node-agent-other-bar-node",
					Namespace:       "kube-system",
					APIGroup:        "coordination.k8s.io",
					Resource:        "leases",
					ResourceRequest: true,
					Verb:            verb,
				}
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionDeny))
				Expect(reason).To(Equal(fmt.Sprintf("this gardener-node-agent can only access lease \"gardener-node-agent-%s\" in \"kube-system\" namespace", nodeName)))
			},
				Entry("get", "get"),
				Entry("update", "update"),
				Entry("list", "list"),
				Entry("watch", "watch"),
			)

			DescribeTable("should deny accessing a lease in a different namespace", func(verb string) {
				attrs := &auth.AttributesRecord{
					User:            nodeAgentUser,
					Name:            fmt.Sprintf("gardener-node-agent-%s", nodeName),
					Namespace:       "default",
					APIGroup:        "coordination.k8s.io",
					Resource:        "leases",
					ResourceRequest: true,
					Verb:            verb,
				}
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionDeny))
				Expect(reason).To(Equal(fmt.Sprintf("this gardener-node-agent can only access lease \"gardener-node-agent-%s\" in \"kube-system\" namespace", nodeName)))
			},
				Entry("get", "get"),
				Entry("update", "update"),
				Entry("list", "list"),
				Entry("watch", "watch"),
			)

			DescribeTable("should deny because no allowed verb", func(verb string) {
				attrs := &auth.AttributesRecord{
					User:            nodeAgentUser,
					Name:            "gardener-node-agent-other-bar-node",
					Namespace:       "kube-system",
					APIGroup:        "coordination.k8s.io",
					Resource:        "leases",
					ResourceRequest: true,
					Verb:            verb,
				}
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionDeny))
				Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get list watch create update]"))
			},
				Entry("patch", "patch"),
				Entry("delete", "delete"),
				Entry("deletecollection", "deletecollection"),
			)
		})

		Context("#Nodes", func() {
			DescribeTable("should allow some verbs unconditionally", func(verb string) {
				attrs := &auth.AttributesRecord{
					User:            nodeAgentUser,
					Name:            "",
					APIGroup:        "",
					Resource:        "nodes",
					ResourceRequest: true,
					Verb:            verb,
				}
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
			},
				Entry("list", "list"),
				Entry("watch", "watch"),
			)

			DescribeTable("should allow accessing the node which belongs to the gardener-node-agent instance", func(verb string) {
				attrs := &auth.AttributesRecord{
					User:            nodeAgentUser,
					Name:            nodeName,
					APIGroup:        "",
					Resource:        "nodes",
					ResourceRequest: true,
					Verb:            verb,
				}
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
			},
				Entry("get", "get"),
				Entry("patch", "patch"),
				Entry("update", "update"),
			)

			DescribeTable("should deny accessing a node which belongs to a different gardener-node-agent instance", func(verb string) {
				anotherNodeName := "another-bar-node"
				attrs := &auth.AttributesRecord{
					User:            nodeAgentUser,
					Name:            anotherNodeName,
					APIGroup:        "",
					Resource:        "nodes",
					ResourceRequest: true,
					Verb:            verb,
				}
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionDeny))
				Expect(reason).To(Equal(fmt.Sprintf("node %q does not belong to machine %q", anotherNodeName, machineName)))
			},
				Entry("get", "get"),
				Entry("patch", "patch"),
				Entry("update", "update"),
			)

			DescribeTable("should allow accessing status subresource which the node which belongs to the gardener-node-agent instance", func(verb string) {
				attrs := &auth.AttributesRecord{
					User:            nodeAgentUser,
					Name:            nodeName,
					APIGroup:        "",
					Resource:        "nodes",
					Subresource:     "status",
					ResourceRequest: true,
					Verb:            verb,
				}
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
			},
				Entry("get", "get"),
				Entry("patch", "patch"),
				Entry("update", "update"),
				Entry("list", "list"),
				Entry("watch", "watch"),
			)

			DescribeTable("should deny accessing a random subresource", func(verb string) {
				attrs := &auth.AttributesRecord{
					User:            nodeAgentUser,
					Name:            nodeName,
					APIGroup:        "",
					Resource:        "nodes",
					Subresource:     "foo-subresource",
					ResourceRequest: true,
					Verb:            verb,
				}
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionDeny))
				Expect(reason).To(Equal("only the following subresources are allowed for this resource type: [status]"))
			},
				Entry("get", "get"),
				Entry("patch", "patch"),
				Entry("update", "update"),
				Entry("list", "list"),
				Entry("watch", "watch"),
			)

			DescribeTable("should deny because no allowed verb", func(verb string) {
				attrs := &auth.AttributesRecord{
					User:            nodeAgentUser,
					Name:            "",
					APIGroup:        "",
					Resource:        "nodes",
					ResourceRequest: true,
					Verb:            verb,
				}
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionDeny))
				Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get list watch patch update]"))
			},
				Entry("create", "create"),
				Entry("delete", "delete"),
				Entry("deletecollection", "deletecollection"),
			)
		})

		Context("#Secrets", func() {
			DescribeTable("should allow accessing the secrets which belong to the gardener-node-agent instance", func(verb string) {
				attrs := &auth.AttributesRecord{
					User:            nodeAgentUser,
					Name:            "",
					Namespace:       "kube-system",
					APIGroup:        "",
					Resource:        "secrets",
					ResourceRequest: true,
					Verb:            verb,
				}
				secretNames := []string{machineSecretName, "gardener-valitail"}

				for _, secretName := range secretNames {
					attrs.Name = secretName
					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				}
			},
				Entry("get", "get"),
				Entry("list", "list"),
				Entry("watch", "watch"),
			)

			DescribeTable("should deny accessing the node agent config secret which belong to a different gardener-node-agent instance", func(verb string) {
				attrs := &auth.AttributesRecord{
					User:            nodeAgentUser,
					Name:            newMachineSecretName,
					Namespace:       "kube-system",
					APIGroup:        "",
					Resource:        "secrets",
					ResourceRequest: true,
					Verb:            verb,
				}

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionDeny))
				Expect(reason).To(Equal(fmt.Sprintf("gardener-node-agent can only access secrets [%s gardener-valitail] in \"kube-system\" namespace", machineSecretName)))
			},
				Entry("get", "get"),
				Entry("list", "list"),
				Entry("watch", "watch"),
			)

			DescribeTable("should deny accessing secrets in a different namespace", func(verb string) {
				attrs := &auth.AttributesRecord{
					User:            nodeAgentUser,
					Name:            "",
					Namespace:       "default",
					APIGroup:        "",
					Resource:        "secrets",
					ResourceRequest: true,
					Verb:            verb,
				}
				secretNames := []string{machineSecretName, "gardener-valitail"}

				for _, secretName := range secretNames {
					attrs.Name = secretName
					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionDeny))
					Expect(reason).To(Equal(fmt.Sprintf("gardener-node-agent can only access secrets [%s gardener-valitail] in \"kube-system\" namespace", machineSecretName)))
				}
			},
				Entry("get", "get"),
				Entry("list", "list"),
				Entry("watch", "watch"),
			)

			DescribeTable("should deny because no allowed verb", func(verb string) {
				attrs := &auth.AttributesRecord{
					User:            nodeAgentUser,
					Name:            newMachineSecretName,
					Namespace:       "kube-system",
					APIGroup:        "",
					Resource:        "secrets",
					ResourceRequest: true,
					Verb:            verb,
				}
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionDeny))
				Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get list watch]"))
			},
				Entry("create", "create"),
				Entry("patch", "patch"),
				Entry("update", "update"),
				Entry("delete", "delete"),
				Entry("deletecollection", "deletecollection"),
			)
		})

		Context("#Pods", func() {
			var (
				nodeName     string
				podName      string
				podNamespace string
				pod          *corev1.Pod
				attrs        auth.AttributesRecord
			)

			BeforeEach(func() {
				nodeName = "foo-node"
				podName = "foo-pod"
				podNamespace = "default"

				pod = &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: podNamespace,
					},
					Spec: corev1.PodSpec{
						NodeName: nodeName,
					},
				}
				Expect(targetClient.Create(ctx, pod)).To(Succeed())

				attrs = auth.AttributesRecord{
					User:            nodeAgentUser,
					Name:            podName,
					Namespace:       podNamespace,
					APIGroup:        "",
					Resource:        "pods",
					ResourceRequest: true,
					Verb:            "get",
				}
			})

			DescribeTable("should allow accessing the pods which belong to the same node", func(verb string) {
				attrs.Verb = verb
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
			},
				Entry("get", "get"),
				Entry("delete", "delete"),
			)

			DescribeTable("should allow accessing the pods because authorizeWithSelectors is false", func(verb string) {
				attrs.Name = ""
				attrs.Verb = verb
				authorizer = NewAuthorizer(log, sourceClient, targetClient, machineNamespace, false)
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
			},
				Entry("list", "list"),
				Entry("watch", "watch"),
			)

			DescribeTable("should allow accessing the pods because the request has a field selector for this node", func(verb string) {
				attrs.Name = ""
				attrs.Verb = verb
				attrs.FieldSelectorRequirements = fields.Requirements{
					{
						Field:    "spec.nodeName",
						Operator: selection.Equals,
						Value:    nodeName,
					},
				}
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
			},
				Entry("list", "list"),
				Entry("watch", "watch"),
			)

			DescribeTable("should allow accessing the pods because the request is a list for a particular pod, and the pod belongs to this node", func(verb string) {
				attrs.Name = podName
				attrs.Namespace = podNamespace
				attrs.Verb = verb
				attrs.FieldSelectorRequirements = fields.Requirements{
					{
						Field:    "metadata.name",
						Operator: selection.In,
						Value:    podName,
					},
				}
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
			},
				Entry("list", "list"),
				Entry("watch", "watch"),
			)

			DescribeTable("should allow accessing the pods because the request is a list for a particular pod, and the pod doesn't belong to this node", func(verb string) {
				attrs.Name = podName
				attrs.Namespace = podNamespace
				attrs.Verb = verb

				pod.Spec.NodeName = "different-node"
				Expect(targetClient.Update(ctx, pod)).To(Succeed())

				attrs.FieldSelectorRequirements = fields.Requirements{
					{
						Field:    "metadata.name",
						Operator: selection.In,
						Value:    podName,
					},
				}
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionDeny))
				Expect(reason).To(ContainSubstring(fmt.Sprintf("pod %q does not belong to node %q", client.ObjectKeyFromObject(pod), nodeName)))
			},
				Entry("list", "list"),
				Entry("watch", "watch"),
			)

			DescribeTable("should deny accessing the pods because the request doesn't have a field selector for this node", func(verb string) {
				attrs.Name = ""
				attrs.Verb = verb
				attrs.FieldSelectorRequirements = fields.Requirements{
					{
						Field:    "spec.nodeName",
						Operator: selection.Equals,
						Value:    "different-node",
					},
				}
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionDeny))
				Expect(reason).To(ContainSubstring("can only list/watch pods with spec.nodeName=%s field selector", nodeName))
			},
				Entry("list", "list"),
				Entry("watch", "watch"),
			)

			DescribeTable("should deny because no allowed verb", func(verb string) {
				attrs.Verb = verb
				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionDeny))
				Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get list watch delete]"))
			},
				Entry("create", "create"),
				Entry("patch", "patch"),
				Entry("update", "update"),
				Entry("deletecollection", "deletecollection"),
			)

			DescribeTable("should deny access if the machine does not exist", func(verb string) {
				attrs.Verb = verb
				Expect(sourceClient.Delete(ctx, machine)).To(Succeed())

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).To(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionDeny))
				Expect(reason).To(BeEmpty())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("error getting machine %q", machineName)))
			},
				Entry("get", "get"),
				Entry("list", "list"),
				Entry("watch", "watch"),
				Entry("delete", "delete"),
			)

			DescribeTable("should deny access if the machine does not have a 'node' label", func(verb string) {
				attrs.Verb = verb
				delete(machine.Labels, machinev1alpha1.NodeLabelKey)
				Expect(sourceClient.Update(ctx, machine)).To(Succeed())

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionDeny))
				Expect(reason).To(ContainSubstring(fmt.Sprintf(`expecting "node" label on machine %q`, machineName)))
			},
				Entry("get", "get"),
				Entry("list", "list"),
				Entry("watch", "watch"),
				Entry("delete", "delete"),
			)

			DescribeTable("should deny access if the pod does not exist", func(verb string) {
				attrs.Verb = verb
				Expect(targetClient.Delete(ctx, pod)).To(Succeed())

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).To(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionDeny))
				Expect(reason).To(BeEmpty())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("error getting pod %q", client.ObjectKey{Name: podName, Namespace: podNamespace})))
			},
				Entry("get", "get"),
				Entry("delete", "delete"),
			)

			DescribeTable("should deny access if the pod belongs to a different node", func(verb string) {
				attrs.Verb = verb
				pod.Spec.NodeName = "different-node"
				Expect(targetClient.Update(ctx, pod)).To(Succeed())

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionDeny))
				Expect(reason).To(ContainSubstring(fmt.Sprintf("pod %q does not belong to node %q", client.ObjectKeyFromObject(pod), nodeName)))
			},
				Entry("get", "get"),
				Entry("delete", "delete"),
			)
		})
	})
})

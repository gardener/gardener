// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed_test

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	certificatesv1 "k8s.io/api/certificates/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/apiserver/pkg/authentication/user"
	auth "k8s.io/apiserver/pkg/authorization/authorizer"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhook/auth/seed"
	graphpkg "github.com/gardener/gardener/pkg/admissioncontroller/webhook/auth/seed/graph"
	mockgraph "github.com/gardener/gardener/pkg/admissioncontroller/webhook/auth/seed/graph/mock"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/kubernetes/bootstraptoken"
)

var _ = Describe("Seed", func() {
	var (
		ctx  context.Context
		ctrl *gomock.Controller

		log        logr.Logger
		graph      *mockgraph.MockInterface
		authorizer auth.Authorizer

		seedName      string
		seedUser      user.Info
		gardenletUser user.Info
		extensionUser user.Info
	)

	BeforeEach(func() {
		ctx = context.Background()
		ctrl = gomock.NewController(GinkgoT())

		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		graph = mockgraph.NewMockInterface(ctrl)
		authorizer = NewAuthorizer(log, graph)

		seedName = "seed"
		gardenletUser = &user.DefaultInfo{
			Name:   fmt.Sprintf("%s%s", v1beta1constants.SeedUserNamePrefix, seedName),
			Groups: []string{v1beta1constants.SeedsGroup},
		}
		extensionUser = (&serviceaccount.ServiceAccountInfo{
			Name:      v1beta1constants.ExtensionGardenServiceAccountPrefix + "provider-local",
			Namespace: gardenerutils.SeedNamespaceNamePrefix + seedName,
		}).UserInfo()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Authorize", func() {
		Context("when resource is unhandled", func() {
			It("should have no opinion because no seed", func() {
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

			It("should have no opinion because no resource request", func() {
				attrs := auth.AttributesRecord{
					User:     gardenletUser,
					APIGroup: "",
					Resource: "",
				}

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionNoOpinion))
				Expect(reason).To(BeEmpty())
			})

			It("should have no opinion because resource is irrelevant", func() {
				attrs := auth.AttributesRecord{
					User:            gardenletUser,
					APIGroup:        "",
					Resource:        "",
					ResourceRequest: true,
				}

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionNoOpinion))
				Expect(reason).To(BeEmpty())
			})
		})

		testCommonAccess := func() {
			Context("when requested for CloudProfiles", func() {
				var (
					cloudProfileName string
					attrs            *auth.AttributesRecord
				)

				BeforeEach(func() {
					cloudProfileName = "fooCloud"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            cloudProfileName,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "cloudprofiles",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				DescribeTable("should return correct result if path exists",
					func(verb string) {
						attrs.Verb = verb

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeCloudProfile, "", cloudProfileName, graphpkg.VertexTypeSeed, "", seedName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
				)

				DescribeTable("should have no opinion because no allowed verb", func(verb string) {
					attrs.Verb = verb
					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get list watch]"))
				},
					Entry("create", "create"),
					Entry("update", "update"),
					Entry("patch", "patch"),
					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because path to seed does not exists", func() {
					graph.EXPECT().HasPathFrom(graphpkg.VertexTypeCloudProfile, "", cloudProfileName, graphpkg.VertexTypeSeed, "", seedName).Return(false)

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("no relationship found"))
				})

				It("should have no opinion because request is for a subresource", func() {
					attrs.Subresource = "status"

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: []"))
				})

				It("should have no opinion because no resource name is given", func() {
					attrs.Name = ""

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("No Object name found"))
				})
			})

			Context("when requested for ConfigMaps", func() {
				var (
					name, namespace string
					attrs           *auth.AttributesRecord
				)

				BeforeEach(func() {
					name, namespace = "foo", "bar"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						Namespace:       namespace,
						APIGroup:        corev1.SchemeGroupVersion.Group,
						Resource:        "configmaps",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				It("should allow because cluster-identity is retrieved", func() {
					attrs.Name = "cluster-identity"
					attrs.Namespace = "kube-system"

					graph.EXPECT().HasPathFrom(graphpkg.VertexTypeConfigMap, attrs.Namespace, attrs.Name, graphpkg.VertexTypeSeed, "", seedName).Return(true)

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				It("should allow without consulting the graph because verb is create", func() {
					attrs.Verb = "create"

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				It("should allow when verb is delete and resource does not exist", func() {
					attrs.Verb = "delete"

					graph.EXPECT().HasVertex(graphpkg.VertexTypeConfigMap, namespace, name).Return(false)
					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				DescribeTable("should return correct result if path exists",
					func(verb string) {
						attrs.Verb = verb

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeConfigMap, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(true)

						decision, reason, err := authorizer.Authorize(ctx, attrs)

						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("patch", "patch"),
					Entry("update", "update"),
					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
				)

				DescribeTable("should have no opinion because no allowed verb",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)

						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [create get patch update delete list watch]"))
					},

					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because path to seed does not exists", func() {
					graph.EXPECT().HasPathFrom(graphpkg.VertexTypeConfigMap, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(false)

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("no relationship found"))
				})

				It("should have no opinion because request is for a subresource", func() {
					attrs.Subresource = "status"

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: []"))
				})

				It("should have no opinion because no resource name is given", func() {
					attrs.Name = ""

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("No Object name found"))
				})
			})

			Context("when requested for SecretBindings", func() {
				var (
					name, namespace string
					attrs           *auth.AttributesRecord
				)

				BeforeEach(func() {
					name, namespace = "foo", "bar"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						Namespace:       namespace,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "secretbindings",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				DescribeTable("should return correct result if path exists",
					func(verb string) {
						attrs.Verb = verb

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeSecretBinding, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(true)

						decision, reason, err := authorizer.Authorize(ctx, attrs)

						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
				)

				DescribeTable("should have no opinion because no allowed verb",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)

						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get list watch]"))
					},

					Entry("create", "create"),
					Entry("patch", "patch"),
					Entry("update", "update"),
					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because path to seed does not exists", func() {
					graph.EXPECT().HasPathFrom(graphpkg.VertexTypeSecretBinding, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(false)

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("no relationship found"))
				})

				It("should have no opinion because request is for a subresource", func() {
					attrs.Subresource = "status"

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: []"))
				})

				It("should have no opinion because no resource name is given", func() {
					attrs.Name = ""

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("No Object name found"))
				})
			})

			Context("when requested for CredentialsBindings", func() {
				var (
					name, namespace string
					attrs           *auth.AttributesRecord
				)

				BeforeEach(func() {
					name, namespace = "foo", "bar"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						Namespace:       namespace,
						APIGroup:        securityv1alpha1.SchemeGroupVersion.Group,
						Resource:        "credentialsbindings",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				DescribeTable("should return correct result if path exists",
					func(verb string) {
						attrs.Verb = verb

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeCredentialsBinding, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(true)

						decision, reason, err := authorizer.Authorize(ctx, attrs)

						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
				)

				DescribeTable("should have no opinion because no allowed verb",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)

						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get list watch]"))
					},

					Entry("create", "create"),
					Entry("patch", "patch"),
					Entry("update", "update"),
					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because path to seed does not exists", func() {
					graph.EXPECT().HasPathFrom(graphpkg.VertexTypeCredentialsBinding, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(false)

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("no relationship found"))
				})

				It("should have no opinion because request is for a subresource", func() {
					attrs.Subresource = "status"

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: []"))
				})

				It("should have no opinion because no resource name is given", func() {
					attrs.Name = ""

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("No Object name found"))
				})
			})

			Context("when requested for WorkloadIdentities", func() {
				var (
					name, namespace string
					attrs           *auth.AttributesRecord
				)

				BeforeEach(func() {
					name, namespace = "foo", "bar"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						Namespace:       namespace,
						APIGroup:        securityv1alpha1.SchemeGroupVersion.Group,
						Resource:        "workloadidentities",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				DescribeTable("should return correct result if path exists",
					func(verb string) {
						attrs.Verb = verb

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeWorkloadIdentity, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(true)

						decision, reason, err := authorizer.Authorize(ctx, attrs)

						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
					Entry("create", "create"),
				)

				It("should allow the request when the request is for 'token' subresource and path exists", func() {
					attrs.Subresource = "token"
					attrs.Verb = "create"

					graph.EXPECT().HasPathFrom(graphpkg.VertexTypeWorkloadIdentity, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(true)

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				DescribeTable("should have no opinion because no allowed verb",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)

						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get list watch create]"))
					},

					Entry("patch", "patch"),
					Entry("update", "update"),
					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because path to seed does not exists", func() {
					graph.EXPECT().HasPathFrom(graphpkg.VertexTypeWorkloadIdentity, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(false)

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("no relationship found"))
				})

				It("should have no opinion because request is for a subresource", func() {
					attrs.Subresource = "status"

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: [token]"))
				})

				It("should have no opinion because no resource name is given", func() {
					attrs.Name = ""

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("No Object name found"))
				})
			})

			Context("when requested for ShootStates", func() {
				var (
					name, namespace string
					attrs           *auth.AttributesRecord
				)

				BeforeEach(func() {
					name, namespace = "foo", "bar"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						Namespace:       namespace,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "shootstates",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				It("should allow because verb is create", func() {
					attrs.Verb = "create"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				It("should allow when verb is delete and resource does not exist", func() {
					attrs.Verb = "delete"

					graph.EXPECT().HasVertex(graphpkg.VertexTypeShootState, namespace, name).Return(false)
					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				DescribeTable("should return correct result if path exists",
					func(verb string) {
						attrs.Verb = verb

						if verb == "delete" {
							graph.EXPECT().HasVertex(graphpkg.VertexTypeShootState, namespace, name).Return(true).Times(2)
						}

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeShootState, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeShootState, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(false)
						decision, reason, err = authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("no relationship found"))
					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
					Entry("patch", "patch"),
					Entry("update", "update"),
					Entry("delete", "delete"),
				)

				DescribeTable("should have no opinion because no allowed verb",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [create get update patch delete list watch]"))
					},

					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because request is for a subresource", func() {
					attrs.Subresource = "status"

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: []"))
				})

				It("should have no opinion because no resource name is given", func() {
					attrs.Name = ""

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("No Object name found"))
				})
			})

			Context("when requested for NamespacedCloudProfiles", func() {
				var (
					namespacedCloudProfileName, namespace string
					attrs                                 *auth.AttributesRecord
				)

				BeforeEach(func() {
					namespacedCloudProfileName = "fooCloud"
					namespace = "bar"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            namespacedCloudProfileName,
						Namespace:       namespace,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "namespacedcloudprofiles",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				DescribeTable("should return correct result if path exists",
					func(verb string) {
						attrs.Verb = verb

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeNamespacedCloudProfile, namespace, namespacedCloudProfileName, graphpkg.VertexTypeSeed, "", seedName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
				)

				DescribeTable("should have no opinion because no allowed verb", func(verb string) {
					attrs.Verb = verb
					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get list watch]"))
				},
					Entry("create", "create"),
					Entry("update", "update"),
					Entry("patch", "patch"),
					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because path to seed does not exists", func() {
					graph.EXPECT().HasPathFrom(graphpkg.VertexTypeNamespacedCloudProfile, namespace, namespacedCloudProfileName, graphpkg.VertexTypeSeed, "", seedName).Return(false)

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("no relationship found"))
				})
			})

			Context("when requested for Namespaces", func() {
				var (
					name  string
					attrs *auth.AttributesRecord
				)

				BeforeEach(func() {
					name = "foo"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						APIGroup:        corev1.SchemeGroupVersion.Group,
						Resource:        "namespaces",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				DescribeTable("should return correct result if path exists",
					func(verb string) {
						attrs.Verb = verb

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeNamespace, "", name, graphpkg.VertexTypeSeed, "", seedName).Return(true)

						decision, reason, err := authorizer.Authorize(ctx, attrs)

						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
				)

				DescribeTable("should have no opinion because no allowed verb",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)

						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get list watch]"))
					},

					Entry("create", "create"),
					Entry("patch", "patch"),
					Entry("update", "update"),
					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because path to seed does not exists", func() {
					graph.EXPECT().HasPathFrom(graphpkg.VertexTypeNamespace, "", name, graphpkg.VertexTypeSeed, "", seedName).Return(false)

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("no relationship found"))
				})

				It("should have no opinion because no resources requested", func() {
					attrs.Subresource = "status"

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: []"))
				})

				It("should have no opinion because no resource name is given", func() {
					attrs.Name = ""

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("No Object name found"))
				})
			})

			Context("when requested for Projects", func() {
				var (
					name  string
					attrs *auth.AttributesRecord
				)

				BeforeEach(func() {
					name = "foo"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "projects",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				DescribeTable("should return correct result if path exists",
					func(verb string) {
						attrs.Verb = verb

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeProject, "", name, graphpkg.VertexTypeSeed, "", seedName).Return(true)

						decision, reason, err := authorizer.Authorize(ctx, attrs)

						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
				)

				DescribeTable("should have no opinion because no allowed verb",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)

						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get list watch]"))
					},

					Entry("create", "create"),
					Entry("patch", "patch"),
					Entry("update", "update"),
					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because path to seed does not exists", func() {
					graph.EXPECT().HasPathFrom(graphpkg.VertexTypeProject, "", name, graphpkg.VertexTypeSeed, "", seedName).Return(false)

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("no relationship found"))
				})

				It("should have no opinion because no resources requested", func() {
					attrs.Subresource = "status"

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: []"))
				})

				It("should have no opinion because no resource name is given", func() {
					attrs.Name = ""

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("No Object name found"))
				})
			})

			Context("when requested for BackupBuckets", func() {
				var (
					name  string
					attrs *auth.AttributesRecord
				)

				BeforeEach(func() {
					name = "foo"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "backupbuckets",
						ResourceRequest: true,
						Verb:            "list",
					}
				})

				DescribeTable("should allow without consulting the graph because verb is get, list, watch, create",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
					Entry("create", "create"),
				)

				DescribeTable("should have no opinion because no allowed verb",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [create get list watch update patch delete]"))
					},

					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because no allowed subresource", func() {
					attrs.Subresource = "foo"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: [status]"))
				})

				It("should allow when verb is delete and resource does not exist", func() {
					attrs.Verb = "delete"

					graph.EXPECT().HasVertex(graphpkg.VertexTypeBackupBucket, "", name).Return(false)
					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				DescribeTable("should return correct result if path exists",
					func(verb, subresource string) {
						attrs.Verb = verb
						attrs.Subresource = subresource

						if verb == "delete" {
							graph.EXPECT().HasVertex(graphpkg.VertexTypeBackupBucket, "", name).Return(true).Times(2)
						}

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeBackupBucket, "", name, graphpkg.VertexTypeSeed, "", seedName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeBackupBucket, "", name, graphpkg.VertexTypeSeed, "", seedName).Return(false)
						decision, reason, err = authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("no relationship found"))
					},

					Entry("patch w/o subresource", "patch", ""),
					Entry("patch w/ subresource", "patch", "status"),
					Entry("update w/o subresource", "update", ""),
					Entry("update w/ subresource", "update", "status"),
					Entry("delete", "delete", ""),
				)
			})

			Context("when requested for BackupEntries", func() {
				var (
					name, namespace string
					attrs           *auth.AttributesRecord
				)

				BeforeEach(func() {
					name, namespace = "foo", "bar"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						Namespace:       namespace,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "backupentries",
						ResourceRequest: true,
						Verb:            "list",
					}
				})

				DescribeTable("should allow without consulting the graph because verb is get, list, watch, create",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
					Entry("create", "create"),
				)

				DescribeTable("should have no opinion because verb is not allowed",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [create get list watch update patch delete]"))

					},

					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because no allowed subresource", func() {
					attrs.Subresource = "foo"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: [status]"))
				})

				It("should allow when verb is delete and resource does not exist", func() {
					attrs.Verb = "delete"

					graph.EXPECT().HasVertex(graphpkg.VertexTypeBackupEntry, namespace, name).Return(false)
					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				DescribeTable("should return correct result if path exists",
					func(verb, subresource string) {
						attrs.Verb = verb
						attrs.Subresource = subresource

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeBackupEntry, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeBackupEntry, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(false)
						decision, reason, err = authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("no relationship found"))
					},

					Entry("patch w/o subresource", "patch", ""),
					Entry("patch w/ subresource", "patch", "status"),
					Entry("update w/o subresource", "update", ""),
					Entry("update w/ subresource", "update", "status"),
				)
			})

			Context("when requested for ExposureClasses", func() {
				var (
					exposureClassName string
					attrs             *auth.AttributesRecord
				)

				BeforeEach(func() {
					exposureClassName = "fooExposureClass"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            exposureClassName,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "exposureclasses",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				DescribeTable("should return correct result if path exists",
					func(verb string) {
						attrs.Verb = verb

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeExposureClass, "", exposureClassName, graphpkg.VertexTypeSeed, "", seedName).Return(true)

						decision, reason, err := authorizer.Authorize(ctx, attrs)

						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
				)

				DescribeTable("should have no opinion because no allowed verb",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)

						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get list watch]"))
					},

					Entry("create", "create"),
					Entry("patch", "patch"),
					Entry("update", "update"),
					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because path to seed does not exists", func() {
					graph.EXPECT().HasPathFrom(graphpkg.VertexTypeExposureClass, "", exposureClassName, graphpkg.VertexTypeSeed, "", seedName).Return(false)

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("no relationship found"))
				})

				It("should have no opinion because request is for a subresource", func() {
					attrs.Subresource = "status"

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: []"))
				})

				It("should have no opinion because no resource name is given", func() {
					attrs.Name = ""

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("No Object name found"))
				})
			})

			Context("when requested for Bastions", func() {
				var (
					name, namespace string
					attrs           *auth.AttributesRecord
				)

				BeforeEach(func() {
					name, namespace = "foo", "bar"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						Namespace:       namespace,
						APIGroup:        operationsv1alpha1.SchemeGroupVersion.Group,
						Resource:        "bastions",
						ResourceRequest: true,
						Verb:            "list",
					}
				})

				DescribeTable("should allow with consulting the graph because verb is get, list, watch, create",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
					Entry("create", "create"),
				)

				DescribeTable("should deny because verb is not allowed",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [create get list watch update patch]"))

					},

					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because no allowed subresource", func() {
					attrs.Subresource = "foo"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: [status]"))
				})

				DescribeTable("should return correct result if path exists",
					func(verb, subresource string) {
						attrs.Verb = verb
						attrs.Subresource = subresource

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeBastion, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeBastion, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(false)
						decision, reason, err = authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("no relationship found"))
					},

					Entry("patch w/o subresource", "patch", ""),
					Entry("patch w/ subresource", "patch", "status"),
					Entry("update w/o subresource", "update", ""),
					Entry("update w/ subresource", "update", "status"),
				)
			})

			Context("when requested for ManagedSeeds", func() {
				var (
					name, namespace string
					attrs           *auth.AttributesRecord
				)

				BeforeEach(func() {
					name, namespace = "foo", "bar"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						Namespace:       namespace,
						APIGroup:        seedmanagementv1alpha1.SchemeGroupVersion.Group,
						Resource:        "managedseeds",
						ResourceRequest: true,
						Verb:            "list",
					}
				})

				DescribeTable("should allow without consulting the graph because verb is get, list, or watch",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
				)

				DescribeTable("should have no opinion because verb is not allowed",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get list watch update patch]"))

					},

					Entry("create", "create"),
					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because no allowed subresource", func() {
					attrs.Subresource = "foo"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: [status]"))
				})

				DescribeTable("should return correct result if path exists",
					func(verb, subresource string) {
						attrs.Verb = verb
						attrs.Subresource = subresource

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeManagedSeed, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeManagedSeed, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(false)
						decision, reason, err = authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("no relationship found"))
					},

					Entry("patch w/o subresource", "patch", ""),
					Entry("patch w/ subresource", "patch", "status"),
					Entry("update w/o subresource", "update", ""),
					Entry("update w/ subresource", "update", "status"),
				)
			})

			Context("when requested for Gardenlets", func() {
				var (
					name, namespace string
					attrs           *auth.AttributesRecord
				)

				BeforeEach(func() {
					name, namespace = "foo", "bar"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						Namespace:       namespace,
						APIGroup:        seedmanagementv1alpha1.SchemeGroupVersion.Group,
						Resource:        "gardenlets",
						ResourceRequest: true,
						Verb:            "list",
					}
				})

				DescribeTable("should allow without consulting the graph because verb is get, list, watch, or create",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
					Entry("create", "create"),
				)

				DescribeTable("should have no opinion because verb is not allowed",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get list watch create update patch]"))

					},

					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because no allowed subresource", func() {
					attrs.Subresource = "foo"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: [status]"))
				})

				DescribeTable("should return correct result if path exists",
					func(verb, subresource string) {
						attrs.Verb = verb
						attrs.Subresource = subresource

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeGardenlet, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeGardenlet, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(false)
						decision, reason, err = authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("no relationship found"))
					},

					Entry("patch w/o subresource", "patch", ""),
					Entry("patch w/ subresource", "patch", "status"),
					Entry("update w/o subresource", "update", ""),
					Entry("update w/ subresource", "update", "status"),
				)
			})

			Context("when requested for ControllerInstallations", func() {
				var (
					name  string
					attrs *auth.AttributesRecord
				)

				BeforeEach(func() {
					name = "foo"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "controllerinstallations",
						ResourceRequest: true,
						Verb:            "list",
					}
				})

				DescribeTable("should allow without consulting the graph because verb is get, list, or watch",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
				)

				DescribeTable("should have no opinion because verb is not allowed",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get list watch update patch]"))

					},

					Entry("create", "create"),
					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because no allowed subresource", func() {
					attrs.Subresource = "foo"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: [status]"))
				})

				DescribeTable("should return correct result if path exists",
					func(verb, subresource string) {
						attrs.Verb = verb
						attrs.Subresource = subresource

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeControllerInstallation, "", name, graphpkg.VertexTypeSeed, "", seedName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeControllerInstallation, "", name, graphpkg.VertexTypeSeed, "", seedName).Return(false)
						decision, reason, err = authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("no relationship found"))
					},

					Entry("patch w/o subresource", "patch", ""),
					Entry("patch w/ subresource", "patch", "status"),
					Entry("update w/o subresource", "update", ""),
					Entry("update w/ subresource", "update", "status"),
				)
			})

			Context("when requested for corev1.Events", func() {
				var (
					name, namespace string
					attrs           *auth.AttributesRecord
				)

				BeforeEach(func() {
					name, namespace = "foo", "bar"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						Namespace:       namespace,
						APIGroup:        corev1.SchemeGroupVersion.Group,
						Resource:        "events",
						ResourceRequest: true,
						Verb:            "create",
					}
				})

				DescribeTable("should allow without consulting the graph because verb is create",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("create", "create"),
					Entry("patch", "patch"),
				)

				DescribeTable("should have no opinion because verb is not allowed",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [create patch]"))

					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
					Entry("update", "update"),
					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because request is for a subresource", func() {
					attrs.Subresource = "status"

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: []"))
				})
			})

			Context("when requested for events.k8s.io/v1.Events", func() {
				var (
					name, namespace string
					attrs           *auth.AttributesRecord
				)

				BeforeEach(func() {
					name, namespace = "foo", "bar"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						Namespace:       namespace,
						APIGroup:        eventsv1.SchemeGroupVersion.Group,
						Resource:        "events",
						ResourceRequest: true,
						Verb:            "create",
					}
				})

				DescribeTable("should allow without consulting the graph because verb is create",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("create", "create"),
					Entry("patch", "patch"),
				)

				DescribeTable("should have no opinion because verb is not allowed",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [create patch]"))

					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
					Entry("update", "update"),
					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because request is for a subresource", func() {
					attrs.Subresource = "status"

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: []"))
				})
			})

			Context("when requested for Shoots", func() {
				var (
					name, namespace string
					attrs           *auth.AttributesRecord
				)

				BeforeEach(func() {
					name, namespace = "foo", "bar"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						Namespace:       namespace,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "shoots",
						ResourceRequest: true,
						Verb:            "list",
					}
				})

				DescribeTable("should allow without consulting the graph because verb is get, list, or watch",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
				)

				DescribeTable("should deny because verb is not allowed",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get list watch update patch]"))

					},

					Entry("create", "create"),
					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because no allowed subresource", func() {
					attrs.Subresource = "foo"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: [status]"))
				})

				DescribeTable("should return correct result if path exists",
					func(verb, subresource string) {
						attrs.Verb = verb
						attrs.Subresource = subresource

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeShoot, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeShoot, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(false)
						decision, reason, err = authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("no relationship found"))
					},

					Entry("patch w/o subresource", "patch", ""),
					Entry("patch w/ subresource", "patch", "status"),
					Entry("update w/o subresource", "update", ""),
					Entry("update w/ subresource", "update", "status"),
				)
			})

			Context("when requested for Seeds", func() {
				var (
					name  string
					attrs *auth.AttributesRecord
				)

				BeforeEach(func() {
					name = "foo"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "seeds",
						ResourceRequest: true,
						Verb:            "list",
					}
				})

				DescribeTable("should allow without consulting the graph because verb is get, list, watch, create",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
					Entry("create", "create"),
				)

				It("should allow when verb is delete and resource does not exist", func() {
					attrs.Verb = "delete"

					graph.EXPECT().HasVertex(graphpkg.VertexTypeSeed, "", name).Return(false)
					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				DescribeTable("should return correct result if path exists",
					func(verb, subresource string) {
						attrs.Verb = verb
						attrs.Subresource = subresource

						if verb == "delete" {
							graph.EXPECT().HasVertex(graphpkg.VertexTypeSeed, "", name).Return(true).Times(2)
						}

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeSeed, "", name, graphpkg.VertexTypeSeed, "", seedName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeSeed, "", name, graphpkg.VertexTypeSeed, "", seedName).Return(false)
						decision, reason, err = authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("no relationship found"))
					},

					Entry("patch w/o subresource", "patch", ""),
					Entry("patch w/ subresource", "patch", "status"),
					Entry("update w/o subresource", "update", ""),
					Entry("update w/ subresource", "update", "status"),
					Entry("delete", "delete", ""),
				)

				It("should have no opinion because no allowed verb", func() {
					attrs.Verb = "deletecollection"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [create get list watch update patch delete]"))
				})

				It("should have no opinion because no allowed subresource", func() {
					attrs.Subresource = "foo"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: [status]"))
				})
			})

			Context("when requested for ControllerRegistrations", func() {
				var (
					name  string
					attrs *auth.AttributesRecord
				)

				BeforeEach(func() {
					name = "foo"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "controllerregistrations",
						ResourceRequest: true,
						Verb:            "list",
					}
				})

				DescribeTable("should allow without consulting the graph because verb is get, list, or watch",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
				)

				DescribeTable("should deny because verb is not allowed",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get list watch]"))

					},

					Entry("create", "create"),
					Entry("update", "update"),
					Entry("patch", "patch"),
					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because no allowed subresource", func() {
					attrs.Subresource = "foo"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: []"))
				})
			})

			Context("when requested for ControllerDeployments", func() {
				var (
					name  string
					attrs *auth.AttributesRecord
				)

				BeforeEach(func() {
					name = "foo"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "controllerdeployments",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				DescribeTable("should deny because verb is not allowed",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get list watch]"))

					},

					Entry("create", "create"),
					Entry("update", "update"),
					Entry("patch", "patch"),
					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because no allowed subresource", func() {
					attrs.Subresource = "foo"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: []"))
				})

				DescribeTable("should return correct result if path exists",
					func(verb string) {
						attrs.Verb = verb

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeControllerDeployment, "", name, graphpkg.VertexTypeSeed, "", seedName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeControllerDeployment, "", name, graphpkg.VertexTypeSeed, "", seedName).Return(false)
						decision, reason, err = authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("no relationship found"))
					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
				)
			})

			Context("when requested for Secrets", func() {
				var (
					name, namespace string
					attrs           *auth.AttributesRecord
				)

				BeforeEach(func() {
					name, namespace = "foo", "bar"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						Namespace:       namespace,
						APIGroup:        corev1.SchemeGroupVersion.Group,
						Resource:        "secrets",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				DescribeTable("should allow without consulting the graph because verb is get, list, or watch in the seed's namespace",
					func(verb string) {
						attrs.Namespace = "seed-" + seedName
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
				)

				It("should allow to delete the gardenlet's bootstrap tokens without consulting the graph", func() {
					attrs.Verb = "delete"
					attrs.Namespace = "kube-system"
					attrs.Name = "bootstrap-token-" + bootstraptoken.TokenID(metav1.ObjectMeta{Name: seedName, Namespace: v1beta1constants.GardenNamespace})

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				DescribeTable("should allow without consulting the graph because verb is create",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("create", "create"),
				)

				DescribeTable("should deny because verb is not allowed",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [create get patch update delete]"))

					},

					Entry("list", "list"),
					Entry("watch", "watch"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because no allowed subresource", func() {
					attrs.Subresource = "foo"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: []"))
				})

				It("should allow when verb is delete and resource does not exist", func() {
					attrs.Verb = "delete"

					graph.EXPECT().HasVertex(graphpkg.VertexTypeSecret, namespace, name).Return(false)
					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				DescribeTable("should return correct result if path exists",
					func(verb string) {
						attrs.Verb = verb

						if verb == "delete" {
							graph.EXPECT().HasVertex(graphpkg.VertexTypeSecret, namespace, name).Return(true).Times(2)
						}

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeSecret, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeSecret, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(false)
						decision, reason, err = authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("no relationship found"))
					},

					Entry("get", "get"),
					Entry("patch", "patch"),
					Entry("update", "update"),
					Entry("delete", "delete"),
				)
			})

			Context("when requested for InternalSecrets", func() {
				var (
					name, namespace string
					attrs           *auth.AttributesRecord
				)

				BeforeEach(func() {
					name, namespace = "foo", "bar"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						Namespace:       namespace,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "internalsecrets",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				It("should allow because verb is create", func() {
					attrs.Verb = "create"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				It("should allow when verb is delete and resource does not exist", func() {
					attrs.Verb = "delete"

					graph.EXPECT().HasVertex(graphpkg.VertexTypeInternalSecret, namespace, name).Return(false)
					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				DescribeTable("should return correct result if path exists",
					func(verb string) {
						attrs.Verb = verb

						if verb == "delete" {
							graph.EXPECT().HasVertex(graphpkg.VertexTypeInternalSecret, namespace, name).Return(true).Times(2)
						}

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeInternalSecret, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeInternalSecret, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(false)
						decision, reason, err = authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("no relationship found"))
					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
					Entry("patch", "patch"),
					Entry("update", "update"),
					Entry("delete", "delete"),
				)

				DescribeTable("should have no opinion because no allowed verb",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [create get update patch delete list watch]"))
					},

					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because request is for a subresource", func() {
					attrs.Subresource = "status"

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: []"))
				})

				It("should have no opinion because no resource name is given", func() {
					attrs.Name = ""

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("No Object name found"))
				})
			})
		}

		Context("gardenlet client", func() {
			BeforeEach(func() {
				seedUser = gardenletUser
			})

			testCommonAccess()

			Context("when requested for CertificateSigningRequests", func() {
				var (
					name  string
					attrs *auth.AttributesRecord
				)

				BeforeEach(func() {
					name = "foo"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						APIGroup:        certificatesv1.SchemeGroupVersion.Group,
						Resource:        "certificatesigningrequests",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				DescribeTable("should allow without consulting the graph because verb is create",
					func(verb, subresource string) {
						attrs.Verb = verb
						attrs.Subresource = subresource

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("create", "create", ""),
					Entry("create with subresource", "create", "seedclient"),
				)

				DescribeTable("should return correct result if path exists",
					func(verb, subresource string) {
						attrs.Verb = verb
						attrs.Subresource = subresource

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeCertificateSigningRequest, "", name, graphpkg.VertexTypeSeed, "", seedName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeCertificateSigningRequest, "", name, graphpkg.VertexTypeSeed, "", seedName).Return(false)
						decision, reason, err = authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("no relationship found"))
					},

					Entry("get", "get", ""),
					Entry("list", "list", ""),
					Entry("watch", "watch", ""),
					Entry("get with subresource", "get", "seedclient"),
				)

				DescribeTable("should deny because verb is not allowed",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [create get list watch]"))

					},

					Entry("update", "update"),
					Entry("patch", "patch"),
					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because no allowed subresource", func() {
					attrs.Subresource = "foo"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: [seedclient]"))
				})
			})

			Context("when requested for ClusterRoleBindings", func() {
				var (
					name  string
					attrs *auth.AttributesRecord
				)

				BeforeEach(func() {
					name = "fooClusterRoleBinding"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						APIGroup:        rbacv1.SchemeGroupVersion.Group,
						Resource:        "clusterrolebindings",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				It("should allow to delete the gardenlet's bootstrap cluster role binding without consulting the graph", func() {
					attrs.Verb = "delete"
					attrs.Name = "gardener.cloud:system:seed-bootstrapper:garden:" + seedName

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				It("should allow because path to seed exists", func() {
					graph.EXPECT().HasPathFrom(graphpkg.VertexTypeClusterRoleBinding, "", name, graphpkg.VertexTypeSeed, "", seedName).Return(true)

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				It("should have no opinion because path to seed does not exists", func() {
					graph.EXPECT().HasPathFrom(graphpkg.VertexTypeClusterRoleBinding, "", name, graphpkg.VertexTypeSeed, "", seedName).Return(false)

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("no relationship found"))
				})

				It("should allow without consulting the graph because verb is create", func() {
					attrs.Verb = "create"

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				DescribeTable("should deny because verb is not allowed",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [create get patch update]"))

					},

					Entry("list", "list"),
					Entry("watch", "watch"),
					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because no allowed subresource", func() {
					attrs.Subresource = "foo"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: []"))
				})

				It("should have no opinion because no resource name is given", func() {
					attrs.Name = ""

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("No Object name found"))
				})
			})

			Context("when requested for Leases", func() {
				var (
					name, namespace string
					attrs           *auth.AttributesRecord
				)

				BeforeEach(func() {
					name, namespace = "foo", "bar"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						Namespace:       namespace,
						APIGroup:        coordinationv1.SchemeGroupVersion.Group,
						Resource:        "leases",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				DescribeTable("should allow without consulting the graph because verb is create",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("create", "create"),
				)

				DescribeTable("should return correct result if path exists",
					func(verb string) {
						attrs.Verb = verb

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeLease, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeLease, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(false)
						decision, reason, err = authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("no relationship found"))
					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
					Entry("update", "update"),
					Entry("patch", "patch"),
				)

				DescribeTable("should have no opinion because verb is not allowed",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [create get update patch list watch]"))

					},
					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because no allowed subresource", func() {
					attrs.Subresource = "foo"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: []"))
				})
			})

			Context("when requested for ServiceAccounts", func() {
				var (
					name, namespace string
					attrs           *auth.AttributesRecord
				)

				BeforeEach(func() {
					name, namespace = "foo", "bar"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						Namespace:       namespace,
						APIGroup:        corev1.SchemeGroupVersion.Group,
						Resource:        "serviceaccounts",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				It("should allow to delete the gardenlet's bootstrap service account without consulting the graph", func() {
					attrs.Verb = "delete"
					attrs.Namespace = "garden"
					attrs.Name = "gardenlet-bootstrap-" + seedName

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				It("should allow because path to seed exists", func() {
					graph.EXPECT().HasPathFrom(graphpkg.VertexTypeServiceAccount, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(true)

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				It("should have no opinion because path to seed does not exists", func() {
					graph.EXPECT().HasPathFrom(graphpkg.VertexTypeServiceAccount, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(false)

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("no relationship found"))
				})

				It("should allow without consulting the graph because verb is create", func() {
					attrs.Verb = "create"

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				DescribeTable("should allow without consulting the graph because object is in the seed's namespace",
					func(verb string) {
						attrs.Namespace = "seed-" + seedName
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
					Entry("create", "create"),
					Entry("update", "update"),
					Entry("patch", "patch"),
					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should allow token subresource without consulting the graph because object is in the seed's namespace", func() {
					attrs.Namespace = "seed-" + seedName
					attrs.Verb = "create"
					attrs.Subresource = "token"

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				DescribeTable("should deny because verb is not allowed",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [create get patch update]"))

					},

					Entry("list", "list"),
					Entry("watch", "watch"),
					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because no allowed subresource", func() {
					attrs.Subresource = "foo"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: []"))
				})

				It("should have no opinion because no resource name is given", func() {
					attrs.Name = ""

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("No Object name found"))
				})
			})
		})

		Context("extension client", func() {
			BeforeEach(func() {
				seedUser = extensionUser
			})

			testCommonAccess()

			Context("when requested for CertificateSigningRequests", func() {
				var (
					name  string
					attrs *auth.AttributesRecord
				)

				BeforeEach(func() {
					name = "foo"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						APIGroup:        certificatesv1.SchemeGroupVersion.Group,
						Resource:        "certificatesigningrequests",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				DescribeTable("should allow read access if path exists",
					func(verb string) {
						attrs.Verb = verb

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeCertificateSigningRequest, "", name, graphpkg.VertexTypeSeed, "", seedName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())

						graph.EXPECT().HasPathFrom(graphpkg.VertexTypeCertificateSigningRequest, "", name, graphpkg.VertexTypeSeed, "", seedName).Return(false)
						decision, reason, err = authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("no relationship found"))
					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
				)

				DescribeTable("should deny because verb is not allowed",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get list watch]"))

					},

					Entry("create", "create"),
					Entry("update", "update"),
					Entry("patch", "patch"),
					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because no allowed subresource", func() {
					attrs.Subresource = "seedclient"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: []"))
				})
			})

			Context("when requested for ClusterRoleBindings", func() {
				var (
					name  string
					attrs *auth.AttributesRecord
				)

				BeforeEach(func() {
					name = "fooClusterRoleBinding"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						APIGroup:        rbacv1.SchemeGroupVersion.Group,
						Resource:        "clusterrolebindings",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				It("should allow because path to seed exists", func() {
					graph.EXPECT().HasPathFrom(graphpkg.VertexTypeClusterRoleBinding, "", name, graphpkg.VertexTypeSeed, "", seedName).Return(true)

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				It("should have no opinion because path to seed does not exists", func() {
					graph.EXPECT().HasPathFrom(graphpkg.VertexTypeClusterRoleBinding, "", name, graphpkg.VertexTypeSeed, "", seedName).Return(false)

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("no relationship found"))
				})

				DescribeTable("should deny because verb is not allowed",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get]"))
					},

					Entry("create", "create"),
					Entry("list", "list"),
					Entry("watch", "watch"),
					Entry("patch", "patch"),
					Entry("update", "update"),
					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because no allowed subresource", func() {
					attrs.Subresource = "foo"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: []"))
				})

				It("should have no opinion because no resource name is given", func() {
					attrs.Name = ""

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("No Object name found"))
				})
			})

			Context("when requested for Leases", func() {
				var (
					name, namespace string
					attrs           *auth.AttributesRecord
				)

				BeforeEach(func() {
					name, namespace = "foo", "seed-"+seedName
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						Namespace:       namespace,
						APIGroup:        coordinationv1.SchemeGroupVersion.Group,
						Resource:        "leases",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				DescribeTable("should allow without consulting the graph because verb is create",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("create", "create"),
				)

				DescribeTable("should allow because lease is in seed namespace",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("create", "create"),
					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
					Entry("update", "update"),
					Entry("patch", "patch"),
					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				DescribeTable("should have no opinion because verb is not allowed",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [create get list watch update patch delete deletecollection]"))
					},
					Entry("foo", "foo"),
				)

				It("should have no opinion because lease is not in seed namespace", func() {
					attrs.Verb = "create"
					attrs.Name = seedName
					attrs.Namespace = "gardener-system-seed-lease"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("lease object is not in seed namespace"))
				})
			})

			Context("when requested for ServiceAccounts", func() {
				var (
					name, namespace string
					attrs           *auth.AttributesRecord
				)

				BeforeEach(func() {
					name, namespace = "foo", "bar"
					attrs = &auth.AttributesRecord{
						User:            seedUser,
						Name:            name,
						Namespace:       namespace,
						APIGroup:        corev1.SchemeGroupVersion.Group,
						Resource:        "serviceaccounts",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				It("should allow because path to seed exists", func() {
					graph.EXPECT().HasPathFrom(graphpkg.VertexTypeServiceAccount, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(true)

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				It("should have no opinion because path to seed does not exists", func() {
					graph.EXPECT().HasPathFrom(graphpkg.VertexTypeServiceAccount, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(false)

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("no relationship found"))
				})

				DescribeTable("should deny because verb is not allowed",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get]"))

					},

					Entry("create", "create"),
					Entry("list", "list"),
					Entry("watch", "watch"),
					Entry("patch", "patch"),
					Entry("update", "update"),
					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because no allowed subresource", func() {
					attrs.Subresource = "token"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: []"))
				})

				It("should have no opinion because no resource name is given", func() {
					attrs.Name = ""

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("No Object name found"))
				})
			})
		})
	})
})

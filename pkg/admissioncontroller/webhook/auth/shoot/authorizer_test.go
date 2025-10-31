// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot_test

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apiserver/pkg/authentication/user"
	auth "k8s.io/apiserver/pkg/authorization/authorizer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhook/auth/shoot"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	graphutils "github.com/gardener/gardener/pkg/utils/graph"
	mockgraph "github.com/gardener/gardener/pkg/utils/graph/mock"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	authorizerwebhook "github.com/gardener/gardener/pkg/webhook/authorizer"
	fakeauthorizerwebhook "github.com/gardener/gardener/pkg/webhook/authorizer/fake"
)

var _ = Describe("Shoot", func() {
	var (
		ctx  context.Context
		ctrl *gomock.Controller

		log                      logr.Logger
		graph                    *mockgraph.MockInterface
		fakeClient               client.Client
		fakeWithSelectorsChecker authorizerwebhook.WithSelectorsChecker
		authorizer               auth.Authorizer

		shootNamespace string
		shootName      string
		gardenletUser  user.Info
		gardenadmUser  user.Info
	)

	BeforeEach(func() {
		ctx = context.Background()
		ctrl = gomock.NewController(GinkgoT())

		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		graph = mockgraph.NewMockInterface(ctrl)
		fakeClient = fake.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		fakeWithSelectorsChecker = fakeauthorizerwebhook.NewWithSelectorsChecker(true)
		authorizer = NewAuthorizer(log, fakeClient, graph, fakeWithSelectorsChecker)

		shootNamespace = "shoot-namespace"
		shootName = "shoot-name"
		gardenletUser = &user.DefaultInfo{
			Name:   "gardener.cloud:system:shoot:" + shootNamespace + ":" + shootName,
			Groups: []string{"gardener.cloud:system:shoots"},
		}
		gardenadmUser = &user.DefaultInfo{
			Name:   "gardener.cloud:gardenadm:shoot:" + shootNamespace + ":" + shootName,
			Groups: []string{"gardener.cloud:system:shoots"},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Authorize", func() {
		Context("when resource is unhandled", func() {
			It("should have no opinion because no shoot", func() {
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

		Context("gardenlet client", func() {
			Context("when requested for CertificateSigningRequests", func() {
				var (
					name  string
					attrs *auth.AttributesRecord
				)

				BeforeEach(func() {
					name = "foo"
					attrs = &auth.AttributesRecord{
						User:            gardenletUser,
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
					Entry("create with subresource", "create", "shootclient"),
				)

				DescribeTable("should return correct result if path exists",
					func(verb, subresource string) {
						attrs.Verb = verb
						attrs.Subresource = subresource

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeCertificateSigningRequest, "", name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeCertificateSigningRequest, "", name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(false)
						decision, reason, err = authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("no relationship found"))
					},

					Entry("get", "get", ""),
					Entry("list", "list", ""),
					Entry("watch", "watch", ""),
					Entry("get with subresource", "get", "shootclient"),
				)

				DescribeTable("should not have an opinion because verb is not allowed",
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
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: [shootclient]"))
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
						User:            gardenletUser,
						Name:            name,
						Namespace:       namespace,
						APIGroup:        corev1.SchemeGroupVersion.Group,
						Resource:        "configmaps",
						ResourceRequest: true,
						Verb:            "get",
					}
				})
				DescribeTable("should return correct result if path exists",
					func(verb string) {
						attrs.Verb = verb

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeConfigMap, namespace, name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(true)
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
					graph.EXPECT().HasPathFrom(graphutils.VertexTypeConfigMap, namespace, name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(false)

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

			Context("when requested for corev1.Events", func() {
				var (
					name, namespace string
					attrs           *auth.AttributesRecord
				)

				BeforeEach(func() {
					name, namespace = "foo", "bar"
					attrs = &auth.AttributesRecord{
						User:            gardenletUser,
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
						User:            gardenletUser,
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

			Context("when requested for Gardenlets", func() {
				var (
					name, namespace string
					attrs           *auth.AttributesRecord
				)

				BeforeEach(func() {
					name, namespace = "foo", shootNamespace
					attrs = &auth.AttributesRecord{
						User:            gardenletUser,
						Name:            name,
						Namespace:       namespace,
						APIGroup:        seedmanagementv1alpha1.SchemeGroupVersion.Group,
						Resource:        "gardenlets",
						ResourceRequest: true,
						Verb:            "list",
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

				DescribeTable("should have no opinion because verb is not allowed",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [create get list patch update watch]"))

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

				It("should have no opinion because namespace does not match", func() {
					attrs.Namespace = "not-allowed"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring(fmt.Sprintf("only the following namespaces are allowed for this resource type: [%s]", shootNamespace)))
				})

				DescribeTable("should allow list/watch requests if field selector is provided",
					func(verb string, withSelector bool) {
						attrs.Verb = verb

						if withSelector {
							selector, err := fields.ParseSelector("metadata.name=self-hosted-shoot-" + shootName)
							Expect(err).NotTo(HaveOccurred())
							attrs.FieldSelectorRequirements = selector.Requirements()
						}

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())

						if withSelector {
							Expect(decision).To(Equal(auth.DecisionAllow))
							Expect(reason).To(BeEmpty())
						} else {
							Expect(decision).To(Equal(auth.DecisionNoOpinion))
							Expect(reason).To(ContainSubstring("must specify field or label selector"))
						}
					},

					Entry("list w/ needed selector", "list", true),
					Entry("list w/o needed selector", "list", false),
				)

				DescribeTable("should return correct result if path exists",
					func(verb, subresource string) {
						attrs.Verb = verb
						attrs.Subresource = subresource

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeGardenlet, namespace, name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeGardenlet, namespace, name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(false)
						decision, reason, err = authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("no relationship found"))
					},

					Entry("get w/o subresource", "get", ""),
					Entry("get w/ subresource", "get", "status"),
					Entry("patch w/o subresource", "patch", ""),
					Entry("patch w/ subresource", "patch", "status"),
					Entry("update w/o subresource", "update", ""),
					Entry("update w/ subresource", "update", "status"),
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
						User:            gardenletUser,
						Name:            name,
						Namespace:       namespace,
						APIGroup:        corev1.SchemeGroupVersion.Group,
						Resource:        "secrets",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				When("deletion of bootstrap token secret is requested", func() {
					var bootstrapTokenSecret *corev1.Secret

					BeforeEach(func() {
						bootstrapTokenSecret = &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "bootstrap-token-abcdef",
								Namespace: "kube-system",
							},
						}

						attrs.Verb = "delete"
						attrs.Namespace = bootstrapTokenSecret.Namespace
						attrs.Name = bootstrapTokenSecret.Name
					})

					It("should allow if shoot meta matches", func() {
						bootstrapTokenSecret.Data = map[string][]byte{"description": []byte(fmt.Sprintf("Used for connecting the self-hosted Shoot %s/%s to the Garden cluster", shootNamespace, shootName))}
						Expect(fakeClient.Create(ctx, bootstrapTokenSecret)).To(Succeed())

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					})

					It("should not have an opinion if shoot meta does not match", func() {
						bootstrapTokenSecret.Data = map[string][]byte{"description": []byte("Used for connecting the self-hosted Shoot not-the-namespace/not-the-name to the Garden cluster")}
						Expect(fakeClient.Create(ctx, bootstrapTokenSecret)).To(Succeed())

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring(`shoot meta in bootstrap token secret not-the-namespace/not-the-name does not match with identity of requestor`))
					})

					It("should return an error if description cannot be fetched", func() {
						Expect(fakeClient.Create(ctx, bootstrapTokenSecret)).To(Succeed())

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).To(MatchError(ContainSubstring(`failed fetching shoot meta from bootstrap token description: bootstrap token description does not start with "Used for connecting the self-hosted Shoot "`)))
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(BeEmpty())
					})

					It("should return an error if secret is not found", func() {
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).To(BeNotFoundError())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(BeEmpty())
					})
				})

				DescribeTable("should have no opinion because no allowed verb",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
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
			})

			Context("when requested for Shoots", func() {
				var (
					name, namespace string
					attrs           *auth.AttributesRecord
				)

				BeforeEach(func() {
					name, namespace = shootName, shootNamespace
					attrs = &auth.AttributesRecord{
						User:            gardenletUser,
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

				DescribeTable("should have no opinion because verb is not allowed",
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
			})
		})

		Context("gardenadm client", func() {
			Context("when requested for CloudProfiles", func() {
				var attrs *auth.AttributesRecord

				BeforeEach(func() {
					attrs = &auth.AttributesRecord{
						User:            gardenadmUser,
						Name:            shootName,
						Namespace:       shootNamespace,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "cloudprofiles",
						ResourceRequest: true,
					}
				})

				It("should allow because verb is 'get'", func() {
					attrs.Verb = "get"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				It("should have no opinion because verb is not allowed", func() {
					attrs.Verb = "list"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(BeEmpty())
				})
			})

			Context("when requested for ConfigMaps", func() {
				var attrs *auth.AttributesRecord

				BeforeEach(func() {
					attrs = &auth.AttributesRecord{
						User:            gardenadmUser,
						Name:            shootName,
						Namespace:       shootNamespace,
						APIGroup:        corev1.SchemeGroupVersion.Group,
						Resource:        "configmaps",
						ResourceRequest: true,
					}
				})

				It("should allow because verb is 'create'", func() {
					attrs.Verb = "create"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				It("should have no opinion because verb is not allowed", func() {
					attrs.Verb = "get"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(BeEmpty())
				})

				It("should have no opinion because shoot info does match user info", func() {
					attrs.Verb = "create"
					attrs.Namespace = "other-namespace"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(BeEmpty())
				})
			})

			Context("when requested for CredentialsBindings", func() {
				var attrs *auth.AttributesRecord

				BeforeEach(func() {
					attrs = &auth.AttributesRecord{
						User:            gardenadmUser,
						Name:            shootName,
						Namespace:       shootNamespace,
						APIGroup:        securityv1alpha1.SchemeGroupVersion.Group,
						Resource:        "credentialsbindings",
						ResourceRequest: true,
					}
				})

				It("should allow because verb is 'create'", func() {
					attrs.Verb = "create"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				It("should have no opinion because verb is not allowed", func() {
					attrs.Verb = "get"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(BeEmpty())
				})

				It("should have no opinion because shoot info does match user info", func() {
					attrs.Verb = "create"
					attrs.Namespace = "other-namespace"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(BeEmpty())
				})
			})

			Context("when requested for Projects", func() {
				var attrs *auth.AttributesRecord

				BeforeEach(func() {
					attrs = &auth.AttributesRecord{
						User:            gardenadmUser,
						Name:            shootName,
						Namespace:       shootNamespace,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "projects",
						ResourceRequest: true,
					}
				})

				It("should allow because verb is 'create'", func() {
					attrs.Verb = "create"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				It("should have no opinion because verb is not allowed", func() {
					attrs.Verb = "list"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(BeEmpty())
				})
			})

			Context("when requested for Secrets", func() {
				var attrs *auth.AttributesRecord

				BeforeEach(func() {
					attrs = &auth.AttributesRecord{
						User:            gardenadmUser,
						Name:            shootName,
						Namespace:       shootNamespace,
						APIGroup:        corev1.SchemeGroupVersion.Group,
						Resource:        "secrets",
						ResourceRequest: true,
					}
				})

				It("should allow because verb is 'create'", func() {
					attrs.Verb = "create"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				It("should have no opinion because verb is not allowed", func() {
					attrs.Verb = "get"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(BeEmpty())
				})

				It("should have no opinion because shoot info does match user info", func() {
					attrs.Verb = "create"
					attrs.Namespace = "other-namespace"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(BeEmpty())
				})
			})

			Context("when requested for SecretBindings", func() {
				var attrs *auth.AttributesRecord

				BeforeEach(func() {
					attrs = &auth.AttributesRecord{
						User:            gardenadmUser,
						Name:            shootName,
						Namespace:       shootNamespace,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "secretbindings",
						ResourceRequest: true,
					}
				})

				It("should allow because verb is 'create'", func() {
					attrs.Verb = "create"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				It("should have no opinion because verb is not allowed", func() {
					attrs.Verb = "get"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(BeEmpty())
				})

				It("should have no opinion because shoot info does match user info", func() {
					attrs.Verb = "create"
					attrs.Namespace = "other-namespace"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(BeEmpty())
				})
			})

			Context("when requested for Shoots", func() {
				var attrs *auth.AttributesRecord

				BeforeEach(func() {
					attrs = &auth.AttributesRecord{
						User:            gardenadmUser,
						Name:            shootName,
						Namespace:       shootNamespace,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "shoots",
						ResourceRequest: true,
					}
				})

				DescribeTable("should allow because verb is allowed",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("create", "create"),
					Entry("mark-self-hosted", "mark-self-hosted"),
				)

				It("should have no opinion because verb is not allowed", func() {
					attrs.Verb = "get"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(BeEmpty())
				})

				It("should have no opinion because shoot info does match user info", func() {
					attrs.Verb = "create"
					attrs.Namespace = "other-namespace"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(BeEmpty())
				})
			})
		})
	})
})

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	certificatesv1 "k8s.io/api/certificates/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apiserver/pkg/authentication/user"
	auth "k8s.io/apiserver/pkg/authorization/authorizer"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhook/auth/shoot"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	graphutils "github.com/gardener/gardener/pkg/utils/graph"
	mockgraph "github.com/gardener/gardener/pkg/utils/graph/mock"
	authorizerwebhook "github.com/gardener/gardener/pkg/webhook/authorizer"
	fakeauthorizerwebhook "github.com/gardener/gardener/pkg/webhook/authorizer/fake"
)

var _ = Describe("Shoot", func() {
	var (
		ctx  context.Context
		ctrl *gomock.Controller

		log                      logr.Logger
		graph                    *mockgraph.MockInterface
		fakeWithSelectorsChecker authorizerwebhook.WithSelectorsChecker
		authorizer               auth.Authorizer

		shootNamespace string
		shootName      string
		gardenletUser  user.Info
	)

	BeforeEach(func() {
		ctx = context.Background()
		ctrl = gomock.NewController(GinkgoT())

		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		graph = mockgraph.NewMockInterface(ctrl)
		fakeWithSelectorsChecker = fakeauthorizerwebhook.NewWithSelectorsChecker(true)
		authorizer = NewAuthorizer(log, graph, fakeWithSelectorsChecker)

		shootNamespace = "shoot-namespace"
		shootName = "shoot-name"
		gardenletUser = &user.DefaultInfo{
			Name:   "gardener.cloud:system:shoot:" + shootNamespace + ":" + shootName,
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
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: [shootclient]"))
				})
			})

			Context("when requested for Gardenlets", func() {
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

				DescribeTable("should allow list/watch requests if field selector is provided",
					func(verb string, withSelector bool) {
						attrs.Verb = verb

						if withSelector {
							selector, err := fields.ParseSelector("metadata.name=autonomous-shoot-" + shootName)
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
		})
	})
})

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
	coordinationv1 "k8s.io/api/coordination/v1"
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
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
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
		fakeClient               client.Client
		fakeWithSelectorsChecker authorizerwebhook.WithSelectorsChecker
		authorizer               auth.Authorizer

		shootNamespace          string
		shootName               string
		extensionShootNamespace string
		gardenletUser           user.Info
		gardenadmUser           user.Info
		extensionUser           user.Info
	)

	BeforeEach(func() {
		ctx = context.Background()
		ctrl = gomock.NewController(GinkgoT())

		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		graph = mockgraph.NewMockInterface(ctrl)
		fakeClient = fake.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		fakeWithSelectorsChecker = fakeauthorizerwebhook.NewWithSelectorsChecker(true)
		authorizer = NewAuthorizer(log, fakeClient, graph, fakeWithSelectorsChecker, nil)

		shootNamespace = "shoot-namespace"
		shootName = "shoot-name"
		extensionShootNamespace = "garden-project"
		gardenletUser = &user.DefaultInfo{
			Name:   "gardener.cloud:system:shoot:" + shootNamespace + ":" + shootName,
			Groups: []string{"gardener.cloud:system:shoots"},
		}
		gardenadmUser = &user.DefaultInfo{
			Name:   "gardener.cloud:gardenadm:shoot:" + shootNamespace + ":" + shootName,
			Groups: []string{"gardener.cloud:system:shoots"},
		}
		extensionUser = &user.DefaultInfo{
			Name:   "system:serviceaccount:" + extensionShootNamespace + ":extension-shoot--" + shootName + "--foo",
			Groups: []string{"system:serviceaccounts"},
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

		Context("seed authorizer delegation", func() {
			var gardenExtensionUser user.Info

			BeforeEach(func() {
				gardenExtensionUser = &user.DefaultInfo{
					Name:   "system:serviceaccount:garden:extension-shoot--" + shootName + "--foo",
					Groups: []string{"system:serviceaccounts"},
				}
			})

			It("should delegate to the seed authorizer for extension requests when the shoot is also a seed", func() {
				seedAuthorizer := auth.AuthorizerFunc(func(_ context.Context, _ auth.Attributes) (auth.Decision, string, error) {
					return auth.DecisionAllow, "", nil
				})
				authorizer = NewAuthorizer(log, fakeClient, graph, fakeWithSelectorsChecker, seedAuthorizer)

				graph.EXPECT().HasVertex(graphutils.VertexTypeSeed, "", shootName).Return(true)

				decision, reason, err := authorizer.Authorize(ctx, auth.AttributesRecord{
					User:            gardenExtensionUser,
					Name:            shootName,
					APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
					Resource:        "seeds",
					ResourceRequest: true,
					Verb:            "patch",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
			})

			It("should fall through to shoot authorizer when seed authorizer has no opinion", func() {
				seedAuthorizer := auth.AuthorizerFunc(func(_ context.Context, _ auth.Attributes) (auth.Decision, string, error) {
					return auth.DecisionNoOpinion, "", nil
				})
				authorizer = NewAuthorizer(log, fakeClient, graph, fakeWithSelectorsChecker, seedAuthorizer)

				graph.EXPECT().HasVertex(graphutils.VertexTypeSeed, "", shootName).Return(true)

				decision, reason, err := authorizer.Authorize(ctx, auth.AttributesRecord{
					User:            gardenExtensionUser,
					Name:            shootName,
					APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
					Resource:        "seeds",
					ResourceRequest: true,
					Verb:            "get",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
			})

			It("should not delegate when the shoot is not a seed", func() {
				seedAuthorizer := auth.AuthorizerFunc(func(_ context.Context, _ auth.Attributes) (auth.Decision, string, error) {
					Fail("seed authorizer should not be called")
					return auth.DecisionNoOpinion, "", nil
				})
				authorizer = NewAuthorizer(log, fakeClient, graph, fakeWithSelectorsChecker, seedAuthorizer)

				graph.EXPECT().HasVertex(graphutils.VertexTypeSeed, "", shootName).Return(false)

				decision, reason, err := authorizer.Authorize(ctx, auth.AttributesRecord{
					User:            gardenExtensionUser,
					Name:            shootName,
					APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
					Resource:        "seeds",
					ResourceRequest: true,
					Verb:            "patch",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionNoOpinion))
				Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type"))
			})

			It("should not delegate for gardenlet requests even when the shoot is a seed", func() {
				seedAuthorizer := auth.AuthorizerFunc(func(_ context.Context, _ auth.Attributes) (auth.Decision, string, error) {
					Fail("seed authorizer should not be called")
					return auth.DecisionNoOpinion, "", nil
				})
				authorizer = NewAuthorizer(log, fakeClient, graph, fakeWithSelectorsChecker, seedAuthorizer)

				decision, reason, err := authorizer.Authorize(ctx, auth.AttributesRecord{
					User:            gardenletUser,
					Name:            shootName,
					APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
					Resource:        "seeds",
					ResourceRequest: true,
					Verb:            "get",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
			})

			It("should not delegate for extension requests when shoot is not in the garden namespace", func() {
				seedAuthorizer := auth.AuthorizerFunc(func(_ context.Context, _ auth.Attributes) (auth.Decision, string, error) {
					Fail("seed authorizer should not be called")
					return auth.DecisionNoOpinion, "", nil
				})
				authorizer = NewAuthorizer(log, fakeClient, graph, fakeWithSelectorsChecker, seedAuthorizer)

				decision, reason, err := authorizer.Authorize(ctx, auth.AttributesRecord{
					User:            extensionUser,
					Name:            shootName,
					APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
					Resource:        "seeds",
					ResourceRequest: true,
					Verb:            "patch",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionNoOpinion))
				Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type"))
			})
		})

		Context("gardenlet client", func() {
			Context("when requested for BackupBuckets", func() {
				var (
					name  string
					attrs *auth.AttributesRecord
				)

				BeforeEach(func() {
					name = "foo"
					attrs = &auth.AttributesRecord{
						User:            gardenletUser,
						Name:            name,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "backupbuckets",
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

				DescribeTable("should have no opinion because no allowed verb",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [create delete get list patch update watch]"))
					},

					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because no allowed subresource", func() {
					attrs.Subresource = "foo"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: [finalizers status]"))
				})

				It("should allow when verb is delete and resource does not exist", func() {
					attrs.Verb = "delete"

					graph.EXPECT().HasVertex(graphutils.VertexTypeBackupBucket, "", name).Return(false)
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
							graph.EXPECT().HasVertex(graphutils.VertexTypeBackupBucket, "", name).Return(true).Times(2)
						}

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeBackupBucket, "", name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeBackupBucket, "", name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(false)
						decision, reason, err = authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("no relationship found"))
					},

					Entry("patch w/o subresource", "patch", ""),
					Entry("patch w/ status subresource", "patch", "status"),
					Entry("patch w/ finalizers subresource", "patch", "finalizers"),
					Entry("update w/o subresource", "update", ""),
					Entry("update w/ status subresource", "update", "status"),
					Entry("update w/ finalizers subresource", "update", "finalizers"),
					Entry("delete", "delete", ""),
				)

				DescribeTable("should allow list/watch requests if field selector is provided",
					func(verb string, withSelector bool) {
						attrs.Name = ""
						attrs.Verb = verb

						if withSelector {
							selector, err := fields.ParseSelector("spec.shootRef.name=" + shootName + ",spec.shootRef.namespace=" + shootNamespace)
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
			})

			Context("when requested for BackupEntries", func() {
				var (
					name, namespace string
					attrs           *auth.AttributesRecord
				)

				BeforeEach(func() {
					name, namespace = "kube-system--foo", "bar"
					attrs = &auth.AttributesRecord{
						User:            gardenletUser,
						Name:            name,
						Namespace:       namespace,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "backupentries",
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

				DescribeTable("should have no opinion because no allowed verb",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [create delete get list patch update watch]"))
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

					graph.EXPECT().HasVertex(graphutils.VertexTypeBackupEntry, namespace, name).Return(false)
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
							graph.EXPECT().HasVertex(graphutils.VertexTypeBackupEntry, namespace, name).Return(true).Times(2)
						}

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeBackupEntry, namespace, name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeBackupEntry, namespace, name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(false)
						decision, reason, err = authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("no relationship found"))
					},

					Entry("patch w/o subresource", "patch", ""),
					Entry("patch w/ status subresource", "patch", "status"),
					Entry("update w/o subresource", "update", ""),
					Entry("update w/ status subresource", "update", "status"),
					Entry("delete", "delete", ""),
				)

				DescribeTable("should allow list/watch requests if field selector is provided",
					func(verb string, withSelector bool) {
						attrs.Name = ""
						attrs.Verb = verb

						if withSelector {
							selector, err := fields.ParseSelector("spec.shootRef.name=" + shootName + ",spec.shootRef.namespace=" + shootNamespace)
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
			})

			When("requested for Bastions", func() {
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
						APIGroup:        operationsv1alpha1.SchemeGroupVersion.Group,
						Resource:        "bastions",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				DescribeTable("should have no opinion because no allowed verb",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get list patch update watch]"))
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

				It("should have no opinion because request is for a namespace the gardenlet is not responsible for", func() {
					attrs.Namespace = "other-namespace"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following namespaces are allowed for this resource type"))
				})

				DescribeTable("should return correct result if path exists",
					func(verb, subresource string) {
						attrs.Verb = verb
						attrs.Subresource = subresource

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeBastion, namespace, name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeBastion, namespace, name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(false)
						decision, reason, err = authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("no relationship found"))
					},

					Entry("get", "get", ""),
					Entry("patch w/o subresource", "patch", ""),
					Entry("patch w/ status subresource", "patch", "status"),
					Entry("update w/o subresource", "update", ""),
					Entry("update w/ status subresource", "update", "status"),
				)

				DescribeTable("should allow list/watch requests if field selector is provided",
					func(verb string, withSelector bool) {
						attrs.Name = ""
						attrs.Verb = verb

						if withSelector {
							selector, err := fields.ParseSelector("spec.shootRef.name=" + shootName)
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
					Entry("watch w/ needed selector", "watch", true),
					Entry("watch w/o needed selector", "watch", false),
				)
			})

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

			When("requested for ControllerInstallations", func() {
				var (
					name  string
					attrs *auth.AttributesRecord
				)

				BeforeEach(func() {
					name = "foo"
					attrs = &auth.AttributesRecord{
						User:            gardenletUser,
						Name:            name,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "controllerinstallations",
						ResourceRequest: true,
						Verb:            "list",
					}
				})

				DescribeTable("should have no opinion because no allowed verb",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get list patch update watch]"))
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

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeControllerInstallation, "", name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeControllerInstallation, "", name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(false)
						decision, reason, err = authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("no relationship found"))
					},

					Entry("get w/o subresource", "get", ""),
					Entry("update w/o subresource", "update", ""),
					Entry("update w/ status subresource", "update", "status"),
					Entry("patch w/o subresource", "patch", ""),
					Entry("patch w/ status subresource", "patch", "status"),
				)

				DescribeTable("should allow list/watch requests if field selector is provided",
					func(verb string, withSelector bool) {
						attrs.Name = ""
						attrs.Verb = verb

						if withSelector {
							selector, err := fields.ParseSelector("spec.shootRef.name=" + shootName + ",spec.shootRef.namespace=" + shootNamespace)
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
			})

			When("requested for ControllerDeployments", func() {
				var (
					name  string
					attrs *auth.AttributesRecord
				)

				BeforeEach(func() {
					name = "foo"
					attrs = &auth.AttributesRecord{
						User:            gardenletUser,
						Name:            name,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "controllerdeployments",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				DescribeTable("should return correct result if path exists",
					func(verb string) {
						attrs.Verb = verb

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeControllerDeployment, "", name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(true)
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

				It("should have no opinion because path to shoot does not exist", func() {
					graph.EXPECT().HasPathFrom(graphutils.VertexTypeControllerDeployment, "", name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(false)

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

			When("requested for ControllerRegistrations", func() {
				var (
					name  string
					attrs *auth.AttributesRecord
				)

				BeforeEach(func() {
					name = "foo"
					attrs = &auth.AttributesRecord{
						User:            gardenletUser,
						Name:            name,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "controllerregistrations",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				DescribeTable("should return correct result if path exists",
					func(verb string) {
						attrs.Verb = verb

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeControllerRegistration, "", name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(true)
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

				It("should have no opinion because path to shoot does not exist", func() {
					graph.EXPECT().HasPathFrom(graphutils.VertexTypeControllerRegistration, "", name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(false)

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

			When("requested for Seeds", func() {
				var attrs *auth.AttributesRecord

				BeforeEach(func() {
					attrs = &auth.AttributesRecord{
						User:            gardenletUser,
						Name:            shootName,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "seeds",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				DescribeTable("should allow reading the seed matching the shoot name",
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

				It("should allow list/watch without resource name", func() {
					attrs.Name = ""
					attrs.Verb = "list"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				It("should have no opinion because name does not match shoot name", func() {
					attrs.Name = "other-seed"

					graph.EXPECT().HasPathFrom(graphutils.VertexTypeSeed, "", "other-seed", graphutils.VertexTypeShoot, shootNamespace, shootName).Return(false)
					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("no relationship found"))
				})

				DescribeTable("should have no opinion because verb is not allowed",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [delete get list watch]"))
					},

					Entry("create", "create"),
					Entry("update", "update"),
					Entry("patch", "patch"),
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

			When("requested for Seeds", func() {
				var (
					name  string
					attrs *auth.AttributesRecord
				)

				BeforeEach(func() {
					name = "foo"
					attrs = &auth.AttributesRecord{
						User:            gardenletUser,
						Name:            name,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "seeds",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				DescribeTable("should have no opinion because verb is not allowed",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [delete get list watch]"))
					},

					Entry("create", "create"),
					Entry("update", "update"),
					Entry("patch", "patch"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because no allowed subresource", func() {
					attrs.Subresource = "foo"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: []"))
				})

				DescribeTable("should always allow list/watch requests",
					func(verb string) {
						attrs.Name = ""
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("list", "list"),
					Entry("watch", "watch"),
				)

				It("should allow when verb is delete and resource does not exist", func() {
					attrs.Verb = "delete"

					graph.EXPECT().HasVertex(graphutils.VertexTypeSeed, "", name).Return(false)
					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				DescribeTable("should return correct result if path exists",
					func(verb string) {
						attrs.Verb = verb

						if verb == "delete" {
							graph.EXPECT().HasVertex(graphutils.VertexTypeSeed, "", name).Return(true).Times(2)
						}

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeSeed, "", name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeSeed, "", name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(false)
						decision, reason, err = authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("no relationship found"))
					},

					Entry("get", "get"),
					Entry("delete", "delete"),
				)
			})

			When("requested for ManagedSeeds", func() {
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
						Resource:        "managedseeds",
						ResourceRequest: true,
						Verb:            "list",
					}
				})

				DescribeTable("should have no opinion because verb is not allowed",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get list patch update watch]"))
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

				It("should have no opinion because namespace does not match", func() {
					attrs.Namespace = "not-allowed"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring(fmt.Sprintf("only the following namespaces are allowed for this resource type: [%s]", shootNamespace)))
				})

				DescribeTable("should allow list/watch requests if field selector is provided",
					func(verb string, withSelector bool) {
						attrs.Name = ""
						attrs.Verb = verb

						if withSelector {
							selector, err := fields.ParseSelector(seedmanagement.ManagedSeedShootName + "=" + shootName)
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

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeManagedSeed, namespace, name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeManagedSeed, namespace, name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(false)
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

			Context("when requested for Leases", func() {
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

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeLease, namespace, name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeLease, namespace, name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(false)
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
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: []"))
				})

				Context("extension client", func() {
					BeforeEach(func() {
						attrs.User = extensionUser
						attrs.Namespace = extensionShootNamespace
						attrs.Name = shootName + "--provider-aws-leader-election"
					})

					DescribeTable("should allow when lease is in shoot namespace with correct name prefix and verb is allowed",
						func(verb string) {
							attrs.Verb = verb

							decision, reason, err := authorizer.Authorize(ctx, attrs)
							Expect(err).NotTo(HaveOccurred())
							Expect(decision).To(Equal(auth.DecisionAllow))
							Expect(reason).To(BeEmpty())
						},

						Entry("create", "create"),
						Entry("get", "get"),
						Entry("update", "update"),
					)

					It("should allow list/watch without a name in the shoot namespace",
						func() {
							attrs.Verb = "list"
							attrs.Name = ""

							decision, reason, err := authorizer.Authorize(ctx, attrs)
							Expect(err).NotTo(HaveOccurred())
							Expect(decision).To(Equal(auth.DecisionAllow))
							Expect(reason).To(BeEmpty())
						},
					)

					It("should have no opinion when lease name does not have the shoot name as prefix", func() {
						attrs.Verb = "get"
						attrs.Name = "other-shoot--provider-aws-leader-election"

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("lease object name does not have the shoot name as prefix"))
					})

					DescribeTable("should have no opinion when lease is outside shoot namespace",
						func(verb string) {
							attrs.Namespace = "other-namespace"
							attrs.Verb = verb

							decision, reason, err := authorizer.Authorize(ctx, attrs)
							Expect(err).NotTo(HaveOccurred())
							Expect(decision).To(Equal(auth.DecisionNoOpinion))
							Expect(reason).To(ContainSubstring("lease object is not in shoot namespace"))
						},

						Entry("get", "get"),
						Entry("create", "create"),
						Entry("update", "update"),
					)
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
						User:            gardenletUser,
						Name:            name,
						Namespace:       namespace,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "shootstates",
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

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeShootState, namespace, name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeShootState, namespace, name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(false)
						decision, reason, err = authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("no relationship found"))
					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
					Entry("patch", "patch"),
				)

				DescribeTable("should have no opinion because verb is not allowed",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [create get list patch watch]"))

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

			Context("when requested for WorkloadIdentities", func() {
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
						APIGroup:        securityv1alpha1.SchemeGroupVersion.Group,
						Resource:        "workloadidentities",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				DescribeTable("should return correct result if path exists",
					func(verb string) {
						attrs.Verb = verb

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeWorkloadIdentity, namespace, name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(true)

						decision, reason, err := authorizer.Authorize(ctx, attrs)

						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
					Entry("create", "create"),
					Entry("patch", "patch"),
				)

				It("should allow the request when the request is for 'token' subresource and path exists", func() {
					attrs.Subresource = "token"
					attrs.Verb = "create"

					graph.EXPECT().HasPathFrom(graphutils.VertexTypeWorkloadIdentity, namespace, name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(true)

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
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [create get list patch watch]"))
					},

					Entry("update", "update"),
					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because path to seed does not exists", func() {
					graph.EXPECT().HasPathFrom(graphutils.VertexTypeWorkloadIdentity, namespace, name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(false)

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

			Context("when requested for Namespaces", func() {
				var (
					name  string
					attrs *auth.AttributesRecord
				)

				BeforeEach(func() {
					name = "foo"
					attrs = &auth.AttributesRecord{
						User:            gardenletUser,
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

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeNamespace, "", name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(true)

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

				It("should have no opinion because path to shoot does not exist", func() {
					graph.EXPECT().HasPathFrom(graphutils.VertexTypeNamespace, "", name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(false)

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

			Context("when requested for Projects", func() {
				var (
					name  string
					attrs *auth.AttributesRecord
				)

				BeforeEach(func() {
					name = "foo"
					attrs = &auth.AttributesRecord{
						User:            gardenletUser,
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

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeProject, "", name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(true)

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

				It("should have no opinion because path to shoot does not exist", func() {
					graph.EXPECT().HasPathFrom(graphutils.VertexTypeProject, "", name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(false)

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
						bootstrapTokenSecret.Data = map[string][]byte{"description": fmt.Appendf(nil, "Used for connecting the self-hosted Shoot %s/%s to the Garden cluster", shootNamespace, shootName)}
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

					It("should fall through to graph-based authorization if description is not a self-hosted shoot token", func() {
						Expect(fakeClient.Create(ctx, bootstrapTokenSecret)).To(Succeed())

						graph.EXPECT().HasVertex(graphutils.VertexTypeSecret, bootstrapTokenSecret.Namespace, bootstrapTokenSecret.Name).Return(true)
						graph.EXPECT().HasPathFrom(graphutils.VertexTypeSecret, bootstrapTokenSecret.Namespace, bootstrapTokenSecret.Name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					})

					It("should fall through to graph-based authorization if secret is not found", func() {
						graph.EXPECT().HasVertex(graphutils.VertexTypeSecret, bootstrapTokenSecret.Namespace, bootstrapTokenSecret.Name).Return(false)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					})
				})

				It("should allow when verb is delete and resource does not exist", func() {
					attrs.Verb = "delete"

					graph.EXPECT().HasVertex(graphutils.VertexTypeSecret, namespace, name).Return(false)
					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				DescribeTable("should return correct result if path exists",
					func(verb string) {
						attrs.Verb = verb

						if verb == "delete" {
							graph.EXPECT().HasVertex(graphutils.VertexTypeSecret, namespace, name).Return(true)
						}

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeSecret, namespace, name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("get", "get"),
					Entry("list", "list"),
					Entry("watch", "watch"),
					Entry("patch", "patch"),
					Entry("update", "update"),
					Entry("delete", "delete"),
				)

				DescribeTable("should allow without consulting the graph because verb is get, list, or watch",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					},

					Entry("create", "create"),
				)

				DescribeTable("should have no opinion because no allowed verb",
					func(verb string) {
						attrs.Verb = verb

						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(Equal("only the following verbs are allowed for this resource type: [create delete get list patch update watch]"))
					},
					Entry("deletecollection", "deletecollection"),
				)
			})

			When("requested for ServiceAccounts", func() {
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
						Resource:        "serviceaccounts",
						ResourceRequest: true,
						Verb:            "get",
					}
				})

				It("should allow because path to shoot exists", func() {
					graph.EXPECT().HasPathFrom(graphutils.VertexTypeServiceAccount, namespace, name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(true)

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})

				It("should have no opinion because path to shoot does not exist", func() {
					graph.EXPECT().HasPathFrom(graphutils.VertexTypeServiceAccount, namespace, name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(false)

					decision, reason, err := authorizer.Authorize(ctx, attrs)

					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("no relationship found"))
				})

				DescribeTable("should allow without consulting the graph because object is an extension SA of this shoot in the shoot's project namespace",
					func(verb string) {
						attrs.Namespace = shootNamespace
						attrs.Name = "extension-shoot--" + shootName + "--provider-aws"
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

				It("should fall through to graph check because SA name does not have the shoot's extension prefix (different shoot's SA)", func() {
					attrs.Namespace = shootNamespace
					attrs.Name = "extension-shoot--other-shoot--provider-aws"
					graph.EXPECT().HasPathFrom(graphutils.VertexTypeServiceAccount, shootNamespace, "extension-shoot--other-shoot--provider-aws", graphutils.VertexTypeShoot, shootNamespace, shootName).Return(false)

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("no relationship found"))
				})

				Context("extension client", func() {
					BeforeEach(func() {
						attrs.User = extensionUser
					})

					It("should allow because path to shoot exists", func() {
						graph.EXPECT().HasPathFrom(graphutils.VertexTypeServiceAccount, namespace, name, graphutils.VertexTypeShoot, extensionShootNamespace, shootName).Return(true)

						decision, reason, err := authorizer.Authorize(ctx, attrs)

						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())
					})

					It("should have no opinion because path to shoot does not exist", func() {
						graph.EXPECT().HasPathFrom(graphutils.VertexTypeServiceAccount, namespace, name, graphutils.VertexTypeShoot, extensionShootNamespace, shootName).Return(false)

						decision, reason, err := authorizer.Authorize(ctx, attrs)

						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("no relationship found"))
					})

					DescribeTable("should not have an opinion because verb is not allowed",
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
				})
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
						Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [patch update]"))

					},

					Entry("create", "create"),
					Entry("delete", "delete"),
					Entry("deletecollection", "deletecollection"),
				)

				It("should have no opinion because no allowed subresource", func() {
					attrs.Subresource = "foo"
					attrs.Verb = "patch"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following subresources are allowed for this resource type: [status]"))
				})

				DescribeTable("should return correct result if path exists",
					func(verb, subresource string) {
						attrs.Verb = verb
						attrs.Subresource = subresource

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeShoot, namespace, name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(true)
						decision, reason, err := authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionAllow))
						Expect(reason).To(BeEmpty())

						graph.EXPECT().HasPathFrom(graphutils.VertexTypeShoot, namespace, name, graphutils.VertexTypeShoot, shootNamespace, shootName).Return(false)
						decision, reason, err = authorizer.Authorize(ctx, attrs)
						Expect(err).NotTo(HaveOccurred())
						Expect(decision).To(Equal(auth.DecisionNoOpinion))
						Expect(reason).To(ContainSubstring("no relationship found"))
					},

					Entry("patch w/o subresource", "patch", ""),
					Entry("patch w/ status subresource", "patch", "status"),
					Entry("update w/o subresource", "update", ""),
					Entry("update w/ status subresource", "update", "status"),
				)
			})
		})

		Context("gardenadm client", func() {
			Context("when requested for BackupBuckets", func() {
				var attrs *auth.AttributesRecord

				BeforeEach(func() {
					attrs = &auth.AttributesRecord{
						User:            gardenadmUser,
						Name:            shootName,
						Namespace:       shootNamespace,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "backupbuckets",
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

			Context("when requested for BackupEntries", func() {
				var attrs *auth.AttributesRecord

				BeforeEach(func() {
					attrs = &auth.AttributesRecord{
						User:            gardenadmUser,
						Name:            shootName,
						Namespace:       shootNamespace,
						APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
						Resource:        "backupentries",
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

				It("should allow status patches", func() {
					attrs.Verb = "patch"
					attrs.Subresource = "status"

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionAllow))
					Expect(reason).To(BeEmpty())
				})
			})
		})
	})
})

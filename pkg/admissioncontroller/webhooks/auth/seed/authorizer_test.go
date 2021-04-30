// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seed_test

import (
	"context"
	"fmt"

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhooks/auth/seed"
	graphpkg "github.com/gardener/gardener/pkg/admissioncontroller/webhooks/auth/seed/graph"
	mockgraph "github.com/gardener/gardener/pkg/admissioncontroller/webhooks/auth/seed/graph/mock"
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	eventsv1 "k8s.io/api/events/v1"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	auth "k8s.io/apiserver/pkg/authorization/authorizer"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var _ = Describe("Seed", func() {
	var (
		ctx  context.Context
		ctrl *gomock.Controller

		logger     logr.Logger
		graph      *mockgraph.MockInterface
		authorizer auth.Authorizer

		seedName                string
		seedUser, ambiguousUser user.Info
	)

	BeforeEach(func() {
		ctx = context.Background()
		ctrl = gomock.NewController(GinkgoT())

		logger = logzap.New(logzap.WriteTo(GinkgoWriter))
		graph = mockgraph.NewMockInterface(ctrl)
		authorizer = NewAuthorizer(logger, graph)

		seedName = "seed"
		seedUser = &user.DefaultInfo{
			Name:   fmt.Sprintf("%s%s", v1beta1constants.SeedUserNamePrefix, seedName),
			Groups: []string{v1beta1constants.SeedsGroup},
		}
		ambiguousUser = &user.DefaultInfo{
			Name:   fmt.Sprintf("%s%s", v1beta1constants.SeedUserNamePrefix, v1beta1constants.SeedUserNameSuffixAmbiguous),
			Groups: []string{v1beta1constants.SeedsGroup},
		}
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
					User:     seedUser,
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
					User:            seedUser,
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

			It("should allow because path to seed exists", func() {
				graph.EXPECT().HasPathFrom(graphpkg.VertexTypeCloudProfile, "", cloudProfileName, graphpkg.VertexTypeSeed, "", seedName).Return(true)

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
			})

			It("should have no opinion because path to seed does not exists", func() {
				graph.EXPECT().HasPathFrom(graphpkg.VertexTypeCloudProfile, "", cloudProfileName, graphpkg.VertexTypeSeed, "", seedName).Return(false)

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionNoOpinion))
				Expect(reason).To(ContainSubstring("no relationship found"))
			})

			It("should have no opinion because no get verb", func() {
				attrs.Verb = "list"

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionNoOpinion))
				Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get]"))
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

			It("should allow because seed name is ambiguous", func() {
				attrs.User = ambiguousUser

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
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

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
			})

			It("should allow because path to seed exists", func() {
				graph.EXPECT().HasPathFrom(graphpkg.VertexTypeConfigMap, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(true)

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
			})

			It("should have no opinion because path to seed does not exists", func() {
				graph.EXPECT().HasPathFrom(graphpkg.VertexTypeConfigMap, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(false)

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionNoOpinion))
				Expect(reason).To(ContainSubstring("no relationship found"))
			})

			It("should have no opinion because no get verb", func() {
				attrs.Verb = "list"

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionNoOpinion))
				Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get]"))
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

			It("should allow because seed name is ambiguous", func() {
				attrs.User = ambiguousUser

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
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

			It("should allow because path to seed exists", func() {
				graph.EXPECT().HasPathFrom(graphpkg.VertexTypeSecretBinding, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(true)

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
			})

			It("should have no opinion because path to seed does not exists", func() {
				graph.EXPECT().HasPathFrom(graphpkg.VertexTypeSecretBinding, namespace, name, graphpkg.VertexTypeSeed, "", seedName).Return(false)

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionNoOpinion))
				Expect(reason).To(ContainSubstring("no relationship found"))
			})

			It("should have no opinion because no get verb", func() {
				attrs.Verb = "list"

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionNoOpinion))
				Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get]"))
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

			It("should allow because seed name is ambiguous", func() {
				attrs.User = ambiguousUser

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
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
					APIGroup:        gardencorev1alpha1.SchemeGroupVersion.Group,
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

			DescribeTable("should return correct result if path exists",
				func(verb string) {
					attrs.Verb = verb

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
				Entry("patch", "patch"),
				Entry("update", "update"),
			)

			DescribeTable("should have no opinion because no allowed verb",
				func(verb string) {
					attrs.Verb = verb

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [create get update patch]"))
				},

				Entry("list", "list"),
				Entry("watch", "watch"),
				Entry("delete", "delete"),
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

			It("should allow because seed name is ambiguous", func() {
				attrs.User = ambiguousUser

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
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

			It("should allow because path to seed exists", func() {
				graph.EXPECT().HasPathFrom(graphpkg.VertexTypeNamespace, "", name, graphpkg.VertexTypeSeed, "", seedName).Return(true)

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
			})

			It("should have no opinion because path to seed does not exists", func() {
				graph.EXPECT().HasPathFrom(graphpkg.VertexTypeNamespace, "", name, graphpkg.VertexTypeSeed, "", seedName).Return(false)

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionNoOpinion))
				Expect(reason).To(ContainSubstring("no relationship found"))
			})

			It("should have no opinion because no get verb", func() {
				attrs.Verb = "list"

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionNoOpinion))
				Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get]"))
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

			It("should allow because seed name is ambiguous", func() {
				attrs.User = ambiguousUser

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
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

			It("should allow because path to seed exists", func() {
				graph.EXPECT().HasPathFrom(graphpkg.VertexTypeProject, "", name, graphpkg.VertexTypeSeed, "", seedName).Return(true)

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
			})

			It("should have no opinion because path to seed does not exists", func() {
				graph.EXPECT().HasPathFrom(graphpkg.VertexTypeProject, "", name, graphpkg.VertexTypeSeed, "", seedName).Return(false)

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionNoOpinion))
				Expect(reason).To(ContainSubstring("no relationship found"))
			})

			It("should have no opinion because no get verb", func() {
				attrs.Verb = "list"

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionNoOpinion))
				Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [get]"))
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

			It("should allow because seed name is ambiguous", func() {
				attrs.User = ambiguousUser

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
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

			DescribeTable("should return correct result if path exists",
				func(verb, subresource string) {
					attrs.Verb = verb
					attrs.Subresource = subresource

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
				Entry("delete w/o subresource", "delete", ""),
				Entry("delete w/ subresource", "delete", "status"),
			)

			It("should allow because seed name is ambiguous", func() {
				attrs.User = ambiguousUser

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
			})
		})

		Context("when requested for BackupEntrys", func() {
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

			It("should allow because seed name is ambiguous", func() {
				attrs.User = ambiguousUser

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
			})
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

			It("should allow because seed name is ambiguous", func() {
				attrs.User = ambiguousUser

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
			})
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

			It("should allow because seed name is ambiguous", func() {
				attrs.User = ambiguousUser

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
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
			)

			DescribeTable("should have no opinion because verb is not allowed",
				func(verb string) {
					attrs.Verb = verb

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [create]"))

				},

				Entry("get", "get"),
				Entry("list", "list"),
				Entry("watch", "watch"),
				Entry("update", "update"),
				Entry("patch", "patch"),
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
			)

			DescribeTable("should have no opinion because verb is not allowed",
				func(verb string) {
					attrs.Verb = verb

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [create]"))

				},

				Entry("get", "get"),
				Entry("list", "list"),
				Entry("watch", "watch"),
				Entry("update", "update"),
				Entry("patch", "patch"),
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

			DescribeTable("should have no opinion because verb is not allowed",
				func(verb string) {
					attrs.Verb = verb

					decision, reason, err := authorizer.Authorize(ctx, attrs)
					Expect(err).NotTo(HaveOccurred())
					Expect(decision).To(Equal(auth.DecisionNoOpinion))
					Expect(reason).To(ContainSubstring("only the following verbs are allowed for this resource type: [create get update]"))

				},

				Entry("list", "list"),
				Entry("watch", "watch"),
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
				Entry("update", "update"),
			)

			It("should allow because seed name is ambiguous", func() {
				attrs.User = ambiguousUser

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionAllow))
				Expect(reason).To(BeEmpty())
			})
		})
	})
})

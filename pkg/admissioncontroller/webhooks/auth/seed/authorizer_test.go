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

	corev1 "k8s.io/api/core/v1"

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhooks/auth/seed"
	graphpkg "github.com/gardener/gardener/pkg/admissioncontroller/webhooks/auth/seed/graph"
	mockgraph "github.com/gardener/gardener/pkg/admissioncontroller/webhooks/auth/seed/graph/mock"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		Context("when cases are unhandled", func() {
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
				Expect(reason).To(ContainSubstring("can only get individual resources of this type"))
			})

			It("should have no opinion because no resources requested", func() {
				attrs.Subresource = "status"

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionNoOpinion))
				Expect(reason).To(ContainSubstring("cannot get subresource"))
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
				Expect(reason).To(ContainSubstring("can only get individual resources of this type"))
			})

			It("should have no opinion because no resources requested", func() {
				attrs.Subresource = "status"

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionNoOpinion))
				Expect(reason).To(ContainSubstring("cannot get subresource"))
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
				Expect(reason).To(ContainSubstring("can only get individual resources of this type"))
			})

			It("should have no opinion because no resources requested", func() {
				attrs.Subresource = "status"

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionNoOpinion))
				Expect(reason).To(ContainSubstring("cannot get subresource"))
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
				Expect(reason).To(ContainSubstring("can only get individual resources of this type"))
			})

			It("should have no opinion because no resources requested", func() {
				attrs.Subresource = "status"

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionNoOpinion))
				Expect(reason).To(ContainSubstring("cannot get subresource"))
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
				Expect(reason).To(ContainSubstring("can only get individual resources of this type"))
			})

			It("should have no opinion because no resources requested", func() {
				attrs.Subresource = "status"

				decision, reason, err := authorizer.Authorize(ctx, attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(decision).To(Equal(auth.DecisionNoOpinion))
				Expect(reason).To(ContainSubstring("cannot get subresource"))
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
	})
})

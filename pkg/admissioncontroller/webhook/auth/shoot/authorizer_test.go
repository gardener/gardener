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
	"k8s.io/apiserver/pkg/authentication/user"
	auth "k8s.io/apiserver/pkg/authorization/authorizer"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhook/auth/shoot"
	"github.com/gardener/gardener/pkg/logger"
	mockgraph "github.com/gardener/gardener/pkg/utils/graph/mock"
)

var _ = Describe("Shoot", func() {
	var (
		ctx  context.Context
		ctrl *gomock.Controller

		log        logr.Logger
		graph      *mockgraph.MockInterface
		authorizer auth.Authorizer

		shootNamespace string
		shootName      string
		gardenletUser  user.Info
	)

	BeforeEach(func() {
		ctx = context.Background()
		ctrl = gomock.NewController(GinkgoT())

		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		graph = mockgraph.NewMockInterface(ctrl)
		authorizer = NewAuthorizer(log, graph, nil)

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
			// TODO(rfranzke): Unit tests for handled resources will be added here as development of autonomous shoots
			//  progresses.
		})
	})
})

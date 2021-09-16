// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://wwr.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common_test

import (
	"context"
	"net"

	. "github.com/gardener/gardener/extensions/pkg/controller/common"
	mockcommon "github.com/gardener/gardener/extensions/pkg/controller/common/mock"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	ownerName = "foo.example.com"
	ownerID   = "foo"
)

var _ = Describe("OwnerChecker", func() {
	var (
		ctrl         *gomock.Controller
		resolver     *mockcommon.MockResolver
		ctx          context.Context
		ownerChecker Checker
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		resolver = mockcommon.NewMockResolver(ctrl)
		ctx = context.TODO()
		ownerChecker = NewOwnerChecker(ownerName, ownerID, resolver, log.Log.WithName("test"))
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Check", func() {
		It("should return true if the owner domain name resolves to the specified owner ID", func() {
			resolver.EXPECT().LookupTXT(ctx, ownerName).Return([]string{ownerID}, nil)

			result, err := ownerChecker.Check(ctx)
			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(BeTrue())
		})

		It("should return false if the owner domain name resolves to a different owner ID", func() {
			resolver.EXPECT().LookupTXT(ctx, ownerName).Return([]string{"bar"}, nil)

			result, err := ownerChecker.Check(ctx)
			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(BeFalse())
		})

		It("should fail if the owner domain name could not be resolved", func() {
			resolver.EXPECT().LookupTXT(ctx, ownerName).Return(nil, &net.DNSError{})

			_, err := ownerChecker.Check(ctx)
			Expect(err).To(HaveOccurred())
		})
	})
})

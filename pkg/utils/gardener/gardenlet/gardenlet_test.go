// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardenlet_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	. "github.com/gardener/gardener/pkg/utils/gardener/gardenlet"
)

var _ = Describe("Gardenlet", func() {
	Describe("#SeedIsGarden", func() {
		var (
			ctx        context.Context
			mockReader *mockclient.MockReader
			ctrl       *gomock.Controller
		)

		BeforeEach(func() {
			ctx = context.Background()
			ctrl = gomock.NewController(GinkgoT())
			mockReader = mockclient.NewMockReader(ctrl)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should return that seed is a garden cluster", func() {
			mockReader.EXPECT().List(ctx, gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}), client.Limit(1)).DoAndReturn(
				func(_ context.Context, list *metav1.PartialObjectMetadataList, _ ...client.ListOption) error {
					list.Items = []metav1.PartialObjectMetadata{{}}
					return nil
				})
			Expect(SeedIsGarden(ctx, mockReader)).To(BeTrue())
		})

		It("should return that seed is a not a garden cluster because no garden object found", func() {
			mockReader.EXPECT().List(ctx, gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}), client.Limit(1))
			Expect(SeedIsGarden(ctx, mockReader)).To(BeFalse())
		})

		It("should return that seed is a not a garden cluster because of a no match error", func() {
			mockReader.EXPECT().List(ctx, gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}), client.Limit(1)).DoAndReturn(
				func(_ context.Context, list *metav1.PartialObjectMetadataList, _ ...client.ListOption) error {
					return &meta.NoResourceMatchError{}
				})
			Expect(SeedIsGarden(ctx, mockReader)).To(BeFalse())
		})
	})
})

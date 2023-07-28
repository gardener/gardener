// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package fake_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
)

var _ = Describe("FakeClientMapBuilder", func() {
	var (
		ctx     context.Context
		builder *fake.ClientMapBuilder
		key     clientmap.ClientSetKey
		ctrl    *gomock.Controller
	)

	BeforeEach(func() {
		ctx = context.TODO()
		key = keys.ForShoot(&gardencorev1beta1.Shoot{})
		ctrl = gomock.NewController(GinkgoT())

		builder = fake.NewClientMapBuilder()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("#WithClientSets", func() {
		It("should correctly add ClientSets", func() {
			fakeCS := kubernetesfake.NewClientSet()

			cm := builder.WithClientSets(map[clientmap.ClientSetKey]kubernetes.Interface{
				key: fakeCS,
			}).Build()

			cs, err := cm.GetClient(ctx, key)
			Expect(err).NotTo(HaveOccurred())
			Expect(cs).To(BeIdenticalTo(fakeCS))
		})
	})

	Context("#WithClientSetForKey", func() {
		It("should correctly add a single ClientSet", func() {
			fakeCS := kubernetesfake.NewClientSet()

			cm := builder.WithClientSetForKey(key, fakeCS).Build()

			cs, err := cm.GetClient(ctx, key)
			Expect(err).NotTo(HaveOccurred())
			Expect(cs).To(BeIdenticalTo(fakeCS))
		})
	})

	Context("#WithRuntimeClientForKey", func() {
		It("should correctly add a runtime client", func() {
			mockClient := mockclient.NewMockClient(ctrl)

			cm := builder.WithRuntimeClientForKey(key, mockClient).Build()

			cs, err := cm.GetClient(ctx, key)
			Expect(err).NotTo(HaveOccurred())
			Expect(cs.Client()).To(BeIdenticalTo(mockClient))
		})
	})
})

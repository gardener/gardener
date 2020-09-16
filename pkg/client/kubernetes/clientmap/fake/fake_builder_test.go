// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fake_test

import (
	"context"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	fakeclientset "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		key = keys.ForGarden()
		ctrl = gomock.NewController(GinkgoT())

		builder = fake.NewClientMapBuilder()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("#WithClientSets", func() {
		It("should correctly add ClientSets", func() {
			fakeCS := fakeclientset.NewClientSet()

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
			fakeCS := fakeclientset.NewClientSet()

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

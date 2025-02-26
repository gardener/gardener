// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
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
			fakeCS := fakekubernetes.NewClientSet()

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
			fakeCS := fakekubernetes.NewClientSet()

			cm := builder.WithClientSetForKey(key, fakeCS).Build()

			cs, err := cm.GetClient(ctx, key)
			Expect(err).NotTo(HaveOccurred())
			Expect(cs).To(BeIdenticalTo(fakeCS))
		})
	})

	Context("#WithRuntimeClientForKey", func() {
		It("should correctly add a runtime client", func() {
			mockClient := mockclient.NewMockClient(ctrl)

			cm := builder.WithRuntimeClientForKey(key, mockClient, nil).Build()

			cs, err := cm.GetClient(ctx, key)
			Expect(err).NotTo(HaveOccurred())
			Expect(cs.Client()).To(BeIdenticalTo(mockClient))
		})
	})
})

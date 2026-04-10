// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fake_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
)

var _ = Describe("FakeClientMap", func() {
	var (
		ctx context.Context
		cm  *fake.ClientMap
		key clientmap.ClientSetKey
	)

	BeforeEach(func() {
		ctx = context.TODO()
		key = keys.ForShoot(&gardencorev1beta1.Shoot{})

		cm = fake.NewClientMap()
	})

	Context("#GetClient", func() {
		It("should return error if key is not found", func() {
			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})
	})

	Context("#AddClient", func() {
		It("should correctly add and return a ClientSet", func() {
			fakeCS := fakekubernetes.NewClientSet()
			cm.AddClient(key, fakeCS)

			Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(fakeCS))
		})
	})

	Context("#AddRuntimeClient", func() {
		It("should correctly add and return a runtime client", func() {
			mockClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			cm.AddRuntimeClient(key, mockClient)

			cs, err := cm.GetClient(ctx, key)
			Expect(err).NotTo(HaveOccurred())
			Expect(cs.Client()).To(BeIdenticalTo(mockClient))
		})
	})

	Context("#NewClientMapWithClientSets", func() {
		It("should correctly add and return a ClientSet", func() {
			fakeCS := fakekubernetes.NewClientSet()
			cm = fake.NewClientMapWithClientSets(map[clientmap.ClientSetKey]kubernetes.Interface{
				key: fakeCS,
			})

			Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(fakeCS))
		})
	})

	Context("#InvalidateClient", func() {
		It("should do nothing if matching ClientSet is not found", func() {
			Expect(cm.InvalidateClient(key)).To(Succeed())
		})

		It("should delete the matching ClientSet from the ClientMap", func() {
			cm.AddClient(key, fakekubernetes.NewClientSet())
			Expect(cm.InvalidateClient(key)).To(Succeed())

			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})
	})

	Context("#Start", func() {
		It("should do nothing as the fake ClientMap does not support it", func() {
			Expect(cm.Start(ctx)).To(Succeed())
		})
	})
})

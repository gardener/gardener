// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package clientmap_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
)

var _ = Describe("SelfHostedShootClientMap", func() {
	var (
		ctx       context.Context
		clientSet *fake.ClientSet
		cm        clientmap.ClientMap
		key       clientmap.ClientSetKey
	)

	BeforeEach(func() {
		ctx = context.Background()
		clientSet = fake.NewClientSet()
		cm = clientmap.NewSelfHostedShootClientMap(clientSet)
		key = keys.ForShoot(&gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: "shoot", Namespace: "garden"}})
	})

	Describe("#GetClient", func() {
		It("should always return the same client set", func() {
			result, err := cm.GetClient(ctx, key)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeIdenticalTo(clientSet))
		})
	})

	Describe("#InvalidateClient", func() {
		It("should return nil", func() {
			Expect(cm.InvalidateClient(key)).To(Succeed())
		})
	})

	Describe("#Start", func() {
		It("should return nil", func() {
			Expect(cm.Start(ctx)).To(Succeed())
		})
	})
})

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cluster_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var _ = Describe("Cluster controller tests", func() {
	It("should create and add the virtual cluster", func() {
		mockManager.EXPECT().GetLogger().AnyTimes()

		var virtualClusterToManager cluster.Cluster
		mockManager.EXPECT().Add(gomock.Any()).Do(func(r manager.Runnable) {
			var ok bool
			virtualClusterToManager, ok = r.(cluster.Cluster)
			Expect(ok).To(BeTrue())
		})

		channel <- event.TypedGenericEvent[*rest.Config]{Object: restConfig}

		Eventually(func(g Gomega) {
			g.Expect(virtualCluster).ToNot(BeNil())
			g.Expect(virtualCluster).To(Equal(virtualClusterToManager))
		}).Should(Succeed())

		virtualRestConfig := virtualCluster.GetConfig()
		Expect(virtualRestConfig).NotTo(BeNil())
		Expect(virtualRestConfig.AcceptContentTypes).To(Equal(virtualClientConnection.AcceptContentTypes))
		Expect(virtualRestConfig.ContentType).To(Equal(virtualClientConnection.ContentType))
		Expect(virtualRestConfig.QPS).To(Equal(virtualClientConnection.QPS))
		Expect(virtualRestConfig.Burst).To(Equal(int(virtualClientConnection.Burst)))
	})
})

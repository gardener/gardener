// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package connect

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("#temporaryClusterAdminBindingStillExists", func() {
	var (
		ctx          = context.Background()
		fakeClient   client.Client
		clusterAdmin *rbacv1.ClusterRoleBinding
	)

	BeforeEach(func() {
		fakeClient = fake.NewClientBuilder().Build()
		clusterAdmin = &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "zzz-temporary-cluster-admin-access-for-bootstrapping"}}
	})

	It("should return false when the binding does not exist", func() {
		exists, err := temporaryClusterAdminBindingStillExists(ctx, fakeClient)
		Expect(err).NotTo(HaveOccurred())
		Expect(exists).To(BeFalse())
	})

	It("should return true when the binding still exists", func() {
		Expect(fakeClient.Create(ctx, clusterAdmin)).To(Succeed())

		exists, err := temporaryClusterAdminBindingStillExists(ctx, fakeClient)
		Expect(err).NotTo(HaveOccurred())
		Expect(exists).To(BeTrue())
	})
})

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
)

var _ = Describe("IsGardenletDeployed", func() {
	var (
		ctx       = context.Background()
		c         client.Client
		namespace = "foo"
	)

	BeforeEach(func() {
		c = fake.NewClientBuilder().Build()
	})

	When("gardenlet deployment does not exist", func() {
		It("should return false if no error occurs", func() {
			deployed, err := cmd.IsGardenletDeployed(ctx, c, namespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(deployed).To(BeFalse())
		})
	})

	When("gardenlet deployment exists", func() {
		BeforeEach(func() {
			Expect(c.Create(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "gardenlet", Namespace: namespace}})).NotTo(HaveOccurred())
		})

		When("kubeconfig secret does not exist", func() {
			It("should return false if the kubeconfig secret does not exist", func() {
				deployed, err := cmd.IsGardenletDeployed(ctx, c, namespace)
				Expect(err).NotTo(HaveOccurred())
				Expect(deployed).To(BeFalse())
			})
		})

		When("kubeconfig secret exists", func() {
			BeforeEach(func() {
				Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "gardenlet-kubeconfig", Namespace: namespace}})).NotTo(HaveOccurred())
			})

			It("should return true if both deployment and kubeconfig secret exist", func() {
				deployed, err := cmd.IsGardenletDeployed(ctx, c, namespace)
				Expect(err).NotTo(HaveOccurred())
				Expect(deployed).To(BeTrue())
			})
		})
	})
})
